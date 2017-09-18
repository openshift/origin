package v1

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
)

func TestDefaults(t *testing.T) {
	testCases := []struct {
		External runtime.Object
		Ok       func(runtime.Object) bool
	}{
		{
			External: &Build{
				Spec: BuildSpec{
					CommonSpec: CommonSpec{
						Strategy: BuildStrategy{
							Type: DockerBuildStrategyType,
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.DockerStrategy != nil
			},
		},
		{
			External: &Build{
				Spec: BuildSpec{
					CommonSpec: CommonSpec{
						Strategy: BuildStrategy{
							SourceStrategy: &SourceBuildStrategy{},
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.SourceStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &Build{
				Spec: BuildSpec{
					CommonSpec: CommonSpec{
						Strategy: BuildStrategy{
							DockerStrategy: &DockerBuildStrategy{
								From: &kapiv1.ObjectReference{},
							},
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.DockerStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &Build{
				Spec: BuildSpec{
					CommonSpec: CommonSpec{
						Strategy: BuildStrategy{
							CustomStrategy: &CustomBuildStrategy{},
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*Build)
				if !ok {
					return false
				}
				return obj.Spec.Strategy.CustomStrategy.From.Kind == "ImageStreamTag"
			},
		},
		{
			External: &BuildConfig{
				Spec: BuildConfigSpec{Triggers: []BuildTriggerPolicy{{Type: ImageChangeBuildTriggerType}}},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*BuildConfig)
				if !ok {
					return false
				}
				// conversion drops this trigger because it has no type
				return (len(obj.Spec.Triggers) == 0) && (obj.Spec.RunPolicy == BuildRunPolicySerial)
			},
		},
		{
			External: &BuildConfig{
				Spec: BuildConfigSpec{
					CommonSpec: CommonSpec{
						Source: BuildSource{
							Type: BuildSourceBinary,
						},
						Strategy: BuildStrategy{
							Type: DockerBuildStrategyType,
						},
					},
				},
			},
			Ok: func(out runtime.Object) bool {
				obj, ok := out.(*BuildConfig)
				if !ok {
					return false
				}
				binary := obj.Spec.Source.Binary
				if binary == (*BinaryBuildSource)(nil) || *binary != (BinaryBuildSource{}) {
					return false
				}

				dockerStrategy := obj.Spec.Strategy.DockerStrategy
				// DeepEqual needed because DockerBuildStrategy contains slices
				if dockerStrategy == (*DockerBuildStrategy)(nil) || !reflect.DeepEqual(*dockerStrategy, DockerBuildStrategy{}) {
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
	scheme.Default(obj2)
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = scheme.Convert(obj2, obj3, nil)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}
