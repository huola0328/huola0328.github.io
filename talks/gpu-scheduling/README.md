# K8s GPU / 异构资源调度 —— 技术分享材料

本目录是为「K8s GPU 调度」技术分享会准备的全套材料。

## 目录结构

```
talks/gpu-scheduling/
├── README.md          # 本文件
├── slides.md          # Marp 幻灯片（可直接放映/导出 PDF）
└── demo/
    ├── 01-gpu-pod.yaml              # 演示1：申请1张GPU的Pod
    ├── 02-time-slicing-config.yaml  # 演示2：时间片共享配置
    ├── 03-shared-gpu-pods.yaml      # 演示3：两Pod共享一张卡
    ├── 04-volcano-gang-job.yaml     # 演示4：Volcano成组调度
    └── demo.sh                      # 现场演示串讲脚本
```

## 配套博客文章

正文已发布到 Hexo 站点：`source/_posts/k8s-gpu-scheduling.md`（含 Mermaid 时序图与拓扑图）。

## 幻灯片放映 / 导出

安装 Marp CLI 后：

```bash
# 实时预览
npx @marp-team/marp-cli slides.md --preview

# 导出 PDF
npx @marp-team/marp-cli slides.md --pdf

# 导出 HTML（可直接浏览器打开放映）
npx @marp-team/marp-cli slides.md --html
```

或在 VS Code 安装 **Marp for VS Code** 插件，打开 `slides.md` 直接预览。

## 演示脚本用法

```bash
cd demo
source demo.sh
step0_capacity     # 看节点 GPU 资源
step1_single_gpu   # 单卡 Pod
step2_shared       # 共享一张卡
step3_gang         # Volcano 成组调度
step4_topology     # 查看拓扑 label
cleanup            # 清理
```

> 演示需要一个带 GPU 的 K8s 集群，并已安装 NVIDIA Device Plugin 或 GPU Operator。
> Volcano 演示需先安装 Volcano。

## 分享主线

Device Plugin（认识）→ 共享方案（复用）→ 拓扑感知（跑得快）→ Gang Scheduling（调得动）
