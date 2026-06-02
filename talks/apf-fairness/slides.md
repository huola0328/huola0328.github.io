---
marp: true
theme: default
paginate: true
size: 16:9
header: 'K8s APF 公平限流 & Shuffle Sharding 算法'
footer: 'huola0328'
---

<!-- _class: lead -->
<!-- _paginate: false -->

# 一个失控的客户端
# 如何打垮整个集群？

## 深入 K8s APF 公平限流与 Shuffle Sharding 算法

---

<!-- _class: lead -->

# 开场：一个真实事故

一个写了死循环、疯狂 List 全量 Pod 的脚本

就能把 **API Server 打挂**

→ 所有人的 `kubectl` 失联
→ 所有控制器停摆
→ **集群级故障**

> K8s 怎么解决？答案是 APF + 一个精巧算法

---

# 旧世界：max-inflight 的粗暴限流

APF 之前只有两个全局开关：

- `--max-requests-inflight`（读）
- `--max-mutating-requests-inflight`（写）

**一个全局共享的大池子**

```
失控客户端 ─┐
正常 kubectl ─┼─► [全局并发池] ─► 池满 ─► 雪崩
控制器       ─┤
kubelet 心跳 ─┘
```

没有隔离、没有优先级 → 坏邻居饿死所有人

---

# APF 的两个核心对象

**PriorityLevelConfiguration（优先级）**
定义并发"等级"，每级分一份并发预算，互相隔离

**FlowSchema（流模式）**
按 user/group/resource/verb 匹配请求
→ 分类到某优先级
→ 算一个 flow 区分键（ByUser / ByNamespace）

---

# 请求如何被分类

```
请求 ──► FlowSchema 匹配
           ├── leader-election → 优先级: leader-election
           ├── workload-high   → 优先级: workload-high
           ├── 普通用户         → 优先级: global-default
           └── catch-all       → 优先级: catch-all
```

关键流量（选主/心跳）走高优先级，单独保障

---

# 并发预算如何分配

按各优先级的 shares **按比例瓜分**总并发：

```
某级并发 = 总并发 × (该级 shares / 总 shares)
```

- **Borrowing（借用）**：空闲额度可临时借给繁忙的级
- **Exempt（豁免）**：如 system:masters，完全不限流

---

<!-- _class: lead -->

# 核心算法
## Shuffle Sharding（洗牌分片）

如何让同一优先级内，租户之间互不干扰？

---

# 朴素哈希的问题

把每个流哈希到**一个**队列：

```
流A ─► Q3
流B ─► Q3   ← 碰撞！B 被 A 拖垮
```

**碰撞即互相伤害**，无法隔离坏邻居

---

# Shuffle Sharding 的思路

不映射到单个队列，而是一副**"手牌"**：
伪随机抽 `handSize` 个队列，请求选其中**最短**的

```
队列池: Q0 Q1 Q2 Q3 Q4 Q5 Q6 Q7

流A 手牌: {Q1, Q3, Q6}
流B 手牌: {Q0, Q3, Q5}
         └─ 只重叠 Q3，B 还有 Q0/Q5 可用
```

---

# 为什么有效

两个流"手牌完全重叠"的概率 ≈ `1 / C(n, handSize)`

- n=8, handSize=4 → 组合数 70，碰撞概率已很低
- 真实集群队列更多 → 概率极低

**坏邻居几乎不可能堵死另一租户的所有候选队列**

→ 用很小代价实现**租户级隔离**

---

# 排队与公平分发

入队后：**公平排队（Fair Queuing 变体）**
按"虚拟时间"在多队列间轮转，近似平分服务能力

队列也满了 → 拒绝：
**429 Too Many Requests + Retry-After** → 客户端退避

```
请求 → 分类 → Shuffle 选最短队列
     → 公平轮转 → 有额度则执行 / 满则 429
```

---

# 动手观察

```bash
# 看内置优先级与流模式
kubectl get prioritylevelconfigurations
kubectl get flowschemas

# 看请求被分到哪个 FlowSchema / 优先级（响应头）
kubectl get --raw='/api/v1/namespaces/default/pods?limit=1' \
  -v=8 2>&1 | grep -i 'X-Kubernetes-PF'
```

关键指标：
`apiserver_flowcontrol_rejected_requests_total`
`apiserver_flowcontrol_request_wait_duration_seconds`

---

<!-- _class: lead -->

# 一图总结

请求 → FlowSchema 分类 → PriorityLevel 分预算
→ Shuffle Sharding 抽手牌选最短队列
→ 公平排队 → 执行 / 429 退避

## 把"全局大池子"重构为
## 分优先级 + 租户隔离 + 公平排队的多级限流

---

# Q&A 预备

- **和 Ingress/网格限流区别？** APF 是 API Server 自身的请求级保护
- **被 429 怎么办？** client-go 按 Retry-After 退避重试
- **怎么保护选主/心跳？** 走高优先级 FlowSchema，并发单独保障
- **Exempt 是后门吗？** system:masters 不限流，要严管权限
- **handSize 越大越好吗？** 否，隔离更好但成本更高，需权衡

---

<!-- _class: lead -->
<!-- _paginate: false -->

# 谢谢

## Q & A
