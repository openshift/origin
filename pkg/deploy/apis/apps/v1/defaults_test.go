package v1_test

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	// required to register defaulting functions for containers
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	_ "k8s.io/kubernetes/pkg/api/install"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"

	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	_ "github.com/openshift/origin/pkg/deploy/apis/apps/install"
	deployv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
)

func mkintp(i int64) *int64 {
	return &i
}

func TestDefaults(t *testing.T) {
	defaultIntOrString := intstr.FromString("25%")
	differentIntOrString := intstr.FromInt(5)
	tests := []struct {
		original *deployv1.DeploymentConfig
		expected *deployv1.DeploymentConfig
	}{
		{
			original: &deployv1.DeploymentConfig{},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(deployapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(deployapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(deployapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnConfigChange,
						},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						RecreateParams: &deployv1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
							Pre: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{}, {}},
							},
							Mid: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{}, {}},
							},
							Post: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{}, {}},
							},
						},
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							Pre: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{}, {}},
							},
							Post: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{}, {}},
							},
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            &differentIntOrString,
							MaxUnavailable:      &differentIntOrString,
						},
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnImageChange,
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
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						RecreateParams: &deployv1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
							Pre: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Mid: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Post: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
						},
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							Pre: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							Post: &deployv1.LifecycleHook{
								TagImages: []deployv1.TagImageHook{{ContainerName: "test"}, {ContainerName: "test"}},
							},
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            &differentIntOrString,
							MaxUnavailable:      &differentIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnImageChange,
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
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromString("50%")),
						},
						ActiveDeadlineSeconds: newInt64(3600),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnImageChange,
						},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromString("50%")),
							MaxUnavailable:      newIntOrString(intstr.FromInt(0)),
						},
						ActiveDeadlineSeconds: newInt64(3600),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnImageChange,
						},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnImageChange,
						},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnImageChange,
						},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
						},
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(5),
							IntervalSeconds:     newInt64(6),
							TimeoutSeconds:      newInt64(7),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
							MaxSurge:            newIntOrString(intstr.FromInt(0)),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type:          deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{},
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							IntervalSeconds:     newInt64(deployapi.DefaultRollingIntervalSeconds),
							UpdatePeriodSeconds: newInt64(deployapi.DefaultRollingUpdatePeriodSeconds),
							TimeoutSeconds:      newInt64(deployapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            newIntOrString(intstr.FromString("25%")),
							MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						// test non-nil RecreateParams is filled in
						RecreateParams: &deployv1.RecreateDeploymentStrategyParams{},
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						RecreateParams: &deployv1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						// test nil RecreateParams
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						RecreateParams: &deployv1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{Name: "first"},
							},
						},
					},
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						RecreateParams: &deployv1.RecreateDeploymentStrategyParams{
							Pre: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{}},
								ExecNewPod: &deployv1.ExecNewPodHook{},
							},
							Mid: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{}},
								ExecNewPod: &deployv1.ExecNewPodHook{},
							},
							Post: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{}},
								ExecNewPod: &deployv1.ExecNewPodHook{},
							},
						},
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
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
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						RecreateParams: &deployv1.RecreateDeploymentStrategyParams{
							TimeoutSeconds: newInt64(deployapi.DefaultRollingTimeoutSeconds),
							Pre: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &deployv1.ExecNewPodHook{ContainerName: "first"},
							},
							Mid: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &deployv1.ExecNewPodHook{ContainerName: "first"},
							},
							Post: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &deployv1.ExecNewPodHook{ContainerName: "first"},
							},
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Template: &kapiv1.PodTemplateSpec{
						Spec: kapiv1.PodSpec{
							Containers: []kapiv1.Container{
								{Name: "first"},
							},
						},
					},
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							Pre: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{}},
								ExecNewPod: &deployv1.ExecNewPodHook{},
							},
							Post: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{}},
								ExecNewPod: &deployv1.ExecNewPodHook{},
							},
						},
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
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
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(deployapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(deployapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(deployapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
							Pre: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &deployv1.ExecNewPodHook{ContainerName: "first"},
							},
							Post: &deployv1.LifecycleHook{
								TagImages:  []deployv1.TagImageHook{{ContainerName: "first"}},
								ExecNewPod: &deployv1.ExecNewPodHook{ContainerName: "first"},
							},
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{},
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type:              deployv1.DeploymentTriggerOnImageChange,
							ImageChangeParams: &deployv1.DeploymentTriggerImageChangeParams{},
						},
					},
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRolling,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
							UpdatePeriodSeconds: newInt64(deployapi.DefaultRollingUpdatePeriodSeconds),
							IntervalSeconds:     newInt64(deployapi.DefaultRollingIntervalSeconds),
							TimeoutSeconds:      newInt64(deployapi.DefaultRollingTimeoutSeconds),
							MaxSurge:            &defaultIntOrString,
							MaxUnavailable:      &defaultIntOrString,
						},
						ActiveDeadlineSeconds: newInt64(deployapi.MaxDeploymentDurationSeconds),
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnImageChange,
							ImageChangeParams: &deployv1.DeploymentTriggerImageChangeParams{
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
		t.Logf("test %d", i)
		original := test.original
		expected := test.expected
		obj2 := roundTrip(t, runtime.Object(original))
		got, ok := obj2.(*deployv1.DeploymentConfig)
		if !ok {
			t.Errorf("unexpected object: %v", got)
			t.FailNow()
		}
		// TODO(rebase): check that there are no fields which have different semantics for nil and []
		if !kapihelper.Semantic.DeepEqual(got.Spec, expected.Spec) {
			t.Errorf("got different than expected:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s", got, expected, diff.ObjectDiff(expected, got), diff.ObjectGoPrintSideBySide(expected, got))
		}
	}
}

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := runtime.Encode(kapi.Codecs.LegacyCodec(deployv1.LegacySchemeGroupVersion), obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = kapi.Scheme.Convert(obj2, obj3, nil)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}

func newInt64(val int64) *int64 {
	return &val
}

func newInt32(val int32) *int32 {
	return &val
}

func newIntOrString(ios intstr.IntOrString) *intstr.IntOrString {
	return &ios
}
