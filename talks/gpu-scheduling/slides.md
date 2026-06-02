---
marp: true
theme: default
paginate: true
size: 16:9
header: 'K8s GPU / 异构资源调度深度解析'
footer: 'huola0328'
---

<!-- _class: lead -->
<!-- _paginate: false -->

# K8s 凭什么能调度它"原生不认识"的 GPU？

## 从 Device Plugin 到 GPU 共享与拓扑感知调度

---

<!-- _class: lead -->

# 开场：一个核心矛盾

K8s 原生**只认识 `cpu` 和 `memory`**，它们可压缩、可超卖

而 **GPU 默认：整卡独占、不可超卖、不可分割**

> 8 卡节点最多只能跑 8 个 Pod？这也太浪费了

---

# 贯穿全场的三个问题

1. K8s 凭什么能调度一个它"原生不认识"的 GPU？
   → **Device Plugin**

2. 一张 A100 这么贵，多个小任务能共享一张卡吗？
   → **MIG / MPS / Time-slicing**

3. GPU 和 CPU / 网卡跨 NUMA，性能暴跌怎么办？
   → **拓扑感知调度**

---

# 铺垫：Extended Resource（扩展资源）

节点上报自定义资源：`nvidia.com/gpu: 8`

```yaml
resources:
  limits:
    nvidia.com/gpu: 1   # 只能写 limits
                        # requests 自动 = limits，不可超卖
```

**关键差异**：GPU 是 integer-only、不可压缩、requests == limits

---

<!-- _class: lead -->

# 核心一：Device Plugin
## K8s 如何"认识"GPU

---

# Device Plugin 设计哲学

- K8s 核心代码 **不内置任何厂商硬件逻辑**
- 定义一套 **gRPC 接口**，厂商各自实现
  - NVIDIA / 华为昇腾 / Intel / AMD
- 新硬件只要写个插件，**K8s 无需改一行代码**

> 这就是 K8s 可扩展性的精髓

---

# Device Plugin 工作流程

1. 以 **DaemonSet** 部署在每个 GPU 节点
2. 通过 Unix socket 向 **kubelet** 注册（`Register`）
3. `ListAndWatch`：持续上报 GPU 列表与健康状态
4. kubelet 上报 `nvidia.com/gpu: 8` 给 API Server
5. 调度器据此把 Pod 调到有空闲 GPU 的节点
6. kubelet 调 `Allocate`：返回设备文件 / 环境变量 / 挂载点
7. 透传给 containerd → 容器内可见 GPU

---

# 最重要的认知（易错点）

## 调度器只做"数数"

它只知道"这节点还剩**几张**卡"

**它不知道具体是哪张卡！**

> 真正的设备分配，是 kubelet + Device Plugin
> 在**节点本地**通过 `Allocate` 完成的

---

<!-- _class: lead -->

# 核心二：GPU 共享
## 把一张卡切给多个任务

---

# 四种共享方案对比

| 方案 | 隔离 | 算力隔离 | 显存隔离 | 故障隔离 | 场景 |
|------|------|------|------|------|------|
| Time-slicing | 弱 | 否 | 否 | 否 | 开发/低负载推理 |
| MPS | 中 | 部分 | 部分 | 否 | 多小任务推理 |
| **MIG** | **强** | **是** | **是** | **是** | 生产多租户 |
| vGPU(HAMi) | 中-强 | 软件 | 是 | 部分 | 显存超卖 |

---

# 逐个拆解

- **Time-slicing**：多 Pod 看到"同一张卡"，轮流执行，**会互相抢占**，无隔离，配置最简单
- **MPS**：多进程共享 CUDA context，减少上下文切换，适合大量小推理
- **MIG**：**硬件级切分**，A100 最多切 7 份，各自独立算力/显存/L2，真正强隔离
- **第三方 vGPU**：拦截 CUDA 调用，显存硬限制 + 算力按比例 + 可超卖

> 没有银弹：隔离越强，弹性越差

---

<!-- _class: lead -->

# 核心三：拓扑感知调度
## 调度成功 ≠ 性能好

---

# 问题本质：服务器内部有拓扑

```
NUMA0: [CPU 0-31] [GPU0]==NVLink==[GPU1] [RDMA网卡]
                      |
                   PCIe/QPI (慢)
                      |
NUMA1: [CPU 32-63] [GPU2]==NVLink==[GPU3]
```

8 卡训练若分到**跨 NUMA / 跨 PCIe** 的 GPU
→ **NCCL 带宽暴跌，训练慢一倍以上**

---

# 解决手段

- **Topology Manager**（kubelet）
  协调 CPU / Memory / Device Manager
  让 GPU、CPU、网卡落在**同一 NUMA**

- **GPU Feature Discovery**
  自动给节点打拓扑 label

- **批调度器（Volcano / Koordinator）**
  感知 NVLink 拓扑，优先分配互联紧密的 GPU 组

---

<!-- _class: lead -->

# 核心四：Gang Scheduling
## 让分布式训练"调得动"

---

# 默认调度器的致命缺陷

训练需 8 个 Pod，默认逐个调度：

```
成功调度 5 个  ┐
              ├→ 5 个占着 GPU 空等
剩 3 个无资源  ┘   谁也跑不起来
                  = 死锁 + 资源浪费
```

**Gang Scheduling**：要么 8 个一起成功，要么一个都不调度（All-or-Nothing）

工具：**Volcano**（CNCF 事实标准）/ Koordinator / Coscheduling

---

# 现场演示清单

1. `kubectl get nodes` → 看 `nvidia.com/gpu` capacity
2. 部署申请 1 GPU 的 Pod，容器内跑 `nvidia-smi`
3. 开 time-slicing，2 个 Pod 共享 1 张卡
4. （A100）MIG 切分，`nvidia-smi -L` 看多实例
5. Volcano 提交 gang 任务，演示 all-or-nothing
6. `kubectl describe node` 看拓扑 label

---

<!-- _class: lead -->

# 主线总结

Device Plugin（认识）→ 共享方案（复用）
→ 拓扑感知（跑得快）→ Gang Scheduling（调得动）

## GPU 调度的本质
## = 在「利用率」与「隔离性/性能」之间找平衡

---

# Q&A 预备

- **能超卖吗？** 原生不行；time-slicing/vGPU 可伪超卖，有抢占风险
- **MIG vs Time-slicing？** 多租户选 MIG，开发测试选 time-slicing
- **调度器知道哪张卡？** 不知道，只数数，kubelet 本地分配
- **多卡训练为什么慢？** 拓扑没对齐 / 没走 RDMA / NVLink
- **怎么监控？** DCGM Exporter + Prometheus + Grafana

---

<!-- _class: lead -->
<!-- _paginate: false -->

# 谢谢

## Q & A
