# Spring Boot 应用上 K8s 全链路 —— 技术分享材料

本目录是为「Spring Boot + K8s」技术分享会准备的全套材料。

## 目录结构

```
talks/spring-on-k8s/
├── README.md          # 本文件
├── slides.md          # Marp 幻灯片
└── demo/
    ├── demo.sh                  # 现场演示串讲脚本
    ├── app/
    │   ├── DemoApplication.java # 最小 Spring Boot 应用（含 / 与 /burn 接口）
    │   └── application.yml       # 开启探针端点 + 优雅停机
    └── k8s/
        ├── 01-configmap.yaml     # 配置外置
        ├── 02-deployment.yaml    # 探针 + 配置注入 + 资源限额
        ├── 03-service.yaml       # 服务暴露
        └── 04-hpa.yaml           # CPU 自动扩缩容
```

## 配套博客文章

正文已发布到 Hexo 站点：`source/_posts/spring-boot-on-k8s.md`（含 Mermaid 全景图、探针时序图、优雅停机时序图、HPA 循环图）。

## 幻灯片放映 / 导出

推荐用 **VS Code + Marp for VS Code 插件**：打开 `slides.md` → 预览 → 命令面板 `Marp: Export Slide Deck` 导出 PDF/PPTX。

## 演示前置依赖

- JDK 17+ 与 Maven/Gradle（构建镜像）
- 一个标准 Spring Boot 3 工程（把 `app/` 下的两个文件放进去）
- K8s 集群 + kubectl
- 演示自动扩容需安装 **metrics-server**

## 演示步骤

```bash
cd demo
source demo.sh
step1_build     # 用 Buildpacks 打镜像（一条命令，无需 Dockerfile）
step2_deploy    # 部署 ConfigMap/Deployment/Service
step3_verify    # 验证探针 + ConfigMap 注入生效
step4_rollout   # 滚动更新演示优雅停机
step5_hpa       # 压测 /burn，观察 HPA 自动扩容
cleanup         # 清理
```

## 分享主线

代码 → Buildpacks 打镜像 → Deployment 部署 → Actuator 探针(自愈+摘流量)
→ ConfigMap/Secret(配置) → 优雅停机(零中断) → Micrometer+HPA(弹性伸缩)

## 一句话精髓

让**应用层（Spring）暴露健康/配置/指标**，**平台层（K8s）据此完成自愈、配置、伸缩、零停机**——各司其职又精准协作。
