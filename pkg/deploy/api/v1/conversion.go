package v1

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func convert_v1_DeploymentConfig_To_api_DeploymentConfig(in *DeploymentConfig, out *newer.DeploymentConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Template, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec.Triggers, &out.Triggers, 0); err != nil {
		return err
	}
	out.LatestVersion = in.Status.LatestVersion
	if err := s.Convert(&in.Status.Details, &out.Details, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentConfig_To_v1_DeploymentConfig(in *newer.DeploymentConfig, out *DeploymentConfig, s conversion.Scope) error {
	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}
	if err := s.Convert(&in.Template, &out.Spec, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Triggers, &out.Spec.Triggers, 0); err != nil {
		return err
	}
	out.Status.LatestVersion = in.LatestVersion
	if err := s.Convert(&in.Details, &out.Status.Details, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_DeploymentConfigSpec_To_api_DeploymentTemplate(in *DeploymentConfigSpec, out *newer.DeploymentTemplate, s conversion.Scope) error {
	out.ControllerTemplate.Replicas = in.Replicas
	if in.Selector != nil {
		out.ControllerTemplate.Selector = make(map[string]string)
		for k, v := range in.Selector {
			out.ControllerTemplate.Selector[k] = v
		}
	}
	if in.Template != nil {
		if err := s.Convert(&in.Template, &out.ControllerTemplate.Template, 0); err != nil {
			return err
		}
	}
	if in.TemplateRef != nil {
		if err := s.Convert(&in.TemplateRef, &out.ControllerTemplate.TemplateRef, 0); err != nil {
			return err
		}
	}
	if err := s.Convert(&in.Strategy, &out.Strategy, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_DeploymentStrategy_To_api_DeploymentStrategy(in *DeploymentStrategy, out *newer.DeploymentStrategy, s conversion.Scope) error {
	if err := s.Convert(&in.Type, &out.Type, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.CustomParams, &out.CustomParams, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.RecreateParams, &out.RecreateParams, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.RollingParams, &out.RollingParams, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentStrategy_To_v1_DeploymentStrategy(in *newer.DeploymentStrategy, out *DeploymentStrategy, s conversion.Scope) error {
	if err := s.Convert(&in.Type, &out.Type, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.CustomParams, &out.CustomParams, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.RecreateParams, &out.RecreateParams, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.RollingParams, &out.RollingParams, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentTemplate_To_v1_DeploymentConfigSpec(in *newer.DeploymentTemplate, out *DeploymentConfigSpec, s conversion.Scope) error {
	out.Replicas = in.ControllerTemplate.Replicas
	if in.ControllerTemplate.Selector != nil {
		out.Selector = make(map[string]string)
		for k, v := range in.ControllerTemplate.Selector {
			out.Selector[k] = v
		}
	}
	if in.ControllerTemplate.Template != nil {
		if err := s.Convert(&in.ControllerTemplate.Template, &out.Template, 0); err != nil {
			return err
		}
	}
	if in.ControllerTemplate.TemplateRef != nil {
		if err := s.Convert(&in.ControllerTemplate.TemplateRef, &out.TemplateRef, 0); err != nil {
			return err
		}
	}
	if err := s.Convert(&in.Strategy, &out.Strategy, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams(in *DeploymentTriggerImageChangeParams, out *newer.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
	out.Automatic = in.Automatic
	out.ContainerNames = make([]string, len(in.ContainerNames))
	copy(out.ContainerNames, in.ContainerNames)
	out.LastTriggeredImage = in.LastTriggeredImage
	if err := s.Convert(&in.From, &out.From, 0); err != nil {
		return err
	}
	switch in.From.Kind {
	case "DockerImage":
		ref, err := imageapi.ParseDockerImageReference(in.From.Name)
		if err != nil {
			return err
		}
		out.Tag = ref.Tag
		ref.Tag, ref.ID = "", ""
		out.RepositoryName = ref.String()
	case "ImageStreamTag":
		name, tag, ok := imageapi.SplitImageStreamTag(in.From.Name)
		if !ok {
			return fmt.Errorf("ImageStreamTag object references must be in the form <name>:<tag>: %s", in.From.Name)
		}
		out.From.Kind = "ImageStream"
		out.From.Name = name
		out.Tag = tag
	}
	return nil
}

func convert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams(in *newer.DeploymentTriggerImageChangeParams, out *DeploymentTriggerImageChangeParams, s conversion.Scope) error {
	out.Automatic = in.Automatic
	out.ContainerNames = make([]string, len(in.ContainerNames))
	copy(out.ContainerNames, in.ContainerNames)
	out.LastTriggeredImage = in.LastTriggeredImage
	if err := s.Convert(&in.From, &out.From, 0); err != nil {
		return err
	}
	switch in.From.Kind {
	case "ImageStream":
		out.From.Kind = "ImageStreamTag"
		tag := in.Tag
		if len(tag) == 0 {
			tag = imageapi.DefaultImageTag
		}
		out.From.Name = fmt.Sprintf("%s:%s", in.From.Name, tag)
	}
	return nil
}

func convert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in *DeploymentCauseImageTrigger, out *newer.DeploymentCauseImageTrigger, s conversion.Scope) error {
	switch in.From.Kind {
	case "DockerImage":
		ref, err := imageapi.ParseDockerImageReference(in.From.Name)
		if err != nil {
			return err
		}
		out.Tag = ref.Tag
		ref.Tag, ref.ID = "", ""
		out.RepositoryName = ref.Minimal().String()
	}
	return nil
}

func convert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in *newer.DeploymentCauseImageTrigger, out *DeploymentCauseImageTrigger, s conversion.Scope) error {
	if len(in.RepositoryName) != 0 {
		ref, err := imageapi.ParseDockerImageReference(in.RepositoryName)
		if err != nil {
			return err
		}
		ref.Tag = in.Tag
		out.From.Kind = "DockerImage"
		out.From.Name = ref.String()
	}
	return nil
}

func init() {
	err := api.Scheme.AddDefaultingFuncs(
		func(obj *DeploymentTriggerImageChangeParams) {
			if len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
	)
	if err != nil {
		panic(err)
	}

	err = api.Scheme.AddConversionFuncs(
		convert_v1_DeploymentConfig_To_api_DeploymentConfig,
		convert_api_DeploymentConfig_To_v1_DeploymentConfig,

		convert_v1_DeploymentConfigSpec_To_api_DeploymentTemplate,
		convert_v1_DeploymentStrategy_To_api_DeploymentStrategy,
		convert_api_DeploymentStrategy_To_v1_DeploymentStrategy,
		convert_api_DeploymentTemplate_To_v1_DeploymentConfigSpec,

		convert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams,
		convert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams,

		convert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger,
		convert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger,
	)
	if err != nil {
		panic(err)
	}
}
