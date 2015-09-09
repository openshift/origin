package v1beta3_test

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"

	v1 "github.com/openshift/origin/pkg/api/v1beta3"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1beta3"
)

func TestDefaults(t *testing.T) {
	defaultIntOrString := util.NewIntOrStringFromString("25%")
	differentIntOrString := util.NewIntOrStringFromInt(5)
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
					},
				},
			},
		},
		{
			original: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
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
				},
			},
			expected: &deployv1.DeploymentConfig{
				Spec: deployv1.DeploymentConfigSpec{
					Strategy: deployv1.DeploymentStrategy{
						Type: deployv1.DeploymentStrategyTypeRecreate,
						RollingParams: &deployv1.RollingDeploymentStrategyParams{
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
							UpdatePercent:       newInt(50),
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
							UpdatePercent:       newInt(50),
							MaxSurge:            newIntOrString(util.NewIntOrStringFromString("50%")),
							MaxUnavailable:      newIntOrString(util.NewIntOrStringFromInt(0)),
						},
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
							UpdatePercent:       newInt(-50),
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
							UpdatePercent:       newInt(-50),
							MaxSurge:            newIntOrString(util.NewIntOrStringFromInt(0)),
							MaxUnavailable:      newIntOrString(util.NewIntOrStringFromString("50%")),
						},
					},
					Triggers: []deployv1.DeploymentTriggerPolicy{
						{
							Type: deployv1.DeploymentTriggerOnImageChange,
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
		if !reflect.DeepEqual(got.Spec, expected.Spec) {
			t.Errorf("got different than expected:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s", got, expected, util.ObjectDiff(expected, got), util.ObjectGoPrintSideBySide(expected, got))

		}
	}
}

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := v1.Codec.Encode(obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := kapi.Codec.Decode(data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = kapi.Scheme.Convert(obj2, obj3)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}

func newInt64(val int64) *int64 {
	return &val
}

func newInt(val int) *int {
	return &val
}

func newIntOrString(ios util.IntOrString) *util.IntOrString {
	return &ios
}
