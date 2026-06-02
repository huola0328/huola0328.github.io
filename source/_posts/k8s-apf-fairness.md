---
title: 一个失控的客户端如何打垮整个集群？——深入 K8s APF 公平限流与 Shuffle Sharding 算法
date: 2026-06-01 15:30:00
tags: [Kubernetes, APF, 限流, 算法]
categories: 技术分享
cover: /images/cover-apf.jpg
---

> 一个写了死循环 List 全量 Pod 的脚本，曾经就能把整个集群的 API Server 打挂，
> 导致所有人的 `kubectl`、所有控制器全部失联。
> K8s 是如何用 **API Priority & Fairness（APF）** 解决这个问题的？核心是一个精巧的算法——**Shuffle Sharding（洗牌分片）**。

## 一、旧世界的问题：max-inflight 的粗暴限流

在 APF 之前，API Server 只有两个全局开关：

- `--max-requests-inflight`（只读请求并发上限）
- `--max-mutating-requests-inflight`（写请求并发上限）

这是一个**全局共享的大池子**。问题显而易见：

{% mermaid %}
graph TB
    A[失控客户端<br/>疯狂 List 全量 Pod] -->|占满| P[全局并发池<br/>max-inflight]
    B[正常用户 kubectl] -->|抢不到| P
    C[Deployment Controller] -->|抢不到| P
    D[kubelet 上报] -->|抢不到| P
    P --> E[整个集群 API 雪崩<br/>所有人失联]
{% endmermaid %}

**没有隔离、没有优先级**：一个行为不端的客户端就能耗尽所有并发名额，饿死正常请求，甚至饿死系统关键组件（如 leader election、节点心跳），引发集群级故障。

![浪潮般涌入的请求洪流](/images/body-network.jpg)

## 二、APF 的两个核心对象

APF 用两类资源把"一个大池子"变成"分优先级、分租户隔离的多个池子"：

- **PriorityLevelConfiguration（优先级）**：定义若干并发"等级"，每个等级分到一份并发预算；等级之间相互隔离。
- **FlowSchema（流模式）**：规则匹配进来的请求（按 user / group / resource / verb），把它**分类到某个优先级**，并计算一个 **flow distinguisher**（流区分键，如按用户或按 namespace）。

{% mermaid %}
graph LR
    R[进来的请求] --> FS{FlowSchema 匹配<br/>user/verb/resource}
    FS -->|leader-election| PL1[优先级: leader-election]
    FS -->|workload-high| PL2[优先级: workload-high]
    FS -->|普通用户| PL3[优先级: global-default]
    FS -->|catch-all| PL4[优先级: catch-all]
{% endmermaid %}

每个 FlowSchema 还会用 `distinguisherMethod`（ByUser / ByNamespace）算出"这是谁的流"，为后面的公平排队做准备。

## 三、并发预算如何分配

服务器总并发额度（Server Concurrency Limit）按各优先级的 `nominalConcurrencyShares` **按比例瓜分**：

```
某优先级并发 = 总并发 × (该级 shares / 所有级 shares 之和)
```

K8s 1.24+ 还引入了 **借用（Borrowing）**：某优先级暂时没用满自己的额度时，可以把空闲并发**借给**繁忙的优先级，提升整体利用率，同时保留 `lendablePercent` / `borrowingLimit` 等护栏防止被借空。

还有一类特殊的 **Exempt（豁免）**优先级，比如 `system:masters`，**完全不受限流**，保证管理员永远能进。

## 四、核心算法：Shuffle Sharding（洗牌分片）

这是 APF 最精彩的部分，解决"**如何在同一优先级内部，让租户之间互不干扰**"。

### 4.1 朴素哈希的问题

最简单的做法：把每个 flow 用哈希映射到**一个**队列。但如果两个"重流"恰好哈希到同一个队列，其中一个就会被另一个拖垮——**碰撞即互相伤害**。

### 4.2 Shuffle Sharding 的思路

不把流映射到单个队列，而是映射到一个 **"手牌"（hand）**：从所有队列里**伪随机抽取 `handSize` 个**队列作为该流的候选，请求到来时选其中**最短的队列**入队。

{% mermaid %}
graph TB
    subgraph Queues[优先级内的队列池]
        Q0[Q0]
        Q1[Q1]
        Q2[Q2]
        Q3[Q3]
        Q4[Q4]
        Q5[Q5]
        Q6[Q6]
        Q7[Q7]
    end
    FA[流 A 的手牌] --> Q1
    FA --> Q3
    FA --> Q6
    FB[流 B 的手牌] --> Q0
    FB --> Q3
    FB --> Q5
{% endmermaid %}

关键洞察：**两个流的"手牌"完全重叠的概率极低**。即使流 A 是个疯狂刷请求的坏邻居，它顶多污染自己手牌里的 `handSize` 个队列；流 B 只要有**任意一个**队列不和 A 重叠，就能找到干净队列正常服务。

### 4.3 为什么有效（一点直觉）

假设有 `n` 个队列、手牌大小 `handSize`，两个流"完全碰撞"（手牌完全相同）的概率约为 `1 / C(n, handSize)`。哪怕 `n=8, handSize=4`，组合数也有 70，碰撞概率已很低；真实集群队列更多，**坏邻居几乎不可能同时堵死另一个租户的所有候选队列**。这就用很小的代价实现了**租户级隔离**。

## 五、排队与公平分发

入队之后，同一优先级用 **公平排队（Fair Queuing 的变体）** 在多个队列间轮转分发，按"虚拟时间"近似平分服务能力，保证没有哪个队列被长期饿死。

当队列也满了，请求会被**拒绝**：返回 `429 Too Many Requests` 并带 `Retry-After`，让客户端退避重试。

{% mermaid %}
graph LR
    In[请求] --> Match[FlowSchema 分类]
    Match --> Shuffle[Shuffle Sharding<br/>选最短队列]
    Shuffle --> FQ[公平排队轮转分发]
    FQ -->|有并发额度| Exec[执行]
    FQ -->|队列满| Rej[429 + Retry-After]
{% endmermaid %}

## 六、动手观察

查看集群内置的优先级与流模式：

```bash
kubectl get prioritylevelconfigurations
kubectl get flowschemas
```

看一个请求被分到了哪个 FlowSchema / 优先级——响应头里直接告诉你：

```bash
kubectl get --raw='/api/v1/namespaces/default/pods?limit=1' -v=8 2>&1 \
  | grep -i 'X-Kubernetes-PF'
# X-Kubernetes-PF-FlowSchema-UID: ...
# X-Kubernetes-PF-PriorityLevel-UID: ...
```

关键监控指标（Prometheus）：

- `apiserver_flowcontrol_rejected_requests_total` —— 被拒绝（429）的请求数
- `apiserver_flowcontrol_request_wait_duration_seconds` —— 在队列里等待的时长
- `apiserver_flowcontrol_current_executing_requests` —— 正在执行的请求数
- `apiserver_flowcontrol_current_inqueue_requests` —— 排队中的请求数

完整的"自定义优先级隔离某个客户端"的演示见配套 demo 目录。

## 七、一图总结

{% mermaid %}
graph LR
    A[请求] --> B[FlowSchema 分类<br/>+ 算 flow 区分键]
    B --> C[PriorityLevel<br/>按 shares 分并发预算]
    C --> D[Shuffle Sharding<br/>抽手牌选最短队列]
    D --> E[公平排队分发]
    E --> F[执行 / 429 退避]
{% endmermaid %}

APF 的本质：把"一个全局大池子"重构为**分优先级（保护关键流量）+ 分租户隔离（Shuffle Sharding）+ 公平排队**的多级限流系统，让"一个坏客户端打垮整个集群"成为历史。

## 八、常见问题

- **APF 和 Ingress 限流/服务网格限流区别？** APF 是 **API Server 自身**的请求级保护，作用在控制面入口，和数据面流量限流是两回事。
- **请求被 429 了怎么办？** client-go 默认会按 `Retry-After` 退避重试；也可调整对应优先级的 shares 或排队配置。
- **怎么保护 leader election / 节点心跳？** 它们走高优先级（如 `leader-election`、`system`）FlowSchema，并发被单独保障，不受普通请求挤占。
- **Exempt 会不会成为后门？** `system:masters` 等豁免级不限流，所以更要严管谁有这个权限。
- **handSize 越大越好吗？** 不是。handSize 越大隔离越好但单流可用队列越多、公平性计算成本越高，需权衡（默认值经过调优）。
