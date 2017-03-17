package v1_test

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/build/api/v1"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestDefaults(t *testing.T) {
	testCases := []struct {
		External runtime.Object
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
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*v1.Build)
				if !ok {
					return false
				}
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
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*v1.Build)
				if !ok {
					return false
				}
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
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*v1.Build)
				if !ok {
					return false
				}
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
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*v1.Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.CustomStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &v1.BuildConfig{
				Spec: v1.BuildConfigSpec{Triggers: []v1.BuildTriggerPolicy{{Type: v1.ImageChangeBuildTriggerType}}},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*v1.BuildConfig)
				if !ok {
					return false
				}
				// conversion drops this trigger because it has no type
				return (len(obj.Spec.Triggers) == 0) && (obj.Spec.RunPolicy == v1.BuildRunPolicySerial)
			},
		},
		{
			External: &v1.BuildConfig{
				Spec: v1.BuildConfigSpec{
					CommonSpec: v1.CommonSpec{
						Source: v1.BuildSource{
							Type: v1.BuildSourceBinary,
						},
						Strategy: v1.BuildStrategy{
							Type: v1.DockerBuildStrategyType,
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*v1.BuildConfig)
				if !ok {
					return false
				}
				binary := obj.Spec.Source.Binary
				if binary == (*v1.BinaryBuildSource)(nil) || *binary != (v1.BinaryBuildSource{}) {
					return false
				}

				dockerStrategy := obj.Spec.Strategy.DockerStrategy
				// DeepEqual needed because DockerBuildStrategy contains slices
				if dockerStrategy == (*v1.DockerBuildStrategy)(nil) || !reflect.DeepEqual(*dockerStrategy, v1.DockerBuildStrategy{}) {
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
	data, err := runtime.Encode(kapi.Codecs.LegacyCodec(v1.LegacySchemeGroupVersion), obj)
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
