# K8s APF 公平限流 & Shuffle Sharding —— 技术分享材料

本目录是为「K8s API Priority & Fairness」技术分享会准备的全套材料。

## 目录结构

```
talks/apf-fairness/
├── README.md          # 本文件
├── slides.md          # Marp 幻灯片
└── demo/
    ├── 01-restrict-prioritylevel.yaml  # 低并发隔离优先级
    ├── 02-restrict-flowschema.yaml     # 把某 SA 分类到该优先级
    └── demo.sh                         # 现场演示串讲脚本
```

## 配套博客文章

正文已发布到 Hexo 站点：`source/_posts/k8s-apf-fairness.md`（含 Mermaid 图：旧限流雪崩、请求分类、Shuffle Sharding 手牌、整体流程）。

## 幻灯片放映 / 导出

推荐用 **VS Code + Marp for VS Code 插件**：打开 `slides.md` → 预览 → 命令面板 `Marp: Export Slide Deck` 导出 PDF/PPTX。

## 演示步骤

```bash
cd demo
source demo.sh
step1_builtin        # 看内置优先级与流模式
step2_headers        # 看请求命中的 FlowSchema/优先级(响应头)
step3_apply_restrict # 创建隔离用的优先级 + 流模式 + 测试 SA
step4_verify         # 用该 SA 身份发请求，确认命中隔离优先级
step5_metrics        # 看 APF 监控指标
cleanup              # 清理
```

> APF 在 K8s 1.20+ 默认开启。演示需要 kubectl 与集群的 `flowcontrol.apiserver.k8s.io/v1` API（1.29+ 为 v1；更早版本用 v1beta3/v1beta2，记得调整 apiVersion）。

## 分享主线

请求 → FlowSchema 分类 → PriorityLevel 分并发预算 → Shuffle Sharding 抽手牌选最短队列 → 公平排队 → 执行 / 429 退避

## 一句话精髓

把"一个全局大池子"重构为 **分优先级（保护关键流量）+ 租户隔离（Shuffle Sharding）+ 公平排队** 的多级限流系统。
