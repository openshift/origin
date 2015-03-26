package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	newer "github.com/openshift/origin/pkg/build/api"
	image "github.com/openshift/origin/pkg/image/api"
)

func init() {
	api.Scheme.AddConversionFuncs(
		// Move ContextDir in DockerBuildStrategy to BuildSource
		func(in *newer.BuildParameters, out *BuildParameters, s conversion.Scope) error {
			err := s.DefaultConvert(&in.Strategy, &out.Strategy, conversion.IgnoreMissingFields)
			if err != nil {
				return err
			}
			if out.Strategy.Type == DockerBuildStrategyType && in.Strategy.DockerStrategy != nil {
				out.Strategy.DockerStrategy.ContextDir = in.Source.ContextDir
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
			return nil
		},
		func(in *BuildParameters, out *newer.BuildParameters, s conversion.Scope) error {
			err := s.DefaultConvert(&in.Strategy, &out.Strategy, conversion.IgnoreMissingFields)
			if err != nil {
				return err
			}
			if err := s.Convert(&in.Source, &out.Source, 0); err != nil {
				return err
			}
			if in.Strategy.Type == DockerBuildStrategyType && in.Strategy.DockerStrategy != nil {
				out.Source.ContextDir = in.Strategy.DockerStrategy.ContextDir
			}
			if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
				return err
			}
			return nil
		},
		// Rename STIBuildStrategy.BuildImage to STIBuildStrategy.Image
		func(in *newer.STIBuildStrategy, out *STIBuildStrategy, s conversion.Scope) error {
			out.BuilderImage = in.Image
			out.Image = in.Image
			if in.From != nil {
				out.From = &kapi.ObjectReference{
					Name:      in.From.Name,
					Namespace: in.From.Namespace,
					Kind:      "ImageStream",
				}
			}
			out.Tag = in.Tag
			out.Scripts = in.Scripts
			out.Clean = !in.Incremental
			return s.Convert(&in.Env, &out.Env, 0)
		},
		func(in *STIBuildStrategy, out *newer.STIBuildStrategy, s conversion.Scope) error {
			if in.From != nil {
				out.From = &api.ObjectReference{
					Name:      in.From.Name,
					Namespace: in.From.Namespace,
					Kind:      "ImageStream",
				}
			}
			out.Tag = in.Tag
			out.Scripts = in.Scripts
			out.Incremental = !in.Clean
			if len(in.Image) != 0 {
				out.Image = in.Image
			} else {
				out.Image = in.BuilderImage
			}
			return s.Convert(&in.Env, &out.Env, 0)
		},
		// Rename DockerBuildStrategy.BaseImage to DockerBuildStrategy.Image
		func(in *newer.DockerBuildStrategy, out *DockerBuildStrategy, s conversion.Scope) error {
			out.NoCache = in.NoCache
			out.BaseImage = in.Image
			return nil
		},
		func(in *DockerBuildStrategy, out *newer.DockerBuildStrategy, s conversion.Scope) error {
			out.NoCache = in.NoCache
			if len(in.Image) != 0 {
				out.Image = in.Image
			} else {
				out.Image = in.BaseImage
			}
			return nil
		},
		// Deprecate ImageTag and Registry, replace with To / Tag / DockerImageReference
		func(in *newer.BuildOutput, out *BuildOutput, s conversion.Scope) error {
			if err := s.Convert(&in.To, &out.To, 0); err != nil {
				return err
			}
			out.Tag = in.Tag
			out.PushSecretName = in.PushSecretName
			if len(in.DockerImageReference) > 0 {
				out.DockerImageReference = in.DockerImageReference
				ref, err := image.ParseDockerImageReference(in.DockerImageReference)
				if err != nil {
					return err
				}
				out.Registry = ref.Registry
				ref.Registry = ""
				out.ImageTag = ref.String()
			}
			return nil
		},
		func(in *BuildOutput, out *newer.BuildOutput, s conversion.Scope) error {
			if err := s.Convert(&in.To, &out.To, 0); err != nil {
				return err
			}
			out.Tag = in.Tag
			out.PushSecretName = in.PushSecretName
			if len(in.DockerImageReference) > 0 {
				out.DockerImageReference = in.DockerImageReference
				return nil
			}
			if len(in.ImageTag) != 0 {
				ref, err := image.ParseDockerImageReference(in.ImageTag)
				if err != nil {
					return err
				}
				ref.Registry = in.Registry
				out.DockerImageReference = ref.String()
			}
			return nil
		},
		// Rename ImageRepositoryRef to From
		func(in *newer.ImageChangeTrigger, out *ImageChangeTrigger, s conversion.Scope) error {
			if err := s.Convert(&in.From, &out.From, 0); err != nil {
				return err
			}
			if len(in.From.Name) != 0 {
				out.ImageRepositoryRef = &kapi.ObjectReference{}
				if err := s.Convert(&in.From, out.ImageRepositoryRef, conversion.AllowDifferentFieldTypeNames); err != nil {
					return err
				}
			}
			out.Tag = in.Tag
			out.LastTriggeredImageID = in.LastTriggeredImageID
			out.Image = in.Image
			return nil
		},
		func(in *ImageChangeTrigger, out *newer.ImageChangeTrigger, s conversion.Scope) error {
			if in.ImageRepositoryRef != nil {
				if err := s.Convert(in.ImageRepositoryRef, &out.From, conversion.AllowDifferentFieldTypeNames); err != nil {
					return err
				}
			} else {
				if err := s.Convert(&in.From, &out.From, 0); err != nil {
					return err
				}
			}
			out.Tag = in.Tag
			out.LastTriggeredImageID = in.LastTriggeredImageID
			out.Image = in.Image
			return nil
		})
}
