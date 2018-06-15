package v1

import (
	"fmt"
	"reflect"
	"testing"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func TestDefaults(t *testing.T) {
	defaultIntOrString := intstr.FromString("25%")
	differentIntOrString := intstr.FromInt(5)
	tests := []struct {
		original *v1.DeploymentConfig
		expected *v1.DeploymentConfig
	}{
		{
			original: &v1.DeploymentConfig{},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(appsapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(appsapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(appsapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type: v1.DeploymentTriggerOnConfigChange,
						},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRecreate,
						RecreateParams: &v1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(appsapi.DefaultRollingTimeoutSeconds),
							Pre: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{}, {}},
							},
							Mid: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{}, {}},
							},
							Post: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{}, {}},
							},
						},
						RollingParams: &v1.RollingDeploymentStrategyParams{
							Pre: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{}, {}},
							},
							Post: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{}, {}},
							},
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            &differentIntOrString,
							MaxUnavailable:      &differentIntOrString,
						},
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type: v1.DeploymentTriggerOnImageChange,
						},
					},
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{
									Name: "test",
								},
							},
						},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRecreate,
						RecreateParams: &v1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(appsapi.DefaultRollingTimeoutSeconds),
							Pre: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Mid: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Post: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
						},
						RollingParams: &v1.RollingDeploymentStrategyParams{
							Pre: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Post: &v1.LifecycleHook{
								TagImages: []v1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            &differentIntOrString,
							MaxUnavailable:      &differentIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type: v1.DeploymentTriggerOnImageChange,
						},
					},
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							SecurityContext:               &kapiv1.PodSecurityContext{},
							RestartPolicy:                 kapiv1.RestartPolicyAlways,
							TerminationGracePeriodSeconds: mkintp(30),
							DNSPolicy:                     kapiv1.DNSClusterFirst,
							Containers: []kapiv1.Container{
								{
									Name: "test",
									TerminationMessagePath:   "/dev/termination-log",
									TerminationMessagePolicy: kapiv1.TerminationMessageReadFile,
									// The pull policy will be "PullAlways" only when the
									// image tag is 'latest'. In other case it will be
									// "PullIfNotPresent".
									ImagePullPolicy: kapiv1.PullIfNotPresent,
								},
							},
							SchedulerName: kapiv1.DefaultSchedulerName,
						},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromString("50%")),
						},
						ActiveDeadlineSeconds: newInt64(3600),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type: v1.DeploymentTriggerOnImageChange,
						},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromString("50%")),
							MaxUnavailable:      newIntOrString(intstr.FromInt(0)),
						},
						ActiveDeadlineSeconds: newInt64(3600),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type: v1.DeploymentTriggerOnImageChange,
						},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type: v1.DeploymentTriggerOnImageChange,
						},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type: v1.DeploymentTriggerOnImageChange,
						},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
						},
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type:          v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{},
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							IntervalSeconds:     newInt64(appsapi.DefaultRollingIntervalSeconds),
							UpdatePeriodSeconds: newInt64(appsapi.DefaultRollingUpdatePeriodSeconds),
							TimeoutSeconds:      newInt64(appsapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            newIntOrString(intstr.FromString("25%")),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRecreate,
						// test non-nil RecreateParams is filled in
						RecreateParams: &v1.RecreateDeploymentStrategyParams{},
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRecreate,
						RecreateParams: &v1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(appsapi.DefaultRollingTimeoutSeconds),
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRecreate,
						// test nil RecreateParams
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRecreate,
						RecreateParams: &v1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(appsapi.DefaultRollingTimeoutSeconds),
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{Name: "first"},
							},
						},
					},
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRecreate,
						RecreateParams: &v1.RecreateDeploymentStrategyParams{
							Pre: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{}},
								ExecNewPod: &v1.ExecNewPodHook{},
							},
							Mid: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{}},
								ExecNewPod: &v1.ExecNewPodHook{},
							},
							Post: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{}},
								ExecNewPod: &v1.ExecNewPodHook{},
							},
						},
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{
									Name: "first",
									TerminationMessagePath:   "/dev/termination-log",
									TerminationMessagePolicy: kapiv1.TerminationMessageReadFile,
									ImagePullPolicy:          kapiv1.PullIfNotPresent,
								},
							},
							RestartPolicy:                 kapiv1.RestartPolicyAlways,
							TerminationGracePeriodSeconds: mkintp(30),
							SecurityContext:               &kapiv1.PodSecurityContext{},
							DNSPolicy:                     kapiv1.DNSClusterFirst,
							SchedulerName:                 kapiv1.DefaultSchedulerName,
						},
					},
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRecreate,
						RecreateParams: &v1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(appsapi.DefaultRollingTimeoutSeconds),
							Pre: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &v1.ExecNewPodHook{ContainerName: "first"},
							},
							Mid: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &v1.ExecNewPodHook{ContainerName: "first"},
							},
							Post: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &v1.ExecNewPodHook{ContainerName: "first"},
							},
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{Name: "first"},
							},
						},
					},
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							Pre: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{}},
								ExecNewPod: &v1.ExecNewPodHook{},
							},
							Post: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{}},
								ExecNewPod: &v1.ExecNewPodHook{},
							},
						},
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{
									Name: "first",
									TerminationMessagePath:   "/dev/termination-log",
									TerminationMessagePolicy: kapiv1.TerminationMessageReadFile,
									ImagePullPolicy:          kapiv1.PullIfNotPresent,
								},
							},
							RestartPolicy:                 kapiv1.RestartPolicyAlways,
							TerminationGracePeriodSeconds: mkintp(30),
							SecurityContext:               &kapiv1.PodSecurityContext{},
							DNSPolicy:                     kapiv1.DNSClusterFirst,
							SchedulerName:                 kapiv1.DefaultSchedulerName,
						},
					},
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(appsapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(appsapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(appsapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
							Pre: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &v1.ExecNewPodHook{ContainerName: "first"},
							},
							Post: &v1.LifecycleHook{
								TagImages:  []v1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &v1.ExecNewPodHook{ContainerName: "first"},
							},
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type:              v1.DeploymentTriggerOnImageChange,
							ImageChangeParams: &v1.DeploymentTriggerImageChangeParams{},
						},
					},
				},
			},
			expected: &v1.DeploymentConfig{
				Spec: v1.DeploymentConfigSpec{
					Strategy: v1.DeploymentStrategy{
						Type: v1.DeploymentStrategyTypeRolling,
						RollingParams: &v1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(appsapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(appsapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(appsapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(appsapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []v1.DeploymentTriggerPolicy{
						{
							Type: v1.DeploymentTriggerOnImageChange,
							ImageChangeParams: &v1.DeploymentTriggerImageChangeParams{
								From: kapiv1.ObjectReference{
									Kind: "ImageStreamTag",
								},
							},
						},
					},
				},
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Logf("test %d", i)
			original := test.original
			expected := test.expected
			obj2 := roundTrip(t, runtime.Object(original))
			got, ok := obj2.(*v1.DeploymentConfig)
			if !ok {
				t.Fatalf("unexpected object: %v", got)
			}
			// TODO(rebase): check that there are no fields which have different semantics for nil and []
			if !equality.Semantic.DeepEqual(got.Spec, expected.Spec) {
				t.Errorf("got different than expected:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s", got, expected, diff.ObjectDiff(expected, got), diff.ObjectGoPrintSideBySide(expected, got))
			}
		})
	}
}

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := runtime.Encode(codecs.LegacyCodec(LegacySchemeGroupVersion), obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := runtime.Decode(codecs.UniversalDecoder(), data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = scheme.Convert(obj2, obj3, nil)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}
