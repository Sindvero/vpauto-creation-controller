package controller_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	"github.com/Sindvero/vpa-creation-operator/internal/controller"
)

func setupScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, autoscalingv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	return scheme
}

func TestReconcile_CreatesVPAForAnnotatedDeployment(t *testing.T) {
	scheme := setupScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: "default",
			Annotations: map[string]string{
				"k8s.autoscaling.vpacreation/vpa-enabled": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep).Build()
	r := &controller.VPAControllerReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	res, err := r.Reconcile(context.TODO(), reconcile.Request{
		NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-deploy"},
	})

	assert.NoError(t, err)
	assert.False(t, res.Requeue)

	var vpa autoscalingv1.VerticalPodAutoscaler
	err = fakeClient.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: "test-deploy-vpa"}, &vpa)
	assert.NoError(t, err)
	assert.Equal(t, "test-deploy", vpa.Spec.TargetRef.Name)
	assert.Equal(t, "Deployment", vpa.Spec.TargetRef.Kind)
	assert.Equal(t, "apps/v1", vpa.Spec.TargetRef.APIVersion)
	assert.Equal(t, autoscalingv1.UpdateModeOff, *vpa.Spec.UpdatePolicy.UpdateMode)
}

func TestReconcile_CreatesVPAForAnnotatedDaemonSet(t *testing.T) {
	scheme := setupScheme(t)

	dep := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ds",
			Namespace: "default",
			Annotations: map[string]string{
				"k8s.autoscaling.vpacreation/vpa-enabled": "true",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep).Build()
	r := &controller.VPAControllerReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	res, err := r.Reconcile(context.TODO(), reconcile.Request{
		NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-ds"},
	})

	assert.NoError(t, err)
	assert.False(t, res.Requeue)

	var vpa autoscalingv1.VerticalPodAutoscaler
	err = fakeClient.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: "test-ds-vpa"}, &vpa)
	assert.NoError(t, err)
	assert.Equal(t, "test-ds", vpa.Spec.TargetRef.Name)
	assert.Equal(t, "DaemonSet", vpa.Spec.TargetRef.Kind)
	assert.Equal(t, "apps/v1", vpa.Spec.TargetRef.APIVersion)
	assert.Equal(t, autoscalingv1.UpdateModeOff, *vpa.Spec.UpdatePolicy.UpdateMode)
}

func TestReconcile_CreatesVPAForAnnotatedStatefulSet(t *testing.T) {
	scheme := setupScheme(t)

	dep := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "default",
			Annotations: map[string]string{
				"k8s.autoscaling.vpacreation/vpa-enabled": "true",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep).Build()
	r := &controller.VPAControllerReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	res, err := r.Reconcile(context.TODO(), reconcile.Request{
		NamespacedName: client.ObjectKey{Namespace: "default", Name: "test-sts"},
	})

	assert.NoError(t, err)
	assert.False(t, res.Requeue)

	var vpa autoscalingv1.VerticalPodAutoscaler
	err = fakeClient.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: "test-sts-vpa"}, &vpa)
	assert.NoError(t, err)
	assert.Equal(t, "test-sts", vpa.Spec.TargetRef.Name)
	assert.Equal(t, "StatefulSet", vpa.Spec.TargetRef.Kind)
	assert.Equal(t, "apps/v1", vpa.Spec.TargetRef.APIVersion)
	assert.Equal(t, autoscalingv1.UpdateModeOff, *vpa.Spec.UpdatePolicy.UpdateMode)
}

func TestReconcile_SkipsWithoutAnnotation(t *testing.T) {
	scheme := setupScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-annotated-deploy",
			Namespace: "default",
			// No annotations here
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep).Build()
	reconciler := &controller.VPAControllerReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{
		NamespacedName: client.ObjectKey{Namespace: "default", Name: "no-annotated-deploy"},
	})
	assert.NoError(t, err)

	var vpa autoscalingv1.VerticalPodAutoscaler
	err = fakeClient.Get(context.TODO(), client.ObjectKey{
		Namespace: "default",
		Name:      "no-annotated-deploy-vpa",
	}, &vpa)
	assert.True(t, errors.IsNotFound(err), "VPA should not be created if annotation is missing")
}

func TestReconcile_ExistingVPA_NoDuplicate(t *testing.T) {
	scheme := setupScheme(t)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-deploy",
			Namespace: "default",
			Annotations: map[string]string{
				"k8s.autoscaling.vpacreation/vpa-enabled": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}},
				},
			},
		},
	}

	existingVPA := &autoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-deploy-vpa",
			Namespace: "default",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, existingVPA).Build()
	reconciler := &controller.VPAControllerReconciler{Client: fakeClient, Scheme: scheme}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{
		NamespacedName: client.ObjectKey{Namespace: "default", Name: "existing-deploy"},
	})
	assert.NoError(t, err)

	var vpaList autoscalingv1.VerticalPodAutoscalerList
	err = fakeClient.List(context.TODO(), &vpaList)
	assert.NoError(t, err)
	assert.Len(t, vpaList.Items, 1, "Should not create duplicate VPA")
}

func TestReconcile_OrphanedVPAIsDeleted(t *testing.T) {
	scheme := setupScheme(t)

	// Orphaned VPA (no owner reference)
	orphaned := &autoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orphaned-vpa",
			Namespace: "default",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orphaned).Build()
	reconciler := &controller.VPAControllerReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{})
	assert.NoError(t, err)

	err = fakeClient.Get(context.TODO(), client.ObjectKey{
		Namespace: "default",
		Name:      "orphaned-vpa",
	}, orphaned)
	assert.True(t, errors.IsNotFound(err), "Orphaned VPA should have been deleted")
}
