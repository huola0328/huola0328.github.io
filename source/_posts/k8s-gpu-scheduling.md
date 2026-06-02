---
title: K8s 凭什么能调度它"原生不认识"的 GPU？——从 Device Plugin 到 GPU 共享与拓扑感知
date: 2026-06-01 13:45:00
tags: [Kubernetes, GPU, 调度, AI]
categories: 技术分享
cover: /images/cover-gpu.jpg
---

> K8s 默认只会数 CPU 和内存，它是如何"学会"管理一张昂贵的 GPU 的？
> 本文沿着一条主线展开：**Device Plugin 让 K8s 认识 GPU → 共享方案让 GPU 被复用 → 拓扑感知让 GPU 跑得快 → Gang Scheduling 让训练调得动。**

## 一、核心矛盾：GPU 和 CPU 根本不是一类资源

K8s 原生只认识两种资源：`cpu` 和 `memory`，它们是**可压缩、可超卖**的。但 GPU 完全不同——默认是**整卡独占、不可超卖、不可分割**，一个 8 卡节点最多跑 8 个 Pod。

由此引出贯穿全文的三个问题：

1. K8s 凭什么能调度一个它"原生不认识"的 GPU？
2. 一张 A100 这么贵，多个小任务能不能共享一张卡？
3. GPU 和网卡、CPU 不在同一 NUMA / PCIe 拓扑上，性能会暴跌，怎么办？

## 二、扩展资源（Extended Resource）

K8s 用 **Extended Resource** 让节点上报自定义资源，例如 `nvidia.com/gpu: 8`。Pod 这样申请：

```yaml
resources:
  limits:
    nvidia.com/gpu: 1   # GPU 只能写在 limits，requests 自动等于 limits，不可超卖
```

关键差异：GPU 是 **integer-only、不可压缩、requests 必须等于 limits**。

## 三、Device Plugin：K8s 如何"认识"GPU

设计哲学：K8s 核心代码**不内置任何厂商硬件逻辑**，而是定义一套 **Device Plugin gRPC 接口**，让 NVIDIA / 昇腾 / Intel 各自实现。新硬件只要写个插件，K8s 无需改代码。

工作流程如下：

{% mermaid %}
sequenceDiagram
    participant DP as Device Plugin (DaemonSet)
    participant K as kubelet
    participant API as API Server
    participant S as Scheduler
    participant C as containerd

    DP->>K: 1. Register (通过 Unix socket 注册)
    DP->>K: 2. ListAndWatch (持续上报 GPU 列表与健康)
    K->>API: 3. 上报 nvidia.com/gpu: 8
    API->>S: Pod 申请 nvidia.com/gpu: 1
    S->>API: 4. 调度到有空闲 GPU 的节点(只数数)
    K->>DP: 5. Allocate (kubelet 请求分配)
    DP->>K: 返回设备文件/环境变量/挂载点
    K->>C: 6. 透传给容器运行时
    C->>C: 容器内可见 GPU
{% endmermaid %}

**最重要的认知**：调度器只做"数数"——它只知道"这节点还剩几张卡"，**并不知道具体哪张卡**。真正的设备分配是 kubelet + Device Plugin 在节点本地通过 `Allocate` 完成的。这是个常见误区。

## 四、GPU 共享：把一张卡切给多个任务

直接回答第 2 问，这是降本增效的关键。四种主流方案对比：

| 方案 | 隔离级别 | 算力隔离 | 显存隔离 | 故障隔离 | 适用场景 |
|------|---------|---------|---------|---------|---------|
| **Time-slicing** | 弱 | 否（抢占） | 否 | 否 | 开发/测试、低负载推理 |
| **MPS** | 中 | 部分 | 部分 | 否 | 多小任务并发推理 |
| **MIG** | 强（硬件级） | 是 | 是 | 是 | A100/H100 生产多租户 |
| **vGPU**（如 HAMi/cGPU） | 中-强 | 软件限制 | 是 | 部分 | 通用切分、显存超卖 |

- **Time-slicing**：多个 Pod 看到"同一张卡"，GPU 轮流执行，会互相抢占，无隔离，配置最简单。
- **MPS**：NVIDIA Multi-Process Service，多进程共享 CUDA context，减少上下文切换，适合大量小推理。
- **MIG**：**硬件级切分**，A100 最多切 7 个独立实例，各有独立算力、显存、L2 缓存，真正强隔离，生产多租户首选。
- **第三方 vGPU**：软件层拦截 CUDA 调用，实现显存硬限制 + 算力按比例，还能显存超卖。

没有银弹：隔离越强越浪费弹性。Time-slicing 省钱但危险，MIG 安全但粒度固定。

![数据中心服务器与网络布线](/images/body-datacenter.jpg)

## 五、拓扑感知调度：调度成功 ≠ 性能好

回答第 3 问。一台 GPU 服务器内部是有拓扑结构的：

{% mermaid %}
graph TB
    subgraph NUMA0[NUMA Node 0]
        CPU0[CPU 0-31]
        GPU0[GPU0]
        GPU1[GPU1]
        NIC0[RDMA 网卡]
    end
    subgraph NUMA1[NUMA Node 1]
        CPU1[CPU 32-63]
        GPU2[GPU2]
        GPU3[GPU3]
    end
    GPU0 ---|NVLink 快| GPU1
    GPU2 ---|NVLink 快| GPU3
    GPU1 -.->|PCIe/QPI 慢| GPU2
    CPU0 --- GPU0
    NIC0 --- GPU0
{% endmermaid %}

如果一个 8 卡训练任务被分到了**跨 NUMA、跨 PCIe Switch 的 GPU**，**NCCL 通信带宽会暴跌**，训练速度可能差一倍以上。

解决手段：

- **Topology Manager**（kubelet）：协调 CPU Manager / Memory Manager / Device Manager，让 GPU、CPU、网卡尽量落在同一 NUMA。
- **GPU Feature Discovery**：自动给节点打拓扑相关 label。
- **批调度器（Volcano / Koordinator）**：感知 NVLink 拓扑，优先分配互联紧密的 GPU 组。

## 六、Gang Scheduling：让分布式训练"调得动"

默认调度器逐个 Pod 调度，对 AI 训练是致命缺陷：

{% mermaid %}
graph LR
    A[训练任务需 8 个 Pod] --> B{默认调度器逐个调度}
    B --> C[成功调度 5 个]
    B --> D[剩 3 个无资源]
    C --> E[5 个 Pod 占着 GPU 空等]
    D --> E
    E --> F[死锁 + 资源浪费]
{% endmermaid %}

**Gang Scheduling（成组调度）**：要么 8 个一起调度成功，要么一个都不调度（All-or-Nothing）。工具有 **Volcano**（CNCF，AI 训练事实标准）、**Koordinator**、Scheduler Plugins 的 Coscheduling。

## 七、主线总结

{% mermaid %}
graph LR
    A[Device Plugin<br/>让 K8s 认识 GPU] --> B[共享方案<br/>让 GPU 被复用]
    B --> C[拓扑感知<br/>让 GPU 跑得快]
    C --> D[Gang Scheduling<br/>让训练调得动]
{% endmermaid %}

GPU 调度的本质，是在**利用率**与**隔离性 / 性能**之间找平衡。

## 八、常见问题

- **GPU 能像 CPU 那样超卖吗？** 原生不行（requests=limits）；time-slicing / 第三方 vGPU 可"伪超卖"，但有抢占 / OOM 风险。
- **MIG 和 time-slicing 怎么选？** 生产多租户要强隔离选 MIG；开发测试 / 低负载推理用 time-slicing 省成本。
- **调度器知道分的是哪张卡吗？** 不知道，它只数数，具体卡由 kubelet + Device Plugin 在节点本地分配。
- **多机多卡训练为什么慢？** 大概率拓扑没对齐（跨 NUMA/PCIe）或没走 RDMA/NVLink，需拓扑感知调度 + NCCL 调优。
- **怎么监控 GPU 利用率？** DCGM Exporter + Prometheus + Grafana，看 SM 利用率、显存、温度。
