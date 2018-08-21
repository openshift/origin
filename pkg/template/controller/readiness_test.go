package controller

import (
	"testing"
	"time"

	kappsv1 "k8s.io/api/apps/v1"
	kappsv1beta1 "k8s.io/api/apps/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kextensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	routev1 "github.com/openshift/api/route/v1"
	fakebuild "github.com/openshift/client-go/build/clientset/versioned/fake"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func TestCheckReadiness(t *testing.T) {
	one := int32(1)
	zero := int64(0)

	tests := []struct {
		groupVersionKind schema.GroupVersionKind
		object           runtime.Object
		build            buildv1.Build
		expectedReady    bool
		expectedFailed   bool
	}{
		// Build
		{
			groupVersionKind: groupVersionKind(buildv1.GroupVersion, "Build"),
			object: &buildv1.Build{
				Status: buildv1.BuildStatus{
					Phase: buildv1.BuildPhaseNew,
				},
			},
		},
		{
			groupVersionKind: groupVersionKind(buildv1.GroupVersion, "Build"),
			object: &buildv1.Build{
				Status: buildv1.BuildStatus{
					Phase: buildv1.BuildPhaseComplete,
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(buildv1.GroupVersion, "Build"),
			object: &buildv1.Build{
				Status: buildv1.BuildStatus{
					Phase: buildv1.BuildPhaseError,
				},
			},
			expectedFailed: true,
		},

		// BuildConfig
		{
			groupVersionKind: groupVersionKind(buildv1.GroupVersion, "BuildConfig"),
			object:           &buildv1.BuildConfig{},
		},
		{
			groupVersionKind: groupVersionKind(buildv1.GroupVersion, "BuildConfig"),
			object: &buildv1.BuildConfig{
				Status: buildv1.BuildConfigStatus{
					LastVersion: 1,
				},
			},
			build: buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						buildutil.BuildConfigLabel: "",
					},
					Annotations: map[string]string{
						buildutil.BuildNumberAnnotation: "1",
					},
				},
				Status: buildv1.BuildStatus{
					Phase: buildv1.BuildPhaseComplete,
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(buildv1.GroupVersion, "BuildConfig"),
			object: &buildv1.BuildConfig{
				Status: buildv1.BuildConfigStatus{
					LastVersion: 1,
				},
			},
			build: buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						buildutil.BuildConfigLabel: "",
					},
					Annotations: map[string]string{
						buildutil.BuildNumberAnnotation: "1",
					},
				},
				Status: buildv1.BuildStatus{
					Phase: buildv1.BuildPhaseError,
				},
			},
			expectedFailed: true,
		},

		// Deployment
		{
			groupVersionKind: groupVersionKind(kappsv1.SchemeGroupVersion, "Deployment"),
			object:           &kappsv1.Deployment{},
		},
		{
			groupVersionKind: groupVersionKind(kappsv1.SchemeGroupVersion, "Deployment"),
			object: &kappsv1.Deployment{
				Status: kappsv1.DeploymentStatus{
					Conditions: []kappsv1.DeploymentCondition{
						{
							Type:   kappsv1.DeploymentProgressing,
							Status: corev1.ConditionTrue,
							Reason: deploymentutil.NewRSAvailableReason,
						},
						{
							Type:   kappsv1.DeploymentAvailable,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(kappsv1.SchemeGroupVersion, "Deployment"),
			object: &kappsv1.Deployment{
				Status: kappsv1.DeploymentStatus{
					Conditions: []kappsv1.DeploymentCondition{
						{
							Type:   kappsv1.DeploymentProgressing,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expectedFailed: true,
		},
		{
			groupVersionKind: groupVersionKind(kextensionsv1beta1.SchemeGroupVersion, "Deployment"),
			object:           &kextensionsv1beta1.Deployment{},
		},
		{
			groupVersionKind: groupVersionKind(kextensionsv1beta1.SchemeGroupVersion, "Deployment"),
			object: &kextensionsv1beta1.Deployment{
				Status: kextensionsv1beta1.DeploymentStatus{
					Conditions: []kextensionsv1beta1.DeploymentCondition{
						{
							Type:   kextensionsv1beta1.DeploymentProgressing,
							Status: corev1.ConditionTrue,
							Reason: deploymentutil.NewRSAvailableReason,
						},
						{
							Type:   kextensionsv1beta1.DeploymentAvailable,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(kextensionsv1beta1.SchemeGroupVersion, "Deployment"),
			object: &kextensionsv1beta1.Deployment{
				Status: kextensionsv1beta1.DeploymentStatus{
					Conditions: []kextensionsv1beta1.DeploymentCondition{
						{
							Type:   kextensionsv1beta1.DeploymentProgressing,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expectedFailed: true,
		},

		// DeploymentConfig
		{
			groupVersionKind: groupVersionKind(appsv1.GroupVersion, "DeploymentConfig"),
			object:           &appsv1.DeploymentConfig{},
		},
		{
			groupVersionKind: groupVersionKind(appsv1.GroupVersion, "DeploymentConfig"),
			object: &appsv1.DeploymentConfig{
				Status: appsv1.DeploymentConfigStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:   appsv1.DeploymentProgressing,
							Status: corev1.ConditionTrue,
							Reason: appsutil.NewRcAvailableReason,
						},
						{
							Type:   appsv1.DeploymentAvailable,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(appsv1.GroupVersion, "DeploymentConfig"),
			object: &appsv1.DeploymentConfig{
				Status: appsv1.DeploymentConfigStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:   appsv1.DeploymentProgressing,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expectedFailed: true,
		},

		// Job
		{
			groupVersionKind: groupVersionKind(batchv1.SchemeGroupVersion, "Job"),
			object:           &batchv1.Job{},
		},
		{
			groupVersionKind: groupVersionKind(batchv1.SchemeGroupVersion, "Job"),
			object: &batchv1.Job{
				Status: batchv1.JobStatus{
					CompletionTime: &metav1.Time{Time: time.Unix(0, 0)},
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(batchv1.SchemeGroupVersion, "Job"),
			object: &batchv1.Job{
				Status: batchv1.JobStatus{
					Failed: 1,
				},
			},
			expectedFailed: true,
		},

		// StatefulSet
		{
			groupVersionKind: groupVersionKind(kappsv1.SchemeGroupVersion, "StatefulSet"),
			object: &kappsv1.StatefulSet{
				Spec: kappsv1.StatefulSetSpec{
					Replicas: &one,
				},
			},
		},
		{
			groupVersionKind: groupVersionKind(kappsv1.SchemeGroupVersion, "StatefulSet"),
			object: &kappsv1.StatefulSet{
				Spec: kappsv1.StatefulSetSpec{
					Replicas: &one,
				},
				Status: kappsv1.StatefulSetStatus{
					ObservedGeneration: 0,
					ReadyReplicas:      1,
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(kappsv1beta1.SchemeGroupVersion, "StatefulSet"),
			object: &kappsv1beta1.StatefulSet{
				Spec: kappsv1beta1.StatefulSetSpec{
					Replicas: &one,
				},
			},
		},
		{
			groupVersionKind: groupVersionKind(kappsv1beta1.SchemeGroupVersion, "StatefulSet"),
			object: &kappsv1beta1.StatefulSet{
				Spec: kappsv1beta1.StatefulSetSpec{
					Replicas: &one,
				},
				Status: kappsv1beta1.StatefulSetStatus{
					ObservedGeneration: &zero,
					ReadyReplicas:      1,
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(routev1.GroupVersion, "Route"),
			object: &routev1.Route{
				Spec: routev1.RouteSpec{
					Host: "",
				},
			},
			expectedReady: false,
		},
		{
			groupVersionKind: groupVersionKind(routev1.GroupVersion, "Route"),
			object: &routev1.Route{
				Spec: routev1.RouteSpec{
					Host: "app.example.com",
				},
			},
			expectedReady: true,
		},
		{
			groupVersionKind: groupVersionKind(routev1.GroupVersion, "Route"),
			object: &routev1.Route{
				Spec: routev1.RouteSpec{
					Host: "",
				},
			},
			expectedReady: false,
		},
		{
			groupVersionKind: groupVersionKind(routev1.GroupVersion, "Route"),
			object: &routev1.Route{
				Spec: routev1.RouteSpec{
					Host: "app.example.com",
				},
			},
			expectedReady: true,
		},
	}

	for i, test := range tests {
		buildClient := fakebuild.NewSimpleClientset(&test.build)
		ref := corev1.ObjectReference{
			Kind:       test.groupVersionKind.Kind,
			APIVersion: test.groupVersionKind.GroupVersion().String(),
		}
		if can := CanCheckReadiness(ref); !can {
			t.Errorf("%d: unexpected canCheckReadiness value %v", i, can)
			continue
		}
		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.object)
		if err != nil {
			t.Fatal(err)
		}
		ready, failed, err := CheckReadiness(buildClient, ref, &unstructured.Unstructured{Object: unstructuredObj})
		if err != nil {
			t.Errorf("%d: unexpected err value: %v", i, err)
			continue
		}
		if ready != test.expectedReady {
			t.Errorf("%d[%s]: unexpected ready value: %v", i, test.groupVersionKind.String(), ready)
		}
		if failed != test.expectedFailed {
			t.Errorf("%d[%s]: unexpected failed value: %v", i, test.groupVersionKind.String(), failed)
		}
	}
}
