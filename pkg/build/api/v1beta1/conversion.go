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
		// Rename STIBuildStrategy.BuildImage to STIBuildStrategy.Image
		func(in *newer.STIBuildStrategy, out *STIBuildStrategy, s conversion.Scope) error {
			out.BuilderImage = in.Image
			out.Image = in.Image
			out.Scripts = in.Scripts
			out.Clean = in.Clean
			s.Convert(&in.Env, &out.Env, 0)
			return nil
		},
		func(in *STIBuildStrategy, out *newer.STIBuildStrategy, s conversion.Scope) error {
			out.Scripts = in.Scripts
			out.Clean = in.Clean
			s.Convert(&in.Env, &out.Env, 0)
			if len(in.Image) != 0 {
				out.Image = in.Image
			} else {
				out.Image = in.BuilderImage
			}
			return nil
		},
		// Deprecate ImageTag and Registry, replace with To / Tag / DockerImageReference
		func(in *newer.BuildOutput, out *BuildOutput, s conversion.Scope) error {
			if err := s.Convert(&in.To, &out.To, 0); err != nil {
				return err
			}
			out.Tag = in.Tag
			if len(in.DockerImageReference) > 0 {
				out.DockerImageReference = in.DockerImageReference
				registry, namespace, name, tag, _ := image.SplitDockerPullSpec(in.DockerImageReference)
				out.Registry = registry
				out.ImageTag = image.JoinDockerPullSpec("", namespace, name, tag)
			}
			return nil
		},
		func(in *BuildOutput, out *newer.BuildOutput, s conversion.Scope) error {
			if err := s.Convert(&in.To, &out.To, 0); err != nil {
				return err
			}
			out.Tag = in.Tag
			if len(in.DockerImageReference) > 0 {
				out.DockerImageReference = in.DockerImageReference
				return nil
			}
			if len(in.ImageTag) != 0 {
				_, namespace, name, tag, err := image.SplitDockerPullSpec(in.ImageTag)
				if err != nil {
					return err
				}
				out.DockerImageReference = image.JoinDockerPullSpec(in.Registry, namespace, name, tag)
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
