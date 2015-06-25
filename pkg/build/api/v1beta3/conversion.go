package v1beta3

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi_v1beta3 "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func convert_api_Build_To_v1beta3_Build(in *newer.Build, out *Build, s conversion.Scope) error {
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Parameters, &out.Spec, 0); err != nil {
		return err
	}
	if err := s.Convert(in, &out.Status, 0); err != nil {
		return err
	}
	return s.Convert(&in.Status, &out.Status.Phase, 0)
}

func convert_v1beta3_Build_To_api_Build(in *Build, out *newer.Build, s conversion.Scope) error {
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Status, out, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Parameters, 0); err != nil {
		return err
	}
	return s.Convert(&in.Status.Phase, &out.Status, 0)
}

func convert_api_Build_To_v1beta3_BuildStatus(in *newer.Build, out *BuildStatus, s conversion.Scope) error {
	out.Cancelled = in.Cancelled
	out.CompletionTimestamp = in.CompletionTimestamp
	if err := s.Convert(&in.Config, &out.Config, 0); err != nil {
		return err
	}
	out.Duration = in.Duration
	out.Message = in.Message
	out.StartTimestamp = in.StartTimestamp
	return nil
}

func convert_v1beta3_BuildStatus_To_api_Build(in *BuildStatus, out *newer.Build, s conversion.Scope) error {
	out.Cancelled = in.Cancelled
	out.CompletionTimestamp = in.CompletionTimestamp
	if err := s.Convert(&in.Config, &out.Config, 0); err != nil {
		return err
	}
	out.Duration = in.Duration
	out.Message = in.Message
	out.StartTimestamp = in.StartTimestamp
	return nil
}

func convert_api_BuildStatus_To_v1beta3_BuildPhase(in *newer.BuildStatus, out *BuildPhase, s conversion.Scope) error {
	str := string(*in)
	*out = BuildPhase(str)
	return nil
}

func convert_v1beta3_BuildPhase_To_api_BuildStatus(in *BuildPhase, out *newer.BuildStatus, s conversion.Scope) error {
	str := string(*in)
	*out = newer.BuildStatus(str)
	return nil
}

func convert_api_BuildConfig_To_v1beta3_BuildConfig(in *newer.BuildConfig, out *BuildConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if err := s.Convert(&in.Parameters, &out.Spec, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Triggers, &out.Spec.Triggers, 0); err != nil {
		return err
	}
	out.Status.LastVersion = in.LastVersion
	return nil
}

func convert_v1beta3_BuildConfig_To_api_BuildConfig(in *BuildConfig, out *newer.BuildConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Parameters, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec.Triggers, &out.Triggers, 0); err != nil {
		return err
	}
	out.LastVersion = in.Status.LastVersion
	return nil
}

func convert_api_BuildParameters_To_v1beta3_BuildSpec(in *newer.BuildParameters, out *BuildSpec, s conversion.Scope) error {
	out.ServiceAccount = in.ServiceAccount
	if err := s.Convert(&in.Strategy, &out.Strategy, 0); err != nil {
		return err
	}
	if in.Strategy.Type == newer.SourceBuildStrategyType {
		out.Strategy.Type = SourceBuildStrategyType
	}
	if err := s.Convert(&in.Source, &out.Source, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_BuildSpec_To_api_BuildParameters(in *BuildSpec, out *newer.BuildParameters, s conversion.Scope) error {
	out.ServiceAccount = in.ServiceAccount
	if err := s.Convert(&in.Strategy, &out.Strategy, 0); err != nil {
		return err
	}
	if in.Strategy.Type == SourceBuildStrategyType {
		out.Strategy.Type = newer.SourceBuildStrategyType
	}
	if err := s.Convert(&in.Source, &out.Source, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildParameters_To_v1beta3_BuildConfigSpec(in *newer.BuildParameters, out *BuildConfigSpec, s conversion.Scope) error {
	out.ServiceAccount = in.ServiceAccount
	if err := s.Convert(&in.Strategy, &out.Strategy, 0); err != nil {
		return err
	}
	if in.Strategy.Type == newer.SourceBuildStrategyType {
		out.Strategy.Type = SourceBuildStrategyType
	}
	if err := s.Convert(&in.Source, &out.Source, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_BuildConfigSpec_To_api_BuildParameters(in *BuildConfigSpec, out *newer.BuildParameters, s conversion.Scope) error {
	out.ServiceAccount = in.ServiceAccount
	if err := s.Convert(&in.Strategy, &out.Strategy, 0); err != nil {
		return err
	}
	if in.Strategy.Type == SourceBuildStrategyType {
		out.Strategy.Type = newer.SourceBuildStrategyType
	}
	if err := s.Convert(&in.Source, &out.Source, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy(in *newer.SourceBuildStrategy, out *SourceBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if in.From != nil && in.From.Kind == "ImageStream" {
		out.From.Kind = "ImageStreamTag"
		out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, "")
	}
	return nil
}

func convert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy(in *SourceBuildStrategy, out *newer.SourceBuildStrategy, s conversion.Scope) error {
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

func convert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy(in *newer.DockerBuildStrategy, out *DockerBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if in.From != nil && in.From.Kind == "ImageStream" {
		out.From.Kind = "ImageStreamTag"
		out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, "")
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

func convert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy(in *newer.CustomBuildStrategy, out *CustomBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if in.From != nil && in.From.Kind == "ImageStream" {
		out.From.Kind = "ImageStreamTag"
		out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, "")
	}
	return nil
}

func convert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy(in *CustomBuildStrategy, out *newer.CustomBuildStrategy, s conversion.Scope) error {
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

func convert_api_BuildOutput_To_v1beta3_BuildOutput(in *newer.BuildOutput, out *BuildOutput, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if in.To != nil && (len(in.To.Kind) == 0 || in.To.Kind == "ImageStream") {
		out.To.Kind = "ImageStreamTag"
		out.To.Name = imageapi.JoinImageStreamTag(in.To.Name, in.Tag)
		return nil
	}
	if len(in.DockerImageReference) != 0 {
		out.To = &kapi_v1beta3.ObjectReference{
			Kind: "DockerImage",
			Name: imageapi.JoinImageStreamTag(in.DockerImageReference, in.Tag),
		}
	}
	return nil
}

func convert_v1beta3_BuildOutput_To_api_BuildOutput(in *BuildOutput, out *newer.BuildOutput, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if in.To != nil && in.To.Kind == "ImageStreamTag" {
		name, tag, ok := imageapi.SplitImageStreamTag(in.To.Name)
		if !ok {
			return fmt.Errorf("ImageStreamTag object references must be in the form <name>:<tag>: %s", in.To.Name)
		}
		out.To.Kind = "ImageStream"
		out.To.Name = name
		out.Tag = tag
		return nil
	}
	if in.To != nil && in.To.Kind == "DockerImage" {
		out.To = nil
		if ref, err := imageapi.ParseDockerImageReference(in.To.Name); err == nil {
			out.Tag = ref.Tag
			ref.Tag = ""
			out.DockerImageReference = ref.String()
		} else {
			out.DockerImageReference = in.To.Name
		}
	}
	return nil
}

func convert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in *newer.ImageChangeTrigger, out *ImageChangeTrigger, s conversion.Scope) error {
	out.LastTriggeredImageID = in.LastTriggeredImageID
	return nil
}

func convert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in *ImageChangeTrigger, out *newer.ImageChangeTrigger, s conversion.Scope) error {
	out.LastTriggeredImageID = in.LastTriggeredImageID
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
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = kapi.Scheme.AddDefaultingFuncs(
		func(obj *SourceBuildStrategy) {
			if obj.From != nil && len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
		func(obj *DockerBuildStrategy) {
			if obj.From != nil && len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
		func(obj *CustomBuildStrategy) {
			if obj.From != nil && len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
	)
	if err != nil {
		panic(err)
	}

	kapi.Scheme.AddConversionFuncs(
		convert_api_Build_To_v1beta3_Build,
		convert_v1beta3_Build_To_api_Build,
		convert_api_Build_To_v1beta3_BuildStatus,
		convert_v1beta3_BuildStatus_To_api_Build,
		convert_api_BuildStatus_To_v1beta3_BuildPhase,
		convert_v1beta3_BuildPhase_To_api_BuildStatus,
		convert_api_BuildConfig_To_v1beta3_BuildConfig,
		convert_v1beta3_BuildConfig_To_api_BuildConfig,
		convert_api_BuildParameters_To_v1beta3_BuildSpec,
		convert_v1beta3_BuildSpec_To_api_BuildParameters,
		convert_api_BuildParameters_To_v1beta3_BuildConfigSpec,
		convert_v1beta3_BuildConfigSpec_To_api_BuildParameters,
		convert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy,
		convert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy,
		convert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy,
		convert_v1beta3_DockerBuildStrategy_To_api_DockerBuildStrategy,
		convert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy,
		convert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy,
		convert_api_BuildOutput_To_v1beta3_BuildOutput,
		convert_v1beta3_BuildOutput_To_api_BuildOutput,
		convert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger,
		convert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger,
		convert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy,
		convert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
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
