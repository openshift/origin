package v1

import (
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	newer "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func Convert_v1_BuildConfig_To_api_BuildConfig(in *BuildConfig, out *newer.BuildConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	newTriggers := []newer.BuildTriggerPolicy{}
	// strip off any default imagechange triggers where the buildconfig's
	// "from" is not an ImageStreamTag, because those triggers
	// will never be invoked.
	imageRef := buildutil.GetInputReference(out.Spec.Strategy)
	hasIST := imageRef != nil && imageRef.Kind == "ImageStreamTag"
	for _, trigger := range out.Spec.Triggers {
		if trigger.Type != newer.ImageChangeBuildTriggerType {
			newTriggers = append(newTriggers, trigger)
			continue
		}
		if (trigger.ImageChange == nil || trigger.ImageChange.From == nil) && !hasIST {
			continue
		}
		newTriggers = append(newTriggers, trigger)
	}
	out.Spec.Triggers = newTriggers
	return nil
}

// empty conversion needed because the conversion generator can't handle unidirectional custom conversions
func Convert_api_BuildConfig_To_v1_BuildConfig(in *newer.BuildConfig, out *BuildConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func Convert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy(in *SourceBuildStrategy, out *newer.SourceBuildStrategy, s conversion.Scope) error {
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
func Convert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy(in *newer.SourceBuildStrategy, out *SourceBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func Convert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy(in *DockerBuildStrategy, out *newer.DockerBuildStrategy, s conversion.Scope) error {
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
func Convert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy(in *newer.DockerBuildStrategy, out *DockerBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func Convert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy(in *CustomBuildStrategy, out *newer.CustomBuildStrategy, s conversion.Scope) error {
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
func Convert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy(in *newer.CustomBuildStrategy, out *CustomBuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func Convert_v1_BuildOutput_To_api_BuildOutput(in *BuildOutput, out *newer.BuildOutput, s conversion.Scope) error {
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
func Convert_api_BuildOutput_To_v1_BuildOutput(in *newer.BuildOutput, out *BuildOutput, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

// empty conversion needed because the conversion generator can't handle unidirectional custom conversions
func Convert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy(in *newer.BuildTriggerPolicy, out *BuildTriggerPolicy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.DestFromSource); err != nil {
		return err
	}
	return nil
}

func Convert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy(in *BuildTriggerPolicy, out *newer.BuildTriggerPolicy, s conversion.Scope) error {
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

func Convert_api_SourceRevision_To_v1_SourceRevision(in *newer.SourceRevision, out *SourceRevision, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	out.Type = BuildSourceGit
	return nil
}

func Convert_v1_SourceRevision_To_api_SourceRevision(in *SourceRevision, out *newer.SourceRevision, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func Convert_api_BuildSource_To_v1_BuildSource(in *newer.BuildSource, out *BuildSource, s conversion.Scope) error {
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
	case len(in.Images) > 0:
		out.Type = BuildSourceImage
	default:
		out.Type = BuildSourceNone
	}
	return nil
}

func Convert_v1_BuildSource_To_api_BuildSource(in *BuildSource, out *newer.BuildSource, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func Convert_api_BuildStrategy_To_v1_BuildStrategy(in *newer.BuildStrategy, out *BuildStrategy, s conversion.Scope) error {
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

func Convert_v1_BuildStrategy_To_api_BuildStrategy(in *BuildStrategy, out *newer.BuildStrategy, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		func(source *BuildSource) {
			if (source != nil) && (source.Type == BuildSourceBinary) && (source.Binary == nil) {
				source.Binary = &BinaryBuildSource{}
			}
		},
		func(strategy *BuildStrategy) {
			if (strategy != nil) && (strategy.Type == DockerBuildStrategyType) && (strategy.DockerStrategy == nil) {
				strategy.DockerStrategy = &DockerBuildStrategy{}
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
		// TODO: this defaulter is never called, because triggers with ImageChange type but imagechange nil
		// are dropped.
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
		Convert_v1_BuildConfig_To_api_BuildConfig,
		Convert_api_BuildConfig_To_v1_BuildConfig,
		Convert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy,
		Convert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy,
		Convert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy,
		Convert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy,
		Convert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy,
		Convert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy,
		Convert_v1_BuildOutput_To_api_BuildOutput,
		Convert_api_BuildOutput_To_v1_BuildOutput,
		Convert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
		Convert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy,
		Convert_v1_SourceRevision_To_api_SourceRevision,
		Convert_api_SourceRevision_To_v1_SourceRevision,
		Convert_v1_BuildSource_To_api_BuildSource,
		Convert_api_BuildSource_To_v1_BuildSource,
		Convert_v1_BuildStrategy_To_api_BuildStrategy,
		Convert_api_BuildStrategy_To_v1_BuildStrategy,
	)

	if err := scheme.AddFieldLabelConversionFunc("v1", "Build",
		oapi.GetFieldLabelConversionFunc(newer.BuildToSelectableFields(&newer.Build{}), map[string]string{"name": "metadata.name"}),
	); err != nil {
		panic(err)
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "BuildConfig",
		oapi.GetFieldLabelConversionFunc(newer.BuildConfigToSelectableFields(&newer.BuildConfig{}), map[string]string{"name": "metadata.name"}),
	); err != nil {
		panic(err)
	}
}
