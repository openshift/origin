package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	err := api.Scheme.AddConversionFuncs(
		func(in *DeploymentStrategy, out *newer.DeploymentStrategy, s conversion.Scope) error {
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
		},
		func(in *newer.DeploymentStrategy, out *DeploymentStrategy, s conversion.Scope) error {
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
		},
		func(in *DeploymentTriggerImageChangeParams, out *newer.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
			out.Automatic = in.Automatic
			out.ContainerNames = make([]string, len(in.ContainerNames))
			copy(out.ContainerNames, in.ContainerNames)
			out.LastTriggeredImage = in.LastTriggeredImage
			if err := s.Convert(&in.From, &out.From, 0); err != nil {
				return err
			}
			switch in.From.Kind {
			case "ImageStream", "ImageRepository":
				out.From.Kind = "ImageStreamTag"
				out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, in.Tag)
			}
			return nil
		},
		func(in *newer.DeploymentTriggerImageChangeParams, out *DeploymentTriggerImageChangeParams, s conversion.Scope) error {
			out.Automatic = in.Automatic
			out.ContainerNames = make([]string, len(in.ContainerNames))
			copy(out.ContainerNames, in.ContainerNames)
			out.LastTriggeredImage = in.LastTriggeredImage
			if err := s.Convert(&in.From, &out.From, 0); err != nil {
				return err
			}
			switch in.From.Kind {
			case "ImageStream", "ImageRepository":
				out.From.Kind = "ImageStreamTag"
				out.From.Name = imageapi.JoinImageStreamTag(out.From.Name, out.Tag)
			}
			return nil
		},
	)
	if err != nil {
		panic(err)
	}
}
