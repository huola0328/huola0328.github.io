#!/bin/bash
# APF 现场演示串讲脚本
# 前提：有可用 K8s 集群 + kubectl（APF 默认开启，K8s 1.20+）

set -uo pipefail
DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 环节 1：查看内置的优先级与流模式
step1_builtin() {
  echo "===== 内置 PriorityLevelConfiguration ====="
  kubectl get prioritylevelconfigurations
  echo
  echo "===== 内置 FlowSchema（按 matchingPrecedence 排序）====="
  kubectl get flowschemas
}

# 环节 2：看一个请求被分到了哪个 FlowSchema / 优先级（响应头）
step2_headers() {
  echo "===== 请求的 APF 响应头 ====="
  kubectl get --raw='/api/v1/namespaces/default/pods?limit=1' -v=8 2>&1 \
    | grep -i 'X-Kubernetes-PF' || echo "（如未显示，升高 -v 或检查 APF 是否开启）"
}

# 环节 3：创建"吵闹"客户端的隔离优先级 + 流模式
step3_apply_restrict() {
  echo "===== 创建低并发优先级 + 绑定 FlowSchema ====="
  kubectl create serviceaccount noisy-sa -n default --dry-run=client -o yaml | kubectl apply -f -
  # 给它一点读权限，便于演示发请求
  kubectl create clusterrolebinding noisy-sa-view \
    --clusterrole=view --serviceaccount=default:noisy-sa \
    --dry-run=client -o yaml | kubectl apply -f -
  kubectl apply -f "$DEMO_DIR/01-restrict-prioritylevel.yaml"
  kubectl apply -f "$DEMO_DIR/02-restrict-flowschema.yaml"
  kubectl get flowschema restrict-noisy-sa -o wide
}

# 环节 4：用 noisy-sa 的身份发请求，确认它命中了 restrict 优先级
step4_verify() {
  echo "===== 用 noisy-sa token 发请求，查看 APF 头 ====="
  TOKEN=$(kubectl create token noisy-sa -n default)
  APISERVER=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
  curl -sk -H "Authorization: Bearer $TOKEN" \
    "$APISERVER/api/v1/namespaces/default/pods?limit=1" -D - -o /dev/null \
    | grep -i 'X-Kubernetes-PF'
  echo ">>> 期望看到 PriorityLevel 指向 restrict-noisy 对应的 UID"
}

# 环节 5：观察 APF 监控指标
step5_metrics() {
  echo "===== APF 关键指标 ====="
  kubectl get --raw /metrics 2>/dev/null \
    | grep -E 'apiserver_flowcontrol_(rejected_requests_total|current_executing_requests|current_inqueue_requests)' \
    | grep -v '^#' | head -30
}

# 清理
cleanup() {
  kubectl delete -f "$DEMO_DIR/02-restrict-flowschema.yaml" --ignore-not-found
  kubectl delete -f "$DEMO_DIR/01-restrict-prioritylevel.yaml" --ignore-not-found
  kubectl delete clusterrolebinding noisy-sa-view --ignore-not-found
  kubectl delete serviceaccount noisy-sa -n default --ignore-not-found
}

echo "可用函数：step1_builtin / step2_headers / step3_apply_restrict / step4_verify / step5_metrics / cleanup"
echo "示例：source demo.sh && step1_builtin"
