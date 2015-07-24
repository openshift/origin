package v1

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func convert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy(in *SourceBuildStrategy, out *newer.SourceBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	switch in.From.Kind {
	case "ImageStream":
		out.From.Kind = "ImageStreamTag"
		out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, "")
	}
	return nil
}

// empty conversion needed because the conversion generator can't handle unidirectional custom conversions
func convert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy(in *newer.SourceBuildStrategy, out *SourceBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy(in *DockerBuildStrategy, out *newer.DockerBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if in.From != nil {
		switch in.From.Kind {
		case "ImageStream":
			out.From.Kind = "ImageStreamTag"
			out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, "")
		}
	}
	return nil
}

// empty conversion needed because the conversion generator can't handle unidirectional custom conversions
func convert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy(in *newer.DockerBuildStrategy, out *DockerBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy(in *CustomBuildStrategy, out *newer.CustomBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	switch in.From.Kind {
	case "ImageStream":
		out.From.Kind = "ImageStreamTag"
		out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, "")
	}
	return nil
}

// empty conversion needed because the conversion generator can't handle unidirectional custom conversions
func convert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy(in *newer.CustomBuildStrategy, out *CustomBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_v1_BuildOutput_To_api_BuildOutput(in *BuildOutput, out *newer.BuildOutput, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if in.To != nil && (in.To.Kind == "ImageStream" || len(in.To.Kind) == 0) {
		out.To.Kind = "ImageStreamTag"
		out.To.Name = imageapi.JoinImageStreamTag(in.To.Name, "")
	}
	return nil
}

// empty conversion needed because the conversion generator can't handle unidirectional custom conversions
func convert_api_BuildOutput_To_v1_BuildOutput(in *newer.BuildOutput, out *BuildOutput, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

// empty conversion needed because the conversion generator can't handle unidirectional custom conversions
func convert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy(in *newer.BuildTriggerPolicy, out *BuildTriggerPolicy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.DestFromSource); err != nil {
		return err
	}
	return nil
}

func convert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy(in *BuildTriggerPolicy, out *newer.BuildTriggerPolicy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.DestFromSource); err != nil {
		return err
	}
	switch in.Type {
	case ImageChangeBuildTriggerTypeDeprecated:
		out.Type = newer.ImageChangeBuildTriggerType
	case GenericWebHookBuildTriggerTypeDeprecated:
		out.Type = newer.GenericWebHookBuildTriggerType
	case GitHubWebHookBuildTriggerTypeDeprecated:
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
		convert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy,
		convert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy,
		convert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy,
		convert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy,
		convert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy,
		convert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy,
		convert_v1_BuildOutput_To_api_BuildOutput,
		convert_api_BuildOutput_To_v1_BuildOutput,
		convert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
		convert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy,
	)

	// Add field conversion funcs.
	err = kapi.Scheme.AddFieldLabelConversionFunc("v1", "Build",
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
	err = kapi.Scheme.AddFieldLabelConversionFunc("v1", "BuildConfig",
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
