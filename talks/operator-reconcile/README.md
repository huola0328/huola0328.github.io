# K8s 控制器 Reconcile 机制 & Operator 开发 —— 技术分享材料

本目录是为「K8s 控制器 / Operator」技术分享会准备的全套材料。

## 目录结构

```
talks/operator-reconcile/
├── README.md          # 本文件
├── slides.md          # Marp 幻灯片（可放映/导出 PDF、PPTX）
└── demo/
    ├── setup.sh                # 从零创建 Website Operator 的分步脚本
    ├── website_types.go        # CRD 类型定义（覆盖 kubebuilder 骨架）
    ├── website_controller.go   # Reconcile 控制器实现（核心）
    └── sample-website.yaml     # 示例自定义资源
```

## 配套博客文章

正文已发布到 Hexo 站点：`source/_posts/k8s-operator-reconcile.md`（含 Mermaid 时序图、Reconcile 流程图）。

## 幻灯片放映 / 导出

推荐用 **VS Code + Marp for VS Code 插件**：打开 `slides.md` → 预览 → 命令面板 `Marp: Export Slide Deck` 导出 PDF/PPTX。
（命令行 `marp-cli` 导出 PDF/PPTX 需要本机有 Chrome/Chromium，否则会卡住；本环境暂无浏览器。）

## 演示前置依赖

- Go >= 1.21
- kubebuilder（https://book.kubebuilder.io/quick-start）
- kubectl + 一个 K8s 集群（kind / minikube 即可）

## 演示步骤

```bash
cd demo
source setup.sh /tmp/website-operator
step1_init      # kubebuilder init
step2_api       # 创建 Website API，然后用本目录的 .go 覆盖骨架
step3_install   # make manifests && make install
step4_run       # make run（前台运行控制器）
# 另开一个终端：
step5_demo      # apply 示例 CR，观察自动生成 Deployment
```

现场亮点：

1. `kubectl apply -f sample-website.yaml` → 自动出现 Deployment + Pod
2. `kubectl patch website demo-site -p '{"spec":{"replicas":4}}'` → 控制器秒级对齐
3. `kubectl delete website demo-site` → 子资源（Deployment）级联消失（OwnerReference 的威力）

## 分享主线

声明式 Spec → List-Watch 感知 → WorkQueue 去重/重试 → Reconcile 幂等对齐 → 更新 Status → 持续循环
