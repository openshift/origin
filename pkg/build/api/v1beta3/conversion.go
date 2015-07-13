package v1beta3

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/build/api"
)

func convert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy(in *newer.BuildTriggerPolicy, out *BuildTriggerPolicy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.DestFromSource); err != nil {
		return err
	}
	switch in.Type {
	case newer.ImageChangeBuildTriggerType:
		out.Type = ImageChangeBuildTriggerType
	case newer.GenericWebHookBuildTriggerType:
		out.Type = GenericWebHookBuildTriggerType
	case newer.GitHubWebHookBuildTriggerType:
		out.Type = GitHubWebHookBuildTriggerType
	}
	return nil
}

func convert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy(in *BuildTriggerPolicy, out *newer.BuildTriggerPolicy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.DestFromSource); err != nil {
		return err
	}
	switch in.Type {
	case ImageChangeBuildTriggerType:
		out.Type = newer.ImageChangeBuildTriggerType
	case GenericWebHookBuildTriggerType:
		out.Type = newer.GenericWebHookBuildTriggerType
	case GitHubWebHookBuildTriggerType:
		out.Type = newer.GitHubWebHookBuildTriggerType
	}
	return nil
}

func init() {
	err := kapi.Scheme.AddDefaultingFuncs(
		func(strategy *BuildStrategy) {
			if (strategy != nil) && (strategy.Type == DockerBuildStrategyType) {
				//  initialize DockerStrategy to a default state if it's not set.
				if strategy.DockerStrategy == nil {
					strategy.DockerStrategy = &DockerBuildStrategy{}
				}
			}
		},
		func(obj *SourceBuildStrategy) {
			if len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
		func(obj *DockerBuildStrategy) {
			if obj.From != nil && len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
		func(obj *CustomBuildStrategy) {
			if len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
		func(obj *BuildTriggerPolicy) {
			if obj.Type == ImageChangeBuildTriggerType && obj.ImageChange == nil {
				obj.ImageChange = &ImageChangeTrigger{}
			}
		},
	)
	if err != nil {
		panic(err)
	}

	kapi.Scheme.AddConversionFuncs(
		convert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
		convert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy,
	)

	// Add field conversion funcs.
	err = kapi.Scheme.AddFieldLabelConversionFunc("v1beta3", "Build",
		func(label, value string) (string, string, error) {
			switch label {
			case "name":
				return "metadata.name", value, nil
			case "status":
				return "status", value, nil
			case "podName":
				return "podName", value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = kapi.Scheme.AddFieldLabelConversionFunc("v1beta3", "BuildConfig",
		func(label, value string) (string, string, error) {
			switch label {
			case "name":
				return "metadata.name", value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
