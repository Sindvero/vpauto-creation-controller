package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingcorev1 "k8s.io/api/autoscaling/v1"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/Sindvero/vpa-creation-operator/internal/metrics"
)

// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalers,verbs=get;list;create;watch
// +kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalers/status,verbs=get

type VPAControllerReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Metrics *metrics.Collectors
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

	// Clean up orphaned VPAs
	for _, vpa := range vpaList.Items {
		if vpa.OwnerReferences == nil || len(vpa.OwnerReferences) == 0 {
			logger.Info("Deleting orphaned VPA", "name", vpa.Name)
			r.Metrics.VPADeleted.WithLabelValues(vpa.Namespace).Inc()
			_ = r.Client.Delete(ctx, &vpa)
		}
	}

	kinds := []client.Object{
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&appsv1.StatefulSet{},
	}
	for _, obj := range kinds {
		obj = obj.DeepCopyObject().(client.Object)
		obj.SetNamespace(req.Namespace)
		obj.SetName(req.Name)
		if err := r.Client.Get(ctx, req.NamespacedName, obj); err == nil {
			return r.handleReconcile(ctx, obj)
		} else if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	logger.Info("No matching resource found for request", "name", req.NamespacedName)
	return ctrl.Result{}, nil
}

func (r *VPAControllerReconciler) handleReconcile(ctx context.Context, obj client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	annotations := obj.GetAnnotations()
	if val, ok := annotations[vpaAnnotationKey]; !ok || val != "true" {
		return ctrl.Result{}, nil
	}

	vpaName := obj.GetName() + "-vpa"
	var existingVPA autoscalingv1.VerticalPodAutoscaler
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: vpaName}, &existingVPA); errors.IsNotFound(err) {
		selector := extractSelector(obj)
		kind := getKind(obj)
		vpa := r.generateVPA(vpaName, obj.GetNamespace(), selector, kind, obj)
		logger.Info("Creating VPA", "name", vpa.Name)
		if err := r.Client.Create(ctx, &vpa); err != nil {
			logger.Error(err, "Failed to create VPA", "name", vpa.Name)
			return ctrl.Result{}, err
		}
		r.Metrics.VPACreated.WithLabelValues(kind, obj.GetNamespace()).Inc()
	}

	return ctrl.Result{}, nil
}

func extractSelector(obj runtime.Object) *metav1.LabelSelector {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return o.Spec.Selector
	case *appsv1.DaemonSet:
		return o.Spec.Selector
	case *appsv1.StatefulSet:
		return o.Spec.Selector
	default:
		return &metav1.LabelSelector{}
	}
}

func getKind(obj client.Object) string {
	switch obj.(type) {
	case *appsv1.Deployment:
		return "Deployment"
	case *appsv1.DaemonSet:
		return "DaemonSet"
	case *appsv1.StatefulSet:
		return "StatefulSet"
	default:
		return "Unknown"
	}
}

func (r *VPAControllerReconciler) generateVPA(name, namespace string, selector *metav1.LabelSelector, kind string, owner client.Object) autoscalingv1.VerticalPodAutoscaler {
	vpa := autoscalingv1.VerticalPodAutoscaler{
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
	_ = ctrl.SetControllerReference(owner, &vpa, r.Scheme)
	return vpa
}

func (r *VPAControllerReconciler) SetupWithManagerFor(obj client.Object, mgr ctrl.Manager) error {
	hasAnnotation := predicate.NewPredicateFuncs(func(o client.Object) bool {
		val, ok := o.GetAnnotations()[vpaAnnotationKey]
		return ok && val == "true"
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("vpauto-"+getKind(obj)).
		For(obj, builder.WithPredicates(hasAnnotation)).
		Complete(r)
}
