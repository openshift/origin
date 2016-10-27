package v1

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/intstr"

	newer "github.com/openshift/origin/pkg/deploy/api"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestTriggerRoundTrip(t *testing.T) {
	tests := []struct {
		testName   string
		kind, name string
	}{
		{
			testName: "ImageStream -> ImageStreamTag",
			kind:     "ImageStream",
			name:     "golang",
		},
		{
			testName: "ImageStreamTag -> ImageStreamTag",
			kind:     "ImageStreamTag",
			name:     "golang:latest",
		},
		{
			testName: "ImageRepository -> ImageStreamTag",
			kind:     "ImageRepository",
			name:     "golang",
		},
	}

	for _, test := range tests {
		p := DeploymentTriggerImageChangeParams{
			From: kapiv1.ObjectReference{
				Kind: test.kind,
				Name: test.name,
			},
		}
		out := &newer.DeploymentTriggerImageChangeParams{}
		if err := kapi.Scheme.Convert(&p, out, nil); err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
		if out.From.Name != "golang:latest" {
			t.Errorf("%s: unexpected name: %s", test.testName, out.From.Name)
		}
		if out.From.Kind != "ImageStreamTag" {
			t.Errorf("%s: unexpected kind: %s", test.testName, out.From.Kind)
		}
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
				MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
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
				MaxSurge:            intstr.FromInt(0),
				MaxUnavailable:      intstr.FromString("25%"),
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
				MaxSurge:            newIntOrString(intstr.FromString("25%")),
			},
			out: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            intstr.FromString("25%"),
				MaxUnavailable:      intstr.FromInt(0),
			},
		},
		{
			in: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            newIntOrString(intstr.FromInt(10)),
				MaxUnavailable:      newIntOrString(intstr.FromInt(20)),
			},
			out: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            intstr.FromInt(10),
				MaxUnavailable:      intstr.FromInt(20),
			},
		},
		{
			in: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
			},
			out: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            intstr.FromString("25%"),
				MaxUnavailable:      intstr.FromString("25%"),
			},
		},
	}

	for i, test := range tests {
		t.Logf("running test case #%d", i)
		out := &newer.RollingDeploymentStrategyParams{}
		if err := kapi.Scheme.Convert(test.in, out, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(out, test.out) {
			t.Errorf("got different than expected:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s", out, test.out, diff.ObjectDiff(test.out, out), diff.ObjectGoPrintSideBySide(test.out, out))
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
				MaxSurge:            intstr.FromInt(0),
				MaxUnavailable:      intstr.FromString("25%"),
			},
			out: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            newIntOrString(intstr.FromInt(0)),
				MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
			},
		},
		{
			in: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            intstr.FromString("25%"),
				MaxUnavailable:      intstr.FromInt(0),
			},
			out: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            newIntOrString(intstr.FromString("25%")),
				MaxUnavailable:      newIntOrString(intstr.FromInt(0)),
			},
		},
		{
			in: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            intstr.FromInt(10),
				MaxUnavailable:      intstr.FromInt(20),
			},
			out: &RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            newIntOrString(intstr.FromInt(10)),
				MaxUnavailable:      newIntOrString(intstr.FromInt(20)),
			},
		},
	}

	for _, test := range tests {
		out := &RollingDeploymentStrategyParams{}
		if err := kapi.Scheme.Convert(test.in, out, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(out, test.out) {
			t.Errorf("got different than expected:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s", out, test.out, diff.ObjectDiff(test.out, out), diff.ObjectGoPrintSideBySide(test.out, out))
		}
	}
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

func TestFieldSelectors(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "DeploymentConfig",
		// Ensure all currently returned labels are supported
		newer.DeploymentConfigToSelectableFields(&newer.DeploymentConfig{}),
	)
}
