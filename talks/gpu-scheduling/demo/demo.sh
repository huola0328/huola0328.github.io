#!/bin/bash
# GPU 调度分享 - 现场演示串讲脚本
# 用法：分段手动执行，不要一次性全跑。每个函数对应一个演示环节。
# 前提：已有带 GPU 的 K8s 集群 + 已安装 NVIDIA Device Plugin / GPU Operator。

set -uo pipefail
DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

pause() { read -rp $'\n>>> 按回车继续...\n'; }

# 环节 0：确认集群能看到 GPU 资源
step0_capacity() {
  echo "===== 节点上报的 GPU 资源（Extended Resource）====="
  kubectl get nodes -o json | jq -r '.items[] | "\(.metadata.name): \(.status.capacity["nvidia.com/gpu"] // "无GPU")"'
  echo
  echo "===== Device Plugin DaemonSet ====="
  kubectl get pods -A -o wide | grep -i -E 'device-plugin|nvidia' || echo "未发现 device plugin"
}

# 环节 1：申请一张 GPU，进容器看 nvidia-smi
step1_single_gpu() {
  kubectl apply -f "$DEMO_DIR/01-gpu-pod.yaml"
  echo "等待 Pod 就绪..."
  kubectl wait --for=condition=Ready pod/gpu-demo --timeout=120s
  echo "===== 容器内 nvidia-smi ====="
  kubectl logs gpu-demo
}

# 环节 2：两个 Pod 共享同一张卡（需先应用 time-slicing 配置）
step2_shared() {
  echo "提示：确保已应用 02-time-slicing-config.yaml 并让 device plugin 生效"
  kubectl apply -f "$DEMO_DIR/03-shared-gpu-pods.yaml"
  kubectl wait --for=condition=Ready pod/shared-gpu-a pod/shared-gpu-b --timeout=120s
  echo "===== Pod A 看到的 GPU UUID ====="
  kubectl logs shared-gpu-a
  echo "===== Pod B 看到的 GPU UUID（应与 A 相同物理卡）====="
  kubectl logs shared-gpu-b
}

# 环节 3：Volcano Gang Scheduling
step3_gang() {
  echo "===== 提交需要 4 GPU 的成组训练任务 ====="
  kubectl apply -f "$DEMO_DIR/04-volcano-gang-job.yaml"
  echo "观察：资源不足时 4 个 worker 都处于 Pending（不会部分启动占资源）"
  kubectl get pods -l volcano.sh/job-name=distributed-training -w
}

# 环节 4：查看节点拓扑 label
step4_topology() {
  NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
  echo "===== 节点 $NODE 的 GPU / 拓扑相关 label ====="
  kubectl get node "$NODE" -o json | jq '.metadata.labels | with_entries(select(.key | test("nvidia|gpu|numa";"i")))'
}

# 清理
cleanup() {
  kubectl delete -f "$DEMO_DIR/01-gpu-pod.yaml" --ignore-not-found
  kubectl delete -f "$DEMO_DIR/03-shared-gpu-pods.yaml" --ignore-not-found
  kubectl delete -f "$DEMO_DIR/04-volcano-gang-job.yaml" --ignore-not-found
}

echo "可用函数：step0_capacity / step1_single_gpu / step2_shared / step3_gang / step4_topology / cleanup"
echo "示例：source demo.sh && step0_capacity"
