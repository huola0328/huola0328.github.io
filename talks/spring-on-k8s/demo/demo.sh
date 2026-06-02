#!/bin/bash
# Spring Boot 上 K8s 现场演示串讲脚本
# 前提：有 K8s 集群 + kubectl；演示扩容需安装 metrics-server。
# 注意：构建镜像需要 JDK + Maven 工程（本目录 app/ 是关键源码片段，需放进标准 Spring Boot 工程）。

set -uo pipefail
DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 环节 1：构建镜像（不写 Dockerfile）
step1_build() {
  echo "===== 用 Buildpacks 打镜像（在 Spring Boot 工程根目录执行）====="
  echo "mvn spring-boot:build-image -Dspring-boot.build-image.imageName=myorg/demo:1.0"
  echo "（或 ./gradlew bootBuildImage --imageName=myorg/demo:1.0）"
}

# 环节 2：部署到 K8s
step2_deploy() {
  echo "===== 应用 ConfigMap / Deployment / Service ====="
  kubectl apply -f "$DEMO_DIR/k8s/01-configmap.yaml"
  kubectl apply -f "$DEMO_DIR/k8s/02-deployment.yaml"
  kubectl apply -f "$DEMO_DIR/k8s/03-service.yaml"
  kubectl rollout status deployment/demo
}

# 环节 3：验证探针与配置注入
step3_verify() {
  echo "===== Pod 与探针状态 ====="
  kubectl get pods -l app=demo
  echo "===== 端口转发后访问，应返回 ConfigMap 注入的问候语 ====="
  echo "kubectl port-forward svc/demo 8080:80 &"
  echo "curl localhost:8080/                       # Hello from ConfigMap | host=demo-xxxx"
  echo "curl localhost:8080/actuator/health/readiness"
}

# 环节 4：演示优雅停机（滚动更新不丢请求）
step4_rollout() {
  echo "===== 触发滚动更新，观察零中断 ====="
  kubectl set image deployment/demo demo=myorg/demo:1.1
  kubectl rollout status deployment/demo
}

# 环节 5：演示 HPA 自动扩容
step5_hpa() {
  kubectl apply -f "$DEMO_DIR/k8s/04-hpa.yaml"
  echo "===== 持续压测 /burn 接口，观察副本数上涨 ====="
  echo "# 一个终端压测："
  echo "kubectl run load --image=busybox --restart=Never -- /bin/sh -c \\"
  echo "  'while true; do wget -q -O- http://demo/burn; done'"
  echo "# 另一个终端观察："
  kubectl get hpa demo -w
}

cleanup() {
  kubectl delete -f "$DEMO_DIR/k8s/" --ignore-not-found
  kubectl delete pod load --ignore-not-found
}

echo "可用函数：step1_build / step2_deploy / step3_verify / step4_rollout / step5_hpa / cleanup"
echo "示例：source demo.sh && step2_deploy"
