package v1_test

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/v1"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestDefaults(t *testing.T) {
	testCases := []struct {
		External runtime.Object
		Internal runtime.Object
		Ok       func(runtime.Object) bool
	}{
		{
			External: &v1.Build{
				Spec: v1.BuildSpec{
					CommonSpec: v1.CommonSpec{
						Strategy: v1.BuildStrategy{
							Type: v1.DockerBuildStrategyType,
						},
					},
				},
			},
			Internal: &api.Build{},
			Ok: func(out runtime.Object) bool {
				obj := out.(*api.Build)
				return obj.Spec.Strategy.DockerStrategy != nil
			},
		},
		{
			External: &v1.Build{
				Spec: v1.BuildSpec{
					CommonSpec: v1.CommonSpec{
						Strategy: v1.BuildStrategy{
							SourceStrategy: &v1.SourceBuildStrategy{},
						},
					},
				},
			},
			Internal: &api.Build{},
			Ok: func(out runtime.Object) bool {
				obj := out.(*api.Build)
				return obj.Spec.Strategy.SourceStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &v1.Build{
				Spec: v1.BuildSpec{
					CommonSpec: v1.CommonSpec{
						Strategy: v1.BuildStrategy{
							DockerStrategy: &v1.DockerBuildStrategy{
								From: &kapiv1.ObjectReference{},
							},
						},
					},
				},
			},
			Internal: &api.Build{},
			Ok: func(out runtime.Object) bool {
				obj := out.(*api.Build)
				return obj.Spec.Strategy.DockerStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &v1.Build{
				Spec: v1.BuildSpec{
					CommonSpec: v1.CommonSpec{
						Strategy: v1.BuildStrategy{
							CustomStrategy: &v1.CustomBuildStrategy{},
						},
					},
				},
			},
			Internal: &api.Build{},
			Ok: func(out runtime.Object) bool {
				obj := out.(*api.Build)
				return obj.Spec.Strategy.CustomStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &v1.BuildConfig{
				Spec: v1.BuildConfigSpec{Triggers: []v1.BuildTriggerPolicy{{Type: v1.ImageChangeBuildTriggerType}}},
			},
			Internal: &api.BuildConfig{},
			Ok: func(out runtime.Object) bool {
				obj := out.(*api.BuildConfig)
				// conversion drops this trigger because it has no type
				return (len(obj.Spec.Triggers) == 0) && (obj.Spec.RunPolicy == api.BuildRunPolicySerial)
			},
		},
	}

	for i, test := range testCases {
		if err := kapi.Scheme.Convert(test.External, test.Internal, nil); err != nil {
			t.Fatal(err)
		}
		if !test.Ok(test.Internal) {
			t.Errorf("%d: did not match: %#v", i, test.Internal)
		}
	}
}
