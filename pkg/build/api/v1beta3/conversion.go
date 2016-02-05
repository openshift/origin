package v1beta3

import (
	"fmt"

	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"

	newer "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func convert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy(in *SourceBuildStrategy, out *newer.SourceBuildStrategy, s conversion.Scope) error {
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
func convert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy(in *newer.SourceBuildStrategy, out *SourceBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_DockerBuildStrategy_To_api_DockerBuildStrategy(in *DockerBuildStrategy, out *newer.DockerBuildStrategy, s conversion.Scope) error {
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
func convert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy(in *newer.DockerBuildStrategy, out *DockerBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy(in *CustomBuildStrategy, out *newer.CustomBuildStrategy, s conversion.Scope) error {
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
func convert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy(in *newer.CustomBuildStrategy, out *CustomBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_BuildOutput_To_api_BuildOutput(in *BuildOutput, out *newer.BuildOutput, s conversion.Scope) error {
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
func convert_api_BuildOutput_To_v1beta3_BuildOutput(in *newer.BuildOutput, out *BuildOutput, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
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

func convert_v1beta3_SourceRevision_To_api_SourceRevision(in *SourceRevision, out *newer.SourceRevision, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_api_SourceRevision_To_v1beta3_SourceRevision(in *newer.SourceRevision, out *SourceRevision, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	out.Type = BuildSourceGit
	return nil
}

func convert_v1beta3_BuildSource_To_api_BuildSource(in *BuildSource, out *newer.BuildSource, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildSource_To_v1beta3_BuildSource(in *newer.BuildSource, out *BuildSource, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	switch {
	// it is legal for a buildsource to have both a git+dockerfile source, but in v1 that was represented
	// as type git.
	case in.Git != nil:
		out.Type = BuildSourceGit
	// it is legal for a buildsource to have both a binary+dockerfile source, but in v1 that was represented
	// as type binary.
	case in.Binary != nil:
		out.Type = BuildSourceBinary
	case in.Dockerfile != nil:
		out.Type = BuildSourceDockerfile
	}
	return nil
}

func convert_v1beta3_BuildStrategy_To_api_BuildStrategy(in *BuildStrategy, out *newer.BuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildStrategy_To_v1beta3_BuildStrategy(in *newer.BuildStrategy, out *BuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	switch {
	case in.SourceStrategy != nil:
		out.Type = SourceBuildStrategyType
	case in.DockerStrategy != nil:
		out.Type = DockerBuildStrategyType
	case in.CustomStrategy != nil:
		out.Type = CustomBuildStrategyType
	}
	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
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

	scheme.AddConversionFuncs(
		convert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy,
		convert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy,
		convert_v1beta3_DockerBuildStrategy_To_api_DockerBuildStrategy,
		convert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy,
		convert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy,
		convert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy,
		convert_v1beta3_BuildOutput_To_api_BuildOutput,
		convert_api_BuildOutput_To_v1beta3_BuildOutput,
		convert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
		convert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy,
		convert_v1beta3_SourceRevision_To_api_SourceRevision,
		convert_api_SourceRevision_To_v1beta3_SourceRevision,
		convert_v1beta3_BuildSource_To_api_BuildSource,
		convert_api_BuildSource_To_v1beta3_BuildSource,
		convert_v1beta3_BuildStrategy_To_api_BuildStrategy,
		convert_api_BuildStrategy_To_v1beta3_BuildStrategy,
	)

	// Add field conversion funcs.
	err = scheme.AddFieldLabelConversionFunc("v1beta3", "Build",
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
	err = scheme.AddFieldLabelConversionFunc("v1beta3", "BuildConfig",
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
