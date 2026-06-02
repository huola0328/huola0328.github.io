---
title: K8s 的灵魂——控制器 Reconcile 机制，以及如何亲手写一个 Operator
date: 2026-06-01 14:20:00
tags: [Kubernetes, Operator, Controller, CRD]
categories: 技术分享
cover: /images/cover-operator.jpg
---

> 为什么你删掉一个 Deployment，它管理的 Pod 会自动消失？为什么 Pod 挂了会自动重建？
> 答案不是"调度"，而是 K8s 真正的灵魂——**声明式 API + 控制器循环（Reconcile）**。
> 本文讲透这套机制，并带你用 kubebuilder 亲手写一个 Operator。

## 一、一个被忽视的真相：K8s 是一堆"控制器"组成的

很多人以为 K8s 的核心是调度器。其实调度只是"把 Pod 放到哪个节点"这一个动作。真正让 K8s "活起来"的，是遍布各处的**控制器（Controller）**：

- Deployment Controller 保证副本数
- Node Controller 监控节点健康
- Job Controller 管理任务完成
- ……以及你自己写的 Operator

它们都遵循同一个范式：**声明式 + 控制循环**。

## 二、声明式 vs 命令式：期望态驱动

- **命令式**：你告诉系统"怎么做"——先创建 A，再修改 B，删除 C。
- **声明式**：你只描述"我想要什么"（期望态 Spec），系统自己想办法达成。

K8s 是彻底的声明式。你写下 `replicas: 3`，至于现在有几个 Pod、要创建还是删除，由控制器去"对齐"。

{% mermaid %}
graph LR
    A[期望态 Spec<br/>replicas: 3] --> C{控制器对比}
    B[实际态 Status<br/>当前 2 个 Pod] --> C
    C -->|有差异| D[执行动作<br/>创建 1 个 Pod]
    D --> B
    C -->|无差异| E[什么都不做]
{% endmermaid %}

这个"不断对比期望态与实际态、并努力消除差异"的循环，就叫 **Reconcile（调谐）**。

## 三、控制器的工作引擎：List-Watch + Informer + WorkQueue

控制器不是傻乎乎地轮询 API Server，而是用一套高效的事件驱动机制：

{% mermaid %}
sequenceDiagram
    participant API as API Server
    participant R as Reflector (List-Watch)
    participant S as Informer 本地缓存(Store)
    participant Q as WorkQueue
    participant C as Controller (Reconcile)

    R->>API: 1. List 全量 + Watch 增量
    API-->>R: 资源变更事件
    R->>S: 2. 更新本地缓存
    R->>Q: 3. 把对象 key 入队(去重/限速)
    C->>Q: 4. 取出 key
    C->>S: 5. 从缓存读对象(不直接打 API)
    C->>API: 6. Reconcile: 对齐期望态
    Note over C,Q: 失败则重新入队(指数退避重试)
{% endmermaid %}

**几个关键设计**：

- **Reflector / List-Watch**：先 List 全量，再 Watch 增量，保证不丢事件、顺序一致（底层就是 etcd 的 Watch）。
- **Informer 本地缓存**：控制器读对象走本地缓存，**极大降低 API Server 压力**。
- **WorkQueue**：带**去重**（同一对象短时间多次变更只处理一次）、**限速**（限流）、**指数退避重试**（失败自动重试）。

## 四、Reconcile 函数的两条铁律

写控制器时，`Reconcile()` 函数必须遵守：

**1. 幂等（Idempotent）**
- 同一个对象 reconcile 一次和 N 次，结果必须一样。
- 因为同一个 key 可能被重复触发，绝不能"每次都创建一个新 Pod"。

**2. 面向期望态，而非事件**
- 不要关心"发生了什么事件（增/删/改）"，只关心"现在该长什么样"。
- 每次都重新读当前实际态，和 Spec 对比，缺啥补啥、多啥删啥。

{% mermaid %}
graph TD
    Start[收到 reconcile 请求] --> Get[读取 CR 当前状态]
    Get --> NotFound{对象还在吗?}
    NotFound -->|已删除| End1[清理并返回]
    NotFound -->|存在| Compare[对比期望态 vs 实际态]
    Compare --> Diff{有差异?}
    Diff -->|是| Act[创建/更新/删除子资源]
    Diff -->|否| End2[更新 Status, 返回]
    Act --> UpdateStatus[更新 Status]
    UpdateStatus --> End2
{% endmermaid %}

![像一个 7x24 不知疲倦的运维机器人](/images/body-gears.jpg)

## 五、什么是 Operator？

**Operator = CRD（自定义资源）+ 自定义控制器**。

它把"运维某个复杂应用的人类知识"用代码固化下来。比如一个 Redis Operator，你只需声明：

```yaml
apiVersion: cache.example.com/v1
kind: RedisCluster
spec:
  size: 3
  version: "7.0"
```

Operator 就会自动帮你建主从、配置哨兵、做备份、故障切换——就像一个 7x24 的运维专家。

## 六、动手：用 kubebuilder 写一个 Operator

以一个极简的 `Website` CRD 为例，目标：声明一个 `Website` 就自动创建对应的 Deployment + Service。

```bash
# 1. 初始化项目
kubebuilder init --domain example.com --repo example.com/website-operator

# 2. 创建 API（CRD + Controller 骨架）
kubebuilder create api --group web --version v1 --kind Website
```

核心是填写 `Reconcile()` 逻辑（伪代码）：

```go
func (r *WebsiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. 读取期望态
    var site webv1.Website
    if err := r.Get(ctx, req.NamespacedName, &site); err != nil {
        // 对象已删除：OwnerReference 会自动级联删除子资源
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. 构造期望的 Deployment
    desired := buildDeployment(&site)
    // 设置 OwnerReference：删 Website 时自动删 Deployment
    ctrl.SetControllerReference(&site, desired, r.Scheme)

    // 3. 对齐：不存在则创建，存在且有差异则更新（幂等）
    if err := r.createOrUpdate(ctx, desired); err != nil {
        return ctrl.Result{}, err // 出错返回 error，会自动重新入队重试
    }

    // 4. 更新 Status 反映实际态
    site.Status.AvailableReplicas = desired.Status.AvailableReplicas
    r.Status().Update(ctx, &site)

    return ctrl.Result{}, nil
}
```

```bash
# 3. 安装 CRD 并本地运行控制器
make install
make run
```

完整可跑的步骤见分享配套的 demo 目录。

## 七、一图总结

{% mermaid %}
graph LR
    A[声明式 Spec<br/>我想要什么] --> B[List-Watch<br/>感知变化]
    B --> C[WorkQueue<br/>去重/限速/重试]
    C --> D[Reconcile<br/>幂等地对齐期望态]
    D --> E[更新 Status<br/>反映实际态]
    E -.持续循环.-> B
{% endmermaid %}

理解了 Reconcile，你就理解了 K8s 的本质：**它不是一次性执行命令，而是一个永不停歇、持续把世界拉向你期望状态的控制系统。**

## 八、常见问题

- **Reconcile 为什么会被重复调用？** WorkQueue 的去重和重试机制决定了同一对象可能多次触发，所以必须幂等。
- **控制器读对象为什么不直接查 API Server？** 走 Informer 本地缓存，降低 API 压力；但要注意缓存可能短暂滞后。
- **删除时怎么做清理？** 用 **Finalizer** 拦截删除，在真正删除前执行清理逻辑（如释放外部资源）。
- **子资源怎么自动级联删除？** 给子资源设置 **OwnerReference** 指向 CR，GC 会自动级联删除。
- **Operator 和 Helm 区别？** Helm 是一次性模板渲染安装；Operator 是持续运行、能感知运行时状态并自愈的"活"程序。
