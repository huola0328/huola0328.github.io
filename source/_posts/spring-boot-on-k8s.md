---
title: 从一行 Java 代码到弹性伸缩——Spring Boot 应用上 K8s 全链路实战
date: 2026-06-01 17:40:00
tags: [Spring, Kubernetes, SpringBoot, 云原生]
categories: 技术分享
cover: /images/cover-operator.jpg
---

> 写完一个 Spring Boot 应用，怎么让它在 K8s 上**自动重启、滚动更新、不中断流量、还能按负载自动扩缩容**？
> 本文走一遍完整链路：**打镜像 → 部署 → 健康探针 → 配置注入 → 优雅停机 → 自动扩缩容**，把 Spring 和 K8s 真正打通。

## 一、全景：一个 Spring Boot 应用的上云之路

{% mermaid %}
graph LR
    A[Spring Boot 代码] --> B[构建镜像<br/>Buildpacks / Jib]
    B --> C[推送镜像仓库]
    C --> D[K8s Deployment 拉取]
    D --> E[Pod 运行]
    E --> F[Service 暴露]
    E --> G[Actuator 探针<br/>健康检查]
    E --> H[ConfigMap/Secret<br/>注入配置]
    E --> I[Micrometer 指标<br/>驱动 HPA 扩缩容]
{% endmermaid %}

Spring 负责"把应用写好"，K8s 负责"把应用跑好、管好"。下面逐个打通。

## 二、第一步：把 Spring Boot 打成镜像（不写 Dockerfile）

Spring Boot 内置了 Buildpacks 支持，一条命令即可生成 OCI 镜像：

```bash
# Maven
mvn spring-boot:build-image -Dspring-boot.build-image.imageName=myorg/demo:1.0

# Gradle
./gradlew bootBuildImage --imageName=myorg/demo:1.0
```

它会自动选好 JDK、分层打包（依赖层与代码层分离，**改代码不必重传依赖层**），无需你手写 Dockerfile。也可用 **Jib**，无需本地 Docker 守护进程，适合 CI。

## 三、第二步：最小可用的 Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  replicas: 2
  selector:
    matchLabels: { app: demo }
  template:
    metadata:
      labels: { app: demo }
    spec:
      containers:
        - name: demo
          image: myorg/demo:1.0
          ports:
            - containerPort: 8080
```

但这还不够——K8s 此时**并不知道你的应用是否真的就绪**。这就要靠探针。

## 四、关键：Actuator 探针对接 K8s 健康检查

Spring Boot Actuator 专门为 K8s 暴露了两个端点：

- **liveness（存活）**：应用是否"活着"，挂了就**重启容器**。
- **readiness（就绪）**：应用是否"能接流量"，没就绪就**从 Service 摘除**，不转发请求。

{% mermaid %}
sequenceDiagram
    participant K as kubelet
    participant A as Spring Boot Actuator
    K->>A: GET /actuator/health/liveness
    A-->>K: 200 UP / 非200 DOWN
    Note over K: DOWN → 重启容器
    K->>A: GET /actuator/health/readiness
    A-->>K: 200 UP / 503 OUT_OF_SERVICE
    Note over K: 未就绪 → 从 Service 端点摘除
{% endmermaid %}

开启（`application.yml`）：

```yaml
management:
  endpoint:
    health:
      probes:
        enabled: true       # 暴露 liveness/readiness 子端点
  health:
    livenessstate:
      enabled: true
    readinessstate:
      enabled: true
```

Deployment 里配置探针：

```yaml
          livenessProbe:
            httpGet: { path: /actuator/health/liveness, port: 8080 }
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet: { path: /actuator/health/readiness, port: 8080 }
            initialDelaySeconds: 5
            periodSeconds: 5
```

> 关键点：**liveness 失败会重启，readiness 失败只摘流量不重启**。配错了（比如把慢启动配成 liveness）会导致容器无限重启。

## 五、配置外置：ConfigMap / Secret 注入

代码和配置分离，是云原生的基本原则。把配置放进 ConfigMap，注入容器：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo-config
data:
  APP_GREETING: "Hello from ConfigMap"
  SPRING_PROFILES_ACTIVE: "prod"
```

Deployment 里用 `envFrom` 一次性注入：

```yaml
          envFrom:
            - configMapRef:
                name: demo-config
```

Spring Boot 会自动把环境变量 `APP_GREETING` 映射到 `app.greeting` 属性。敏感信息（密码、密钥）则放 **Secret**，用法一样。

> 进阶：用 **Spring Cloud Kubernetes** 可直接读取 ConfigMap 并支持**热更新**，改配置不用重启 Pod。

## 六、优雅停机：滚动更新不丢请求

K8s 滚动更新或缩容时，会给 Pod 发 `SIGTERM`。如果应用立刻退出，**在途请求会被中断**。Spring Boot 支持优雅停机：

```yaml
server:
  shutdown: graceful          # 停机前先处理完在途请求
spring:
  lifecycle:
    timeout-per-shutdown-phase: 30s
```

配合 K8s 的 `terminationGracePeriodSeconds` 和 `preStop` 钩子，实现**零中断发布**。

{% mermaid %}
sequenceDiagram
    participant K as K8s
    participant P as Pod(Spring Boot)
    K->>P: 1. 从 Service 摘除(不再转发新请求)
    K->>P: 2. 发送 SIGTERM
    P->>P: 3. 处理完在途请求(graceful)
    P-->>K: 4. 进程退出
    Note over K,P: 在途请求不中断 = 零停机发布
{% endmermaid %}

## 七、自动扩缩容：Micrometer + HPA

让应用扛住流量高峰、闲时省钱。Spring 用 **Micrometer** 暴露指标，K8s 用 **HPA（HorizontalPodAutoscaler）** 据此增减副本：

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: demo
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: demo
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70   # CPU 超 70% 就扩容
```

{% mermaid %}
graph LR
    A[流量上涨] --> B[CPU 使用率升高]
    B --> C[HPA 检测到超阈值]
    C --> D[增加 Pod 副本]
    D --> E[负载摊薄, CPU 回落]
    E -.闲时.-> F[HPA 缩减副本省资源]
{% endmermaid %}

> 前提：容器必须设置 `resources.requests.cpu`，HPA 才能算出"使用率"。

## 八、性能加分项：GraalVM 原生镜像

如果追求极致弹性，可用 **Spring Boot 3 + GraalVM Native Image** 把应用 AOT 编译成原生可执行文件：

- 启动时间：**秒级 → 几十毫秒**
- 内存占用：**大幅下降**

```bash
mvn -Pnative spring-boot:build-image
```

这对 K8s 上的**快速扩容、Serverless、冷启动**意义重大——Pod 几乎瞬间就绪。

## 九、一图串起全链路

{% mermaid %}
graph TD
    Code[Spring Boot 代码] --> Image[Buildpacks 打镜像]
    Image --> Deploy[Deployment 部署]
    Deploy --> Probe[Actuator 探针: 自愈+摘流量]
    Deploy --> Config[ConfigMap/Secret 注入配置]
    Deploy --> Graceful[优雅停机: 零中断发布]
    Deploy --> HPA[Micrometer+HPA: 弹性伸缩]
{% endmermaid %}

Spring 和 K8s 的结合，本质是**让应用层（Spring）和平台层（K8s）各司其职又精准协作**：应用暴露自己的"健康、配置、指标"，平台据此完成"自愈、配置、伸缩、零停机"。

## 十、常见问题

- **探针该用 HTTP 还是 TCP？** 优先 Actuator 的 HTTP 健康端点，语义最准确。
- **liveness 一直失败导致反复重启？** 多半是把"启动慢"误配成 liveness，应改用 `startupProbe` 保护慢启动。
- **改了 ConfigMap 要重启吗？** 默认环境变量方式要重启；用 Spring Cloud Kubernetes 或挂载文件 + 监听可热更新。
- **HPA 不扩容？** 检查是否设了 `requests.cpu`、metrics-server 是否安装。
- **为什么要优雅停机？** 否则滚动更新时正在处理的请求会被强杀，用户看到 5xx。
