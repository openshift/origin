package v1

import (
	"reflect"
	"testing"

	kapiv1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/api/apps/v1"
	newer "github.com/openshift/origin/pkg/apps/apis/apps"
)

var scheme = runtime.NewScheme()
var codecs = serializer.NewCodecFactory(scheme)

func init() {
	kapi.AddToScheme(scheme)
	kapiv1.AddToScheme(scheme)
	LegacySchemeBuilder.AddToScheme(scheme)
	newer.LegacySchemeBuilder.AddToScheme(scheme)
	SchemeBuilder.AddToScheme(scheme)
	newer.SchemeBuilder.AddToScheme(scheme)
}

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
		p := v1.DeploymentTriggerImageChangeParams{
			From: kapiv1.ObjectReference{
				Kind: test.kind,
				Name: test.name,
			},
		}
		out := &newer.DeploymentTriggerImageChangeParams{}
		if err := scheme.Convert(&p, out, nil); err != nil {
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

func Test_convert_v1_RollingDeploymentStrategyParams_To_apps_RollingDeploymentStrategyParams(t *testing.T) {
	tests := []struct {
		in  *v1.RollingDeploymentStrategyParams
		out *newer.RollingDeploymentStrategyParams
	}{
		{
			in: &v1.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxUnavailable:      newIntOrString(intstr.FromString("25%")),
				Pre: &v1.LifecycleHook{
					FailurePolicy: v1.LifecycleHookFailurePolicyIgnore,
				},
				Post: &v1.LifecycleHook{
					FailurePolicy: v1.LifecycleHookFailurePolicyAbort,
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
			in: &v1.RollingDeploymentStrategyParams{
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
			in: &v1.RollingDeploymentStrategyParams{
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
			in: &v1.RollingDeploymentStrategyParams{
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
		if err := scheme.Convert(test.in, out, nil); err != nil {
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
		out *v1.RollingDeploymentStrategyParams
	}{
		{
			in: &newer.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            intstr.FromInt(0),
				MaxUnavailable:      intstr.FromString("25%"),
			},
			out: &v1.RollingDeploymentStrategyParams{
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
			out: &v1.RollingDeploymentStrategyParams{
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
			out: &v1.RollingDeploymentStrategyParams{
				UpdatePeriodSeconds: newInt64(5),
				IntervalSeconds:     newInt64(6),
				TimeoutSeconds:      newInt64(7),
				MaxSurge:            newIntOrString(intstr.FromInt(10)),
				MaxUnavailable:      newIntOrString(intstr.FromInt(20)),
			},
		},
	}

	for _, test := range tests {
		out := &v1.RollingDeploymentStrategyParams{}
		if err := scheme.Convert(test.in, out, nil); err != nil {
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
