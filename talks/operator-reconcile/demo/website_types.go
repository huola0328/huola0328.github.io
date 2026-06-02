// 用本文件覆盖 kubebuilder 生成的 api/v1/website_types.go 中的类型定义部分。
// 这是一个极简 CRD：声明一个 Website，期望它对应一个跑指定镜像、指定副本数的服务。
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WebsiteSpec 是用户声明的"期望态"
type WebsiteSpec struct {
	// 要部署的容器镜像，例如 nginx:1.27
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// 副本数，默认 1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// 对外暴露的端口，默认 80
	// +kubebuilder:default=80
	Port int32 `json:"port,omitempty"`
}

// WebsiteStatus 是控制器回写的"实际态"
type WebsiteStatus struct {
	// 当前可用副本数
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// 状态条件（标准做法，便于 kubectl describe 展示）
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.availableReplicas`

// Website 是我们的自定义资源
type Website struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebsiteSpec   `json:"spec,omitempty"`
	Status WebsiteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WebsiteList 包含 Website 列表
type WebsiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Website `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Website{}, &WebsiteList{})
}
