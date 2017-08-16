package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
)

func TestCheckReadiness(t *testing.T) {
	zero := int64(0)

	tests := []struct {
		groupKind      schema.GroupKind
		object         runtime.Object
		build          buildapi.Build
		expectedReady  bool
		expectedFailed bool
	}{
		// Build
		{
			groupKind: buildapi.Kind("Build"),
			object: &buildapi.Build{
				Status: buildapi.BuildStatus{
					Phase: buildapi.BuildPhaseNew,
				},
			},
		},
		{
			groupKind: buildapi.Kind("Build"),
			object: &buildapi.Build{
				Status: buildapi.BuildStatus{
					Phase: buildapi.BuildPhaseComplete,
				},
			},
			expectedReady: true,
		},
		{
			groupKind: buildapi.Kind("Build"),
			object: &buildapi.Build{
				Status: buildapi.BuildStatus{
					Phase: buildapi.BuildPhaseError,
				},
			},
			expectedFailed: true,
		},

		// BuildConfig
		{
			groupKind: buildapi.Kind("BuildConfig"),
			object:    &buildapi.BuildConfig{},
		},
		{
			groupKind: buildapi.Kind("BuildConfig"),
			object: &buildapi.BuildConfig{
				Status: buildapi.BuildConfigStatus{
					LastVersion: 1,
				},
			},
			build: buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						buildapi.BuildNumberAnnotation: "1",
					},
				},
				Status: buildapi.BuildStatus{
					Phase: buildapi.BuildPhaseComplete,
				},
			},
			expectedReady: true,
		},
		{
			groupKind: buildapi.Kind("BuildConfig"),
			object: &buildapi.BuildConfig{
				Status: buildapi.BuildConfigStatus{
					LastVersion: 1,
				},
			},
			build: buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						buildapi.BuildNumberAnnotation: "1",
					},
				},
				Status: buildapi.BuildStatus{
					Phase: buildapi.BuildPhaseError,
				},
			},
			expectedFailed: true,
		},

		// Deployment
		{
			groupKind: apps.Kind("Deployment"),
			object:    &extensions.Deployment{},
		},
		{
			groupKind: apps.Kind("Deployment"),
			object: &extensions.Deployment{
				Status: extensions.DeploymentStatus{
					Conditions: []extensions.DeploymentCondition{
						{
							Type:   extensions.DeploymentProgressing,
							Status: kapi.ConditionTrue,
							Reason: deploymentutil.NewRSAvailableReason,
						},
						{
							Type:   extensions.DeploymentAvailable,
							Status: kapi.ConditionTrue,
						},
					},
				},
			},
			expectedReady: true,
		},
		{
			groupKind: apps.Kind("Deployment"),
			object: &extensions.Deployment{
				Status: extensions.DeploymentStatus{
					Conditions: []extensions.DeploymentCondition{
						{
							Type:   extensions.DeploymentProgressing,
							Status: kapi.ConditionFalse,
						},
					},
				},
			},
			expectedFailed: true,
		},

		// DeploymentConfig
		{
			groupKind: deployapi.Kind("DeploymentConfig"),
			object:    &deployapi.DeploymentConfig{},
		},
		{
			groupKind: deployapi.Kind("DeploymentConfig"),
			object: &deployapi.DeploymentConfig{
				Status: deployapi.DeploymentConfigStatus{
					Conditions: []deployapi.DeploymentCondition{
						{
							Type:   deployapi.DeploymentProgressing,
							Status: kapi.ConditionTrue,
							Reason: deployapi.NewRcAvailableReason,
						},
						{
							Type:   deployapi.DeploymentAvailable,
							Status: kapi.ConditionTrue,
						},
					},
				},
			},
			expectedReady: true,
		},
		{
			groupKind: deployapi.Kind("DeploymentConfig"),
			object: &deployapi.DeploymentConfig{
				Status: deployapi.DeploymentConfigStatus{
					Conditions: []deployapi.DeploymentCondition{
						{
							Type:   deployapi.DeploymentProgressing,
							Status: kapi.ConditionFalse,
						},
					},
				},
			},
			expectedFailed: true,
		},

		// Job
		{
			groupKind: batch.Kind("Job"),
			object:    &batch.Job{},
		},
		{
			groupKind: batch.Kind("Job"),
			object: &batch.Job{
				Status: batch.JobStatus{
					CompletionTime: &metav1.Time{Time: time.Now()},
				},
			},
			expectedReady: true,
		},
		{
			groupKind: batch.Kind("Job"),
			object: &batch.Job{
				Status: batch.JobStatus{
					Failed: 1,
				},
			},
			expectedFailed: true,
		},

		// StatefulSet
		{
			groupKind: apps.Kind("StatefulSet"),
			object: &apps.StatefulSet{
				Spec: apps.StatefulSetSpec{
					Replicas: 1,
				},
			},
		},
		{
			groupKind: apps.Kind("StatefulSet"),
			object: &apps.StatefulSet{
				Spec: apps.StatefulSetSpec{
					Replicas: 1,
				},
				Status: apps.StatefulSetStatus{
					ObservedGeneration: &zero,
					ReadyReplicas:      1,
				},
			},
			expectedReady: true,
		},
	}

	for i, test := range tests {
		cli := testclient.NewSimpleFake(&buildapi.BuildList{Items: []buildapi.Build{test.build}})
		ref := kapi.ObjectReference{
			Kind:       test.groupKind.Kind,
			APIVersion: test.groupKind.WithVersion("v1").GroupVersion().String(),
		}
		if can := canCheckReadiness(ref); !can {
			t.Errorf("%d: unexpected canCheckReadiness value %v", i, can)
			continue
		}
		ready, failed, err := checkReadiness(cli, ref, test.object)
		if err != nil {
			t.Errorf("%d: unexpected err value %v", i, err)
			continue
		}
		if ready != test.expectedReady {
			t.Errorf("%d: unexpected ready value %v", i, ready)
		}
		if failed != test.expectedFailed {
			t.Errorf("%d: unexpected failed value %v", i, failed)
		}
	}
}
