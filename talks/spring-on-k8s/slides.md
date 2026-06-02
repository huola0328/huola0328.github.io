---
marp: true
theme: default
paginate: true
size: 16:9
header: 'Spring Boot 应用上 K8s 全链路实战'
footer: 'huola0328'
---

<!-- _class: lead -->
<!-- _paginate: false -->

# 从一行 Java 代码
# 到弹性伸缩

## Spring Boot 应用上 K8s 全链路实战

---

<!-- _class: lead -->

# 开场：一个问题

写完一个 Spring Boot 应用

怎么让它在 K8s 上做到：

**自动重启 · 滚动更新 · 不中断流量 · 按负载自动扩缩容？**

> 今天走一遍完整链路

---

# 全景：上云之路

```
Spring Boot 代码
   → 构建镜像 (Buildpacks/Jib)
   → 推送仓库
   → Deployment 拉取运行
   → Service 暴露
   → Actuator 探针 (健康)
   → ConfigMap/Secret (配置)
   → Micrometer + HPA (弹性)
```

**Spring 把应用写好，K8s 把应用跑好、管好**

---

# 第一步：打镜像（不写 Dockerfile）

Spring Boot 内置 Buildpacks：

```bash
# 一条命令生成 OCI 镜像
mvn spring-boot:build-image \
  -Dspring-boot.build-image.imageName=myorg/demo:1.0
```

- 自动选 JDK、**分层打包**（依赖层/代码层分离）
- 改代码不必重传依赖层
- 或用 **Jib**：无需 Docker 守护进程，适合 CI

---

# 第二步：最小 Deployment

```yaml
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: demo
          image: myorg/demo:1.0
          ports:
            - containerPort: 8080
```

**问题**：此时 K8s 并不知道你的应用是否真的就绪
→ 需要探针

---

<!-- _class: lead -->

# 关键一：Actuator 探针
## 对接 K8s 健康检查

---

# 两种探针，两种语义

**liveness（存活）**
应用挂了吗？ → 挂了就**重启容器**

**readiness（就绪）**
能接流量吗？ → 没就绪就**从 Service 摘除**（不重启）

```
kubelet → GET /actuator/health/liveness  → 非200 → 重启
kubelet → GET /actuator/health/readiness → 503  → 摘流量
```

---

# 配置探针

application.yml：
```yaml
management:
  endpoint:
    health:
      probes:
        enabled: true
```

Deployment：
```yaml
livenessProbe:
  httpGet: { path: /actuator/health/liveness, port: 8080 }
readinessProbe:
  httpGet: { path: /actuator/health/readiness, port: 8080 }
```

⚠️ 慢启动别配成 liveness，否则无限重启 → 用 startupProbe

---

# 关键二：配置外置 ConfigMap

代码与配置分离：

```yaml
kind: ConfigMap
data:
  APP_GREETING: "Hello from ConfigMap"
  SPRING_PROFILES_ACTIVE: "prod"
```

Deployment 注入：
```yaml
envFrom:
  - configMapRef: { name: demo-config }
```

- 环境变量 `APP_GREETING` 自动映射到 `app.greeting`
- 密码密钥放 **Secret**，用法一样
- Spring Cloud Kubernetes 可**热更新**

---

# 关键三：优雅停机（零中断发布）

滚动更新时 K8s 发 SIGTERM，应用若立刻退出 → 在途请求被杀

```yaml
server:
  shutdown: graceful
spring:
  lifecycle:
    timeout-per-shutdown-phase: 30s
```

```
1. 从 Service 摘除(不再来新请求)
2. SIGTERM
3. 处理完在途请求
4. 退出  → 零停机
```

---

# 关键四：弹性伸缩 Micrometer + HPA

```yaml
kind: HorizontalPodAutoscaler
spec:
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          averageUtilization: 70   # 超70%扩容
```

```
流量↑ → CPU↑ → HPA扩容 → 负载摊薄 → 闲时缩容省钱
```

⚠️ 必须设 `requests.cpu`，HPA 才能算使用率

---

# 加分项：GraalVM 原生镜像

Spring Boot 3 + GraalVM，AOT 编译成原生可执行文件：

- 启动：**秒级 → 几十毫秒**
- 内存：**大幅下降**

```bash
mvn -Pnative spring-boot:build-image
```

对 K8s **快速扩容 / Serverless / 冷启动** 意义重大
→ Pod 几乎瞬间就绪

---

<!-- _class: lead -->

# 全链路总结

代码 → Buildpacks 打镜像 → Deployment 部署
→ Actuator 探针(自愈+摘流量)
→ ConfigMap/Secret(配置) → 优雅停机(零中断)
→ Micrometer+HPA(弹性伸缩)

## 应用层与平台层各司其职、精准协作

---

# Q&A 预备

- **探针用 HTTP 还是 TCP？** 优先 Actuator 的 HTTP 端点，语义最准
- **反复重启？** 慢启动误配成 liveness，改用 startupProbe
- **改 ConfigMap 要重启吗？** 环境变量方式要；Spring Cloud K8s 可热更新
- **HPA 不扩容？** 查 requests.cpu 和 metrics-server
- **为什么要优雅停机？** 否则发布时在途请求被强杀，用户看到 5xx

---

<!-- _class: lead -->
<!-- _paginate: false -->

# 谢谢

## Q & A
