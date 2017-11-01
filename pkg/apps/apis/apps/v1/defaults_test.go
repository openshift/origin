package v1

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func TestDefaults(t *testing.T) {
	defaultIntOrString := intstr.FromString("25%")
	differentIntOrString := intstr.FromInt(5)
	tests := []struct {
		original *DeploymentConfig
		expected *DeploymentConfig
	}{
		{
			original: &DeploymentConfig{},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(deployapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(deployapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(deployapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{
							Type: DeploymentTriggerOnConfigChange,
						},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRecreate,
						RecreateParams: &RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
							Pre: &LifecycleHook{
								TagImages: []TagImageHook{{}, {}},
							},
							Mid: &LifecycleHook{
								TagImages: []TagImageHook{{}, {}},
							},
							Post: &LifecycleHook{
								TagImages: []TagImageHook{{}, {}},
							},
						},
						RollingParams: &RollingDeploymentStrategyParams{
							Pre: &LifecycleHook{
								TagImages: []TagImageHook{{}, {}},
							},
							Post: &LifecycleHook{
								TagImages: []TagImageHook{{}, {}},
							},
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            &differentIntOrString,
							MaxUnavailable:      &differentIntOrString,
						},
					},
					Triggers: []DeploymentTriggerPolicy{
						{
							Type: DeploymentTriggerOnImageChange,
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
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRecreate,
						RecreateParams: &RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
							Pre: &LifecycleHook{
								TagImages: []TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Mid: &LifecycleHook{
								TagImages: []TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Post: &LifecycleHook{
								TagImages: []TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
						},
						RollingParams: &RollingDeploymentStrategyParams{
							Pre: &LifecycleHook{
								TagImages: []TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Post: &LifecycleHook{
								TagImages: []TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            &differentIntOrString,
							MaxUnavailable:      &differentIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{
							Type: DeploymentTriggerOnImageChange,
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
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromString("50%")),
						},
						ActiveDeadlineSeconds: newInt64(3600),
					},
					Triggers: []DeploymentTriggerPolicy{
						{
							Type: DeploymentTriggerOnImageChange,
						},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromString("50%")),
							MaxUnavailable:      newIntOrString(intstr.FromInt(0)),
						},
						ActiveDeadlineSeconds: newInt64(3600),
					},
					Triggers: []DeploymentTriggerPolicy{
						{
							Type: DeploymentTriggerOnImageChange,
						},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
					},
					Triggers: []DeploymentTriggerPolicy{
						{
							Type: DeploymentTriggerOnImageChange,
						},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{
							Type: DeploymentTriggerOnImageChange,
						},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
						},
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type:          DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{},
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							IntervalSeconds:     newInt64(deployapi.DefaultRollingIntervalSeconds),
							UpdatePeriodSeconds: newInt64(deployapi.DefaultRollingUpdatePeriodSeconds),
							TimeoutSeconds:      newInt64(deployapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            newIntOrString(intstr.FromString("25%")),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRecreate,
						// test non-nil RecreateParams is filled in
						RecreateParams: &RecreateDeploymentStrategyParams{},
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRecreate,
						RecreateParams: &RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRecreate,
						// test nil RecreateParams
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRecreate,
						RecreateParams: &RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{Name: "first"},
							},
						},
					},
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRecreate,
						RecreateParams: &RecreateDeploymentStrategyParams{
							Pre: &LifecycleHook{
								TagImages:  []TagImageHook{{}},
								ExecNewPod: &ExecNewPodHook{},
							},
							Mid: &LifecycleHook{
								TagImages:  []TagImageHook{{}},
								ExecNewPod: &ExecNewPodHook{},
							},
							Post: &LifecycleHook{
								TagImages:  []TagImageHook{{}},
								ExecNewPod: &ExecNewPodHook{},
							},
						},
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
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
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRecreate,
						RecreateParams: &RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
							Pre: &LifecycleHook{
								TagImages:  []TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &ExecNewPodHook{ContainerName: "first"},
							},
							Mid: &LifecycleHook{
								TagImages:  []TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &ExecNewPodHook{ContainerName: "first"},
							},
							Post: &LifecycleHook{
								TagImages:  []TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &ExecNewPodHook{ContainerName: "first"},
							},
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{Name: "first"},
							},
						},
					},
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							Pre: &LifecycleHook{
								TagImages:  []TagImageHook{{}},
								ExecNewPod: &ExecNewPodHook{},
							},
							Post: &LifecycleHook{
								TagImages:  []TagImageHook{{}},
								ExecNewPod: &ExecNewPodHook{},
							},
						},
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
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
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(deployapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(deployapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(deployapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
							Pre: &LifecycleHook{
								TagImages:  []TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &ExecNewPodHook{ContainerName: "first"},
							},
							Post: &LifecycleHook{
								TagImages:  []TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &ExecNewPodHook{ContainerName: "first"},
							},
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Triggers: []DeploymentTriggerPolicy{
						{
							Type:              DeploymentTriggerOnImageChange,
							ImageChangeParams: &DeploymentTriggerImageChangeParams{},
						},
					},
				},
			},
			expected: &DeploymentConfig{
				Spec: DeploymentConfigSpec{
					Strategy: DeploymentStrategy{
						Type: DeploymentStrategyTypeRolling,
						RollingParams: &RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(deployapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(deployapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(deployapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []DeploymentTriggerPolicy{
						{
							Type: DeploymentTriggerOnImageChange,
							ImageChangeParams: &DeploymentTriggerImageChangeParams{
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
			got, ok := obj2.(*DeploymentConfig)
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
