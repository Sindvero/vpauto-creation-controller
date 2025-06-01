package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingcorev1 "k8s.io/api/autoscaling/v1"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalers,verbs=get;list;create;watch
// +kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalers/status,verbs=get

type VPAControllerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const vpaAnnotationKey = "k8s.autoscaling.vpacreation/vpa-enabled"

func (r *VPAControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Checking for the existence of VPA CRD
	var vpaList autoscalingv1.VerticalPodAutoscalerList
	err := r.Client.List(ctx, &vpaList)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("VPA CRD not found, will retry later")
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to list VPA resources")
		return ctrl.Result{}, err
	}

	// Process Deployments
	var deployments appsv1.DeploymentList
	r.Client.List(ctx, &deployments)
	for _, dep := range deployments.Items {
		if val, ok := dep.Annotations[vpaAnnotationKey]; ok && val == "true" {
			vpaName := dep.Name + "-vpa"
			var existingVPA autoscalingv1.VerticalPodAutoscaler
			err := r.Client.Get(ctx, client.ObjectKey{Namespace: dep.Namespace, Name: vpaName}, &existingVPA)
			if errors.IsNotFound(err) {
				vpa := generateVPA(dep.Name+"-vpa", dep.Namespace, dep.Spec.Selector, "Deployment")
				logger.Info("Creating VPA for Deployment", "name", vpa.Name)
				r.Client.Create(ctx, &vpa)
			}
		}
	}

	// Process DaemonSets
	var daemonSets appsv1.DaemonSetList
	r.Client.List(ctx, &daemonSets)
	for _, ds := range daemonSets.Items {
		if val, ok := ds.Annotations[vpaAnnotationKey]; ok && val == "true" {
			vpaName := ds.Name + "-vpa"
			var existingVPA autoscalingv1.VerticalPodAutoscaler
			err := r.Client.Get(ctx, client.ObjectKey{Namespace: ds.Namespace, Name: vpaName}, &existingVPA)
			if errors.IsNotFound(err) {
				vpa := generateVPA(ds.Name+"-vpa", ds.Namespace, ds.Spec.Selector, "DaemonSet")
				logger.Info("Creating VPA for DaemonSet", "name", vpa.Name)
				r.Client.Create(ctx, &vpa)
			}
		}
	}

	// Process StatefulSets
	var statefulSets appsv1.StatefulSetList
	r.Client.List(ctx, &statefulSets)
	for _, sts := range statefulSets.Items {
		if val, ok := sts.Annotations[vpaAnnotationKey]; ok && val == "true" {
			vpaName := sts.Name + "-vpa"
			var existingVPA autoscalingv1.VerticalPodAutoscaler
			err := r.Client.Get(ctx, client.ObjectKey{Namespace: sts.Namespace, Name: vpaName}, &existingVPA)
			if errors.IsNotFound(err) {
				vpa := generateVPA(sts.Name+"-vpa", sts.Namespace, sts.Spec.Selector, "StatefulSet")
				logger.Info("Creating VPA for StatefulSet", "name", vpa.Name)
				r.Client.Create(ctx, &vpa)
			}
		}
	}

	return ctrl.Result{}, nil
}

func generateVPA(name, namespace string, selector *metav1.LabelSelector, kind string) autoscalingv1.VerticalPodAutoscaler {
	return autoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: autoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingcorev1.CrossVersionObjectReference{
				Kind:       kind,
				Name:       name[:len(name)-4], // strip "-vpa"
				APIVersion: "apps/v1",
			},
			UpdatePolicy: &autoscalingv1.PodUpdatePolicy{
				UpdateMode: func() *autoscalingv1.UpdateMode {
					mode := autoscalingv1.UpdateModeOff
					return &mode
				}(),
			},
		},
	}
}

func (r *VPAControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
