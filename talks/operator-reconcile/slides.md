---
marp: true
theme: default
paginate: true
size: 16:9
header: 'K8s 控制器 Reconcile 机制 & Operator 开发'
footer: 'huola0328'
---

<!-- _class: lead -->
<!-- _paginate: false -->

# K8s 的灵魂：控制器 Reconcile 机制

## 以及如何亲手写一个 Operator

---

<!-- _class: lead -->

# 开场：一个"理所当然"的问题

为什么你删掉一个 **Deployment**
它管理的 **Pod 会自动消失**？

为什么 Pod 挂了**会自动重建**？

> 答案不是"调度"，而是 **控制器循环（Reconcile）**

---

# 被忽视的真相

很多人以为 K8s 核心是**调度器**

其实调度只是"把 Pod 放哪个节点"这一个动作

**真正让 K8s 活起来的，是遍布各处的控制器：**

- Deployment Controller → 保证副本数
- Node Controller → 监控节点
- Job Controller → 管理任务
- ……以及你自己写的 **Operator**

它们都遵循同一范式：**声明式 + 控制循环**

---

# 命令式 vs 声明式

| | 命令式 | 声明式（K8s） |
|---|--------|--------------|
| 你给的 | 怎么做（步骤） | 想要什么（期望态）|
| 例子 | 建A→改B→删C | `replicas: 3` |
| 谁负责对齐 | 你自己 | 控制器 |

你只写 `replicas: 3`
现在有几个 Pod、要增要删 —— **控制器自己搞定**

---

# 核心概念：Reconcile（调谐）

```
   期望态 Spec (replicas: 3)
            │
            ▼
      ┌──────────┐
      │ 控制器对比 │ ──── 无差异 ──► 什么都不做
      └──────────┘
            │ 有差异
            ▼
      创建 1 个 Pod（实际态 2 → 3）
```

**不断对比"期望态 vs 实际态"并消除差异**

---

<!-- _class: lead -->

# 工作引擎
## List-Watch + Informer + WorkQueue

---

# 控制器不是傻轮询

```
API Server
   │  List 全量 + Watch 增量
   ▼
Reflector ──► Informer 本地缓存(Store)
   │
   ▼
WorkQueue (去重 / 限速 / 退避重试)
   │  取出 key
   ▼
Controller.Reconcile()  ── 读缓存, 对齐期望态
```

---

# 三个关键设计

- **List-Watch**：先 List 全量 + Watch 增量
  不丢事件、顺序一致（底层是 etcd Watch）

- **Informer 本地缓存**：读对象走缓存
  **极大降低 API Server 压力**

- **WorkQueue**：
  去重（多次变更只处理一次）
  限速 + **指数退避重试**（失败自动重试）

---

# Reconcile 的两条铁律

## 1. 幂等（Idempotent）
同一对象 reconcile 1 次和 N 次，结果一样
（同一 key 会被重复触发，绝不能每次都新建）

## 2. 面向期望态，而非事件
不关心"发生了什么事件"
只关心"现在该长什么样"，缺啥补啥

---

# Reconcile 标准流程

```
收到请求 → 读取 CR 当前状态
   │
   ├── 对象已删除? → 清理 + 返回
   │
   └── 存在 → 对比期望态 vs 实际态
              │
              ├── 有差异 → 创建/更新/删除子资源
              │
              └── 更新 Status → 返回
```

---

# 什么是 Operator？

## Operator = CRD + 自定义控制器

把"运维复杂应用的人类知识"用代码固化

```yaml
apiVersion: cache.example.com/v1
kind: RedisCluster
spec:
  size: 3
  version: "7.0"
```

→ 自动建主从、配哨兵、备份、故障切换
**像一个 7x24 的运维专家**

---

# 动手：kubebuilder 写 Operator

```bash
# 初始化项目
kubebuilder init --domain example.com \
  --repo example.com/website-operator

# 创建 API（CRD + Controller 骨架）
kubebuilder create api \
  --group web --version v1 --kind Website
```

目标：声明一个 `Website` → 自动建 Deployment + Service

---

# 核心：填写 Reconcile()

```go
func (r *WebsiteReconciler) Reconcile(ctx, req) {
    // 1. 读期望态
    var site webv1.Website
    if err := r.Get(ctx, req.NN, &site); err != nil {
        return IgnoreNotFound(err) // 删除走级联GC
    }
    // 2. 构造期望 Deployment + 设 OwnerReference
    desired := buildDeployment(&site)
    ctrl.SetControllerReference(&site, desired, r.Scheme)
    // 3. 幂等对齐：没有则建，有差异则更新
    r.createOrUpdate(ctx, desired)
    // 4. 回写 Status
    r.Status().Update(ctx, &site)
}
```

---

# 运行与演示

```bash
make install   # 安装 CRD
make run        # 本地跑控制器

kubectl apply -f config/samples/  # 创建一个 Website
kubectl get deploy,svc            # 看自动创建的子资源
kubectl delete website sample     # 删除 → 子资源级联消失
```

> 现场演示：改 replicas → 控制器秒级对齐

---

<!-- _class: lead -->

# 一图总结

声明式 Spec → List-Watch 感知 → WorkQueue 去重重试
→ Reconcile 幂等对齐 → 更新 Status → 持续循环

## K8s 不是执行一次性命令
## 而是永不停歇地把世界拉向你期望的状态

---

# Q&A 预备

- **为什么 Reconcile 被重复调用？** WorkQueue 去重+重试，必须幂等
- **为什么读缓存不查 API？** Informer 降压，但可能短暂滞后
- **删除时怎么清理？** Finalizer 拦截删除做清理
- **子资源怎么级联删除？** OwnerReference + GC
- **和 Helm 区别？** Helm 一次性模板；Operator 持续运行、自愈

---

<!-- _class: lead -->
<!-- _paginate: false -->

# 谢谢

## Q & A
