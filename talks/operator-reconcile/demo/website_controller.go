// 用本文件覆盖 kubebuilder 生成的 internal/controller/website_controller.go。
// 核心是 Reconcile()：幂等地把"期望态(Website)"对齐为"实际态(Deployment)"。
package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	webv1 "example.com/website-operator/api/v1"
)

type WebsiteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=web.example.com,resources=websites,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=web.example.com,resources=websites/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *WebsiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1) 读取期望态。对象不存在 = 已被删除：
	//    因为下面给 Deployment 设了 OwnerReference，GC 会自动级联删除，这里无需手动处理。
	var site webv1.Website
	if err := r.Get(ctx, req.NamespacedName, &site); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2) 构造"期望的" Deployment
	desired := r.buildDeployment(&site)
	// 设置 OwnerReference：删 Website 时自动级联删除该 Deployment
	if err := ctrl.SetControllerReference(&site, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// 3) 幂等对齐：查实际态 → 不存在则创建，存在且有差异则更新
	var current appsv1.Deployment
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), &current)
	switch {
	case errors.IsNotFound(err):
		logger.Info("创建 Deployment", "name", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err // 返回 error 会自动重新入队重试
		}
	case err != nil:
		return ctrl.Result{}, err
	default:
		// 只在副本数或镜像有差异时更新（避免无意义写）
		if *current.Spec.Replicas != site.Spec.Replicas ||
			current.Spec.Template.Spec.Containers[0].Image != site.Spec.Image {
			current.Spec.Replicas = &site.Spec.Replicas
			current.Spec.Template.Spec.Containers[0].Image = site.Spec.Image
			logger.Info("更新 Deployment 对齐期望态", "name", current.Name)
			if err := r.Update(ctx, &current); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// 4) 回写 Status，反映实际态
	site.Status.AvailableReplicas = current.Status.AvailableReplicas
	if err := r.Status().Update(ctx, &site); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WebsiteReconciler) buildDeployment(site *webv1.Website) *appsv1.Deployment {
	labels := map[string]string{"app": site.Name}
	replicas := site.Spec.Replicas
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      site.Name,
			Namespace: site.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "web",
						Image: site.Spec.Image,
						Ports: []corev1.ContainerPort{{ContainerPort: site.Spec.Port}},
					}},
				},
			},
		},
	}
}

// SetupWithManager 注册控制器：Watch Website，并 Own Deployment（子资源变化也会触发 reconcile）
func (r *WebsiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webv1.Website{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
