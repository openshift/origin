package v1_test

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"

	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestDefaults(t *testing.T) {
	testCases := []struct {
		External runtime.Object
		Ok       func(runtime.Object) bool
	}{
		{
			External: &buildapiv1.Build{
				Spec: buildapiv1.BuildSpec{
					CommonSpec: buildapiv1.CommonSpec{
						Strategy: buildapiv1.BuildStrategy{
							Type: buildapiv1.DockerBuildStrategyType,
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*buildapiv1.Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.DockerStrategy != nil
			},
		},
		{
			External: &buildapiv1.Build{
				Spec: buildapiv1.BuildSpec{
					CommonSpec: buildapiv1.CommonSpec{
						Strategy: buildapiv1.BuildStrategy{
							SourceStrategy: &buildapiv1.SourceBuildStrategy{},
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*buildapiv1.Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.SourceStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &buildapiv1.Build{
				Spec: buildapiv1.BuildSpec{
					CommonSpec: buildapiv1.CommonSpec{
						Strategy: buildapiv1.BuildStrategy{
							DockerStrategy: &buildapiv1.DockerBuildStrategy{
								From: &kapiv1.ObjectReference{},
							},
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*buildapiv1.Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.DockerStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &buildapiv1.Build{
				Spec: buildapiv1.BuildSpec{
					CommonSpec: buildapiv1.CommonSpec{
						Strategy: buildapiv1.BuildStrategy{
							CustomStrategy: &buildapiv1.CustomBuildStrategy{},
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*buildapiv1.Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.CustomStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &buildapiv1.BuildConfig{
				Spec: buildapiv1.BuildConfigSpec{Triggers: []buildapiv1.BuildTriggerPolicy{{Type: buildapiv1.ImageChangeBuildTriggerType}}},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*buildapiv1.BuildConfig)
				if !ok {
					return false
				}
				// conversion drops this trigger because it has no type
				return (len(obj.Spec.Triggers) == 0) && (obj.Spec.RunPolicy == buildapiv1.BuildRunPolicySerial)
			},
		},
		{
			External: &buildapiv1.BuildConfig{
				Spec: buildapiv1.BuildConfigSpec{
					CommonSpec: buildapiv1.CommonSpec{
						Source: buildapiv1.BuildSource{
							Type: buildapiv1.BuildSourceBinary,
						},
						Strategy: buildapiv1.BuildStrategy{
							Type: buildapiv1.DockerBuildStrategyType,
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*buildapiv1.BuildConfig)
				if !ok {
					return false
				}
				binary := obj.Spec.Source.Binary
				if binary == (*buildapiv1.BinaryBuildSource)(nil) || *binary != (buildapiv1.BinaryBuildSource{}) {
					return false
				}

				dockerStrategy := obj.Spec.Strategy.DockerStrategy
				// DeepEqual needed because DockerBuildStrategy contains slices
				if dockerStrategy == (*buildapiv1.DockerBuildStrategy)(nil) || !reflect.DeepEqual(*dockerStrategy, buildapiv1.DockerBuildStrategy{}) {
					return false
				}
				return true
			},
		},
	}

	for i, test := range testCases {
		obj := roundTrip(t, test.External)
		if !test.Ok(obj) {
			t.Errorf("%d: unexpected defaults: %#v", i, obj)
		}
	}
}

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := runtime.Encode(kapi.Codecs.LegacyCodec(buildapiv1.LegacySchemeGroupVersion), obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	kapi.Scheme.Default(obj2)
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = kapi.Scheme.Convert(obj2, obj3, nil)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}
