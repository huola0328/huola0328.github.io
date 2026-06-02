#!/bin/bash
# Operator 现场演示 - 从零创建一个 Website Operator
# 前提：已装 go (>=1.21)、kubebuilder、kubectl，且有可用 K8s 集群（kind/minikube 即可）
# 用法：分步执行，不要一次性全跑。

set -euo pipefail
PROJECT_DIR="${1:-/tmp/website-operator}"

step1_init() {
  mkdir -p "$PROJECT_DIR"
  cd "$PROJECT_DIR"
  echo "===== 1. 初始化 kubebuilder 项目 ====="
  kubebuilder init --domain example.com --repo example.com/website-operator
}

step2_api() {
  cd "$PROJECT_DIR"
  echo "===== 2. 创建 API：group=web version=v1 kind=Website ====="
  # --resource 生成 CRD 类型，--controller 生成控制器骨架
  kubebuilder create api --group web --version v1 --kind Website --resource --controller
  echo ">>> 现在用本 demo 目录里的 website_types.go 和 website_controller.go 覆盖生成的骨架"
  echo ">>> 类型文件：api/v1/website_types.go"
  echo ">>> 控制器：  internal/controller/website_controller.go"
}

step3_install() {
  cd "$PROJECT_DIR"
  echo "===== 3. 生成清单并安装 CRD ====="
  make manifests
  make install
  kubectl get crd | grep websites
}

step4_run() {
  cd "$PROJECT_DIR"
  echo "===== 4. 本地运行控制器（前台，Ctrl+C 退出）====="
  make run
}

step5_demo() {
  echo "===== 5. 演示：创建一个 Website，观察自动生成的 Deployment ====="
  kubectl apply -f "$(dirname "${BASH_SOURCE[0]}")/sample-website.yaml"
  sleep 2
  kubectl get website
  kubectl get deploy,svc -l app=demo-site
  echo ">>> 修改 replicas 看控制器秒级对齐："
  echo "    kubectl patch website demo-site --type merge -p '{\"spec\":{\"replicas\":4}}'"
  echo ">>> 删除 Website 看子资源级联消失："
  echo "    kubectl delete website demo-site"
}

echo "项目目录：$PROJECT_DIR"
echo "可用函数：step1_init / step2_api / step3_install / step4_run / step5_demo"
echo "示例：source setup.sh /tmp/website-operator && step1_init"
