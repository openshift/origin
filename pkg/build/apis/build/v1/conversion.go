package v1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	newer "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func Convert_v1_BuildConfig_To_build_BuildConfig(in *v1.BuildConfig, out *newer.BuildConfig, s conversion.Scope) error {
	if err := autoConvert_v1_BuildConfig_To_build_BuildConfig(in, out, s); err != nil {
		return err
	}

	newTriggers := []newer.BuildTriggerPolicy{}
	// Strip off any default imagechange triggers where the buildconfig's
	// "from" is not an ImageStreamTag, because those triggers
	// will never be invoked.
	imageRef := newer.GetInputReference(out.Spec.Strategy)
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

func Convert_v1_SourceBuildStrategy_To_build_SourceBuildStrategy(in *v1.SourceBuildStrategy, out *newer.SourceBuildStrategy, s conversion.Scope) error {
	if err := autoConvert_v1_SourceBuildStrategy_To_build_SourceBuildStrategy(in, out, s); err != nil {
		return err
	}
	switch in.From.Kind {
	case "ImageStream":
		out.From.Kind = "ImageStreamTag"
		out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, "")
	}
	return nil
}

func Convert_v1_DockerBuildStrategy_To_build_DockerBuildStrategy(in *v1.DockerBuildStrategy, out *newer.DockerBuildStrategy, s conversion.Scope) error {
	if err := autoConvert_v1_DockerBuildStrategy_To_build_DockerBuildStrategy(in, out, s); err != nil {
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

func Convert_v1_CustomBuildStrategy_To_build_CustomBuildStrategy(in *v1.CustomBuildStrategy, out *newer.CustomBuildStrategy, s conversion.Scope) error {
	if err := autoConvert_v1_CustomBuildStrategy_To_build_CustomBuildStrategy(in, out, s); err != nil {
		return err
	}
	switch in.From.Kind {
	case "ImageStream":
		out.From.Kind = "ImageStreamTag"
		out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, "")
	}
	return nil
}

func Convert_v1_BuildOutput_To_build_BuildOutput(in *v1.BuildOutput, out *newer.BuildOutput, s conversion.Scope) error {
	if err := autoConvert_v1_BuildOutput_To_build_BuildOutput(in, out, s); err != nil {
		return err
	}
	if in.To != nil && (in.To.Kind == "ImageStream" || len(in.To.Kind) == 0) {
		out.To.Kind = "ImageStreamTag"
		out.To.Name = imageapi.JoinImageStreamTag(in.To.Name, "")
	}
	return nil
}

func Convert_v1_BuildTriggerPolicy_To_build_BuildTriggerPolicy(in *v1.BuildTriggerPolicy, out *newer.BuildTriggerPolicy, s conversion.Scope) error {
	if err := autoConvert_v1_BuildTriggerPolicy_To_build_BuildTriggerPolicy(in, out, s); err != nil {
		return err
	}

	switch in.Type {
	case v1.ImageChangeBuildTriggerTypeDeprecated:
		out.Type = newer.ImageChangeBuildTriggerType
	case v1.GenericWebHookBuildTriggerTypeDeprecated:
		out.Type = newer.GenericWebHookBuildTriggerType
	case v1.GitHubWebHookBuildTriggerTypeDeprecated:
		out.Type = newer.GitHubWebHookBuildTriggerType
	}
	return nil
}

func Convert_build_SourceRevision_To_v1_SourceRevision(in *newer.SourceRevision, out *v1.SourceRevision, s conversion.Scope) error {
	if err := autoConvert_build_SourceRevision_To_v1_SourceRevision(in, out, s); err != nil {
		return err
	}
	out.Type = v1.BuildSourceGit
	return nil
}

func Convert_build_BuildSource_To_v1_BuildSource(in *newer.BuildSource, out *v1.BuildSource, s conversion.Scope) error {
	if err := autoConvert_build_BuildSource_To_v1_BuildSource(in, out, s); err != nil {
		return err
	}
	switch {
	// It is legal for a buildsource to have both a git+dockerfile source, but in v1 that was represented
	// as type git.
	case in.Git != nil:
		out.Type = v1.BuildSourceGit
	// It is legal for a buildsource to have both a binary+dockerfile source, but in v1 that was represented
	// as type binary.
	case in.Binary != nil:
		out.Type = v1.BuildSourceBinary
	case in.Dockerfile != nil:
		out.Type = v1.BuildSourceDockerfile
	case len(in.Images) > 0:
		out.Type = v1.BuildSourceImage
	default:
		out.Type = v1.BuildSourceNone
	}
	return nil
}

func Convert_build_BuildStrategy_To_v1_BuildStrategy(in *newer.BuildStrategy, out *v1.BuildStrategy, s conversion.Scope) error {
	if err := autoConvert_build_BuildStrategy_To_v1_BuildStrategy(in, out, s); err != nil {
		return err
	}
	switch {
	case in.SourceStrategy != nil:
		out.Type = v1.SourceBuildStrategyType
	case in.DockerStrategy != nil:
		out.Type = v1.DockerBuildStrategyType
	case in.CustomStrategy != nil:
		out.Type = v1.CustomBuildStrategyType
	case in.JenkinsPipelineStrategy != nil:
		out.Type = v1.JenkinsPipelineBuildStrategyType
	default:
		out.Type = ""
	}
	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddConversionFuncs(
		Convert_v1_BuildConfig_To_build_BuildConfig,
		Convert_build_BuildConfig_To_v1_BuildConfig,
		Convert_v1_SourceBuildStrategy_To_build_SourceBuildStrategy,
		Convert_build_SourceBuildStrategy_To_v1_SourceBuildStrategy,
		Convert_v1_DockerBuildStrategy_To_build_DockerBuildStrategy,
		Convert_build_DockerBuildStrategy_To_v1_DockerBuildStrategy,
		Convert_v1_CustomBuildStrategy_To_build_CustomBuildStrategy,
		Convert_build_CustomBuildStrategy_To_v1_CustomBuildStrategy,
		Convert_v1_BuildOutput_To_build_BuildOutput,
		Convert_build_BuildOutput_To_v1_BuildOutput,
		Convert_v1_BuildTriggerPolicy_To_build_BuildTriggerPolicy,
		Convert_build_BuildTriggerPolicy_To_v1_BuildTriggerPolicy,
		Convert_v1_SourceRevision_To_build_SourceRevision,
		Convert_build_SourceRevision_To_v1_SourceRevision,
		Convert_v1_BuildSource_To_build_BuildSource,
		Convert_build_BuildSource_To_v1_BuildSource,
		Convert_v1_BuildStrategy_To_build_BuildStrategy,
		Convert_build_BuildStrategy_To_v1_BuildStrategy,
	)
}

func addLegacyFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(LegacySchemeGroupVersion.String(), "Build", legacyBuildFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(LegacySchemeGroupVersion.String(), "BuildConfig", apihelpers.LegacyMetaV1FieldSelectorConversionWithName); err != nil {
		return err
	}
	return nil
}

func addFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "Build", buildFieldSelectorKeyConversionFunc)
}

// because field selectors can vary in support by version they are exposed under, we have one function for each
// groupVersion we're registering for

func legacyBuildFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "status",
		"podName":
		return label, value, nil
	default:
		return apihelpers.LegacyMetaV1FieldSelectorConversionWithName(label, value)
	}
}

func buildFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "status",
		"podName":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}
