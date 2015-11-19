package v1

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/util"

	newer "github.com/openshift/origin/pkg/deploy/api"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestTriggerRoundTrip(t *testing.T) {
	p := DeploymentTriggerImageChangeParams{
		From: kapiv1.ObjectReference{
			Kind: "DockerImage",
			Name: "",
		},
	}
	out := &newer.DeploymentTriggerImageChangeParams{}
	if err := kapi.Scheme.Convert(&p, out); err == nil {
		t.Errorf("unexpected error: %v", err)
	}
	p.From.Name = "a/b:test"
	out = &newer.DeploymentTriggerImageChangeParams{}
	if err := kapi.Scheme.Convert(&p, out); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out.RepositoryName != "a/b" && out.Tag != "test" {
		t.Errorf("unexpected output: %#v", out)
	}
}

func Test_convert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams(t *testing.T) {
	tests := []struct {
		in  *RollingDeploymentStrategyParams
		out *newer.RollingDeploymentStrategyParams
	}{
		{
			in: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				UpdatePercent:       newInt(-25),
				Pre: &LifecycleHook{
					FailurePolicy: LifecycleHookFailurePolicyIgnore,
				},
				Post: &LifecycleHook{
					FailurePolicy: LifecycleHookFailurePolicyAbort,
				},
			},
			out: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				UpdatePercent:       newInt(-25),
				MaxSurge:            util.NewIntOrStringFromInt(0),
				MaxUnavailable:      util.NewIntOrStringFromString("25%"),
				Pre: &newer.LifecycleHook{
					FailurePolicy: newer.LifecycleHookFailurePolicyIgnore,
				},
				Post: &newer.LifecycleHook{
					FailurePolicy: newer.LifecycleHookFailurePolicyAbort,
				},
			},
		},
		{
			in: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				UpdatePercent:       newInt(25),
			},
			out: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				UpdatePercent:       newInt(25),
				MaxSurge:            util.NewIntOrStringFromString("25%"),
				MaxUnavailable:      util.NewIntOrStringFromInt(0),
			},
		},
		{
			in: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            newIntOrString(util.NewIntOrStringFromInt(10)),
				MaxUnavailable:      newIntOrString(util.NewIntOrStringFromInt(20)),
			},
			out: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            util.NewIntOrStringFromInt(10),
				MaxUnavailable:      util.NewIntOrStringFromInt(20),
			},
		},
	}

	for _, test := range tests {
		out := &newer.RollingDeploymentStrategyParams{}
		if err := kapi.Scheme.Convert(test.in, out); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(out, test.out) {
			t.Errorf("got different than expected:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s", out, test.out, util.ObjectDiff(test.out, out), util.ObjectGoPrintSideBySide(test.out, out))
		}
	}
}

func Test_convert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams(t *testing.T) {
	tests := []struct {
		in  *newer.RollingDeploymentStrategyParams
		out *RollingDeploymentStrategyParams
	}{
		{
			in: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				UpdatePercent:       newInt(-25),
				MaxSurge:            util.NewIntOrStringFromInt(0),
				MaxUnavailable:      util.NewIntOrStringFromString("25%"),
			},
			out: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				UpdatePercent:       newInt(-25),
				MaxSurge:            newIntOrString(util.NewIntOrStringFromInt(0)),
				MaxUnavailable:      newIntOrString(util.NewIntOrStringFromString("25%")),
			},
		},
		{
			in: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				UpdatePercent:       newInt(25),
				MaxSurge:            util.NewIntOrStringFromString("25%"),
				MaxUnavailable:      util.NewIntOrStringFromInt(0),
			},
			out: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				UpdatePercent:       newInt(25),
				MaxSurge:            newIntOrString(util.NewIntOrStringFromString("25%")),
				MaxUnavailable:      newIntOrString(util.NewIntOrStringFromInt(0)),
			},
		},
		{
			in: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            util.NewIntOrStringFromInt(10),
				MaxUnavailable:      util.NewIntOrStringFromInt(20),
			},
			out: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            newIntOrString(util.NewIntOrStringFromInt(10)),
				MaxUnavailable:      newIntOrString(util.NewIntOrStringFromInt(20)),
			},
		},
	}

	for _, test := range tests {
		out := &RollingDeploymentStrategyParams{}
		if err := kapi.Scheme.Convert(test.in, out); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(out, test.out) {
			t.Errorf("got different than expected:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s", out, test.out, util.ObjectDiff(test.out, out), util.ObjectGoPrintSideBySide(test.out, out))
		}
	}
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

func TestFieldSelectors(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "DeploymentConfig",
		// Ensure all currently returned labels are supported
		newer.DeploymentConfigToSelectableFields(&newer.DeploymentConfig{}),
	)
}
