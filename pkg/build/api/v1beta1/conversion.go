package v1beta1

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
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
			if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
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
			if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
				return err
			}
			return nil
		},
		func(in *newer.STIBuildStrategy, out *STIBuildStrategy, s conversion.Scope) error {
			if in.From != nil {
				switch in.From.Kind {
				case "ImageStreamImage":
					// This will break v1beta1 clients that assume From is always an ImageStream
					// kind, but there's no better alternative here.
					out.From = &kapi.ObjectReference{
						Name:      in.From.Name,
						Namespace: in.From.Namespace,
						Kind:      in.From.Kind,
					}
				case "ImageStreamTag":
					name, tag, ok := imageapi.SplitImageStreamTag(in.From.Name)
					if !ok {
						return fmt.Errorf("ImageStreamTag object references must be in the form <name>:<tag>: %s", in.From.Name)
					}
					out.From = &kapi.ObjectReference{
						Kind:      "ImageStream",
						Namespace: in.From.Namespace,
						Name:      name,
					}
					out.Tag = tag
				case "DockerImage":
					out.Image = in.From.Name
					out.BuilderImage = in.From.Name
				}
			}
			out.Scripts = in.Scripts
			out.Clean = !in.Incremental
			return s.Convert(&in.Env, &out.Env, 0)
		},
		func(in *STIBuildStrategy, out *newer.STIBuildStrategy, s conversion.Scope) error {
			out.Scripts = in.Scripts
			out.Incremental = !in.Clean
			if in.From != nil {
				out.From = &api.ObjectReference{
					Kind:      in.From.Kind,
					Name:      in.From.Name,
					Namespace: in.From.Namespace,
				}
				if len(in.From.Kind) == 0 || in.From.Kind == "ImageStream" || in.From.Kind == "ImageRepository" {
					out.From.Kind = "ImageStreamTag"
					out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, in.Tag)
				}
			}
			if in.Image != "" {
				out.From = &api.ObjectReference{
					Name: in.Image,
					Kind: "DockerImage",
				}
			} else if in.BuilderImage != "" {
				out.From = &api.ObjectReference{
					Name: in.BuilderImage,
					Kind: "DockerImage",
				}
			}
			return s.Convert(&in.Env, &out.Env, 0)
		},
		// Rename DockerBuildStrategy.BaseImage to DockerBuildStrategy.Image
		func(in *newer.DockerBuildStrategy, out *DockerBuildStrategy, s conversion.Scope) error {
			out.NoCache = in.NoCache
			if in.From != nil {
				switch in.From.Kind {
				case "ImageStreamImage":
					// This will break v1beta1 clients that assume From is always an ImageStream
					// kind, but there's no better alternative here.
					out.From = &kapi.ObjectReference{
						Name:      in.From.Name,
						Namespace: in.From.Namespace,
						Kind:      in.From.Kind,
					}
				case "ImageStreamTag":
					name, tag, ok := imageapi.SplitImageStreamTag(in.From.Name)
					if !ok {
						return fmt.Errorf("ImageStreamTag object references must be in the form <name>:<tag>: %s", in.From.Name)
					}
					out.From = &kapi.ObjectReference{
						Kind:      "ImageStream",
						Namespace: in.From.Namespace,
						Name:      name,
					}
					out.Tag = tag
				case "DockerImage":
					out.Image = in.From.Name
					out.BaseImage = in.From.Name
				}
			}
			return nil
		},
		func(in *DockerBuildStrategy, out *newer.DockerBuildStrategy, s conversion.Scope) error {
			out.NoCache = in.NoCache
			if in.From != nil {
				out.From = &api.ObjectReference{
					Kind:      in.From.Kind,
					Name:      in.From.Name,
					Namespace: in.From.Namespace,
				}
				if len(in.From.Kind) == 0 || in.From.Kind == "ImageStream" || in.From.Kind == "ImageRepository" {
					out.From.Kind = "ImageStreamTag"
					out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, in.Tag)
				}
			}
			if in.Image != "" {
				out.From = &api.ObjectReference{
					Name: in.Image,
					Kind: "DockerImage",
				}
			} else if in.BaseImage != "" {
				out.From = &api.ObjectReference{
					Name: in.BaseImage,
					Kind: "DockerImage",
				}
			}
			return nil
		},
		func(in *newer.CustomBuildStrategy, out *CustomBuildStrategy, s conversion.Scope) error {
			if in.From != nil {
				switch in.From.Kind {
				case "ImageStreamImage":
					// This will break v1beta1 clients that assume From is always an ImageStream
					// kind, but there's no better alternative here.
					out.From = &kapi.ObjectReference{
						Name:      in.From.Name,
						Namespace: in.From.Namespace,
						Kind:      in.From.Kind,
					}
				case "ImageStreamTag":
					name, tag, ok := imageapi.SplitImageStreamTag(in.From.Name)
					if !ok {
						return fmt.Errorf("ImageStreamTag object references must be in the form <name>:<tag>: %s", in.From.Name)
					}
					out.From = &kapi.ObjectReference{
						Kind:      "ImageStream",
						Namespace: in.From.Namespace,
						Name:      name,
					}
					out.Tag = tag
				case "DockerImage":
					out.Image = in.From.Name
				}
			}
			out.ExposeDockerSocket = in.ExposeDockerSocket
			return s.Convert(&in.Env, &out.Env, 0)
		},
		func(in *CustomBuildStrategy, out *newer.CustomBuildStrategy, s conversion.Scope) error {
			out.ExposeDockerSocket = in.ExposeDockerSocket
			if in.From != nil {
				out.From = &api.ObjectReference{
					Kind:      in.From.Kind,
					Name:      in.From.Name,
					Namespace: in.From.Namespace,
				}
				if len(in.From.Kind) == 0 || in.From.Kind == "ImageStream" || in.From.Kind == "ImageRepository" {
					out.From.Kind = "ImageStreamTag"
					out.From.Name = imageapi.JoinImageStreamTag(in.From.Name, in.Tag)
				}
			}
			if len(in.Image) != 0 {
				out.From = &api.ObjectReference{
					Name: in.Image,
					Kind: "DockerImage",
				}
			}
			return s.Convert(&in.Env, &out.Env, 0)
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
				ref, err := imageapi.ParseDockerImageReference(in.DockerImageReference)
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
				ref, err := imageapi.ParseDockerImageReference(in.ImageTag)
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
			// Note we lose the From/Image data in the ICT here if it was present,
			// that data no longer has any value.
			out.LastTriggeredImageID = in.LastTriggeredImageID
			return nil
		},
		func(in *ImageChangeTrigger, out *newer.ImageChangeTrigger, s conversion.Scope) error {
			out.LastTriggeredImageID = in.LastTriggeredImageID
			return nil
		})

	// Add field conversion funcs.
	err := api.Scheme.AddFieldLabelConversionFunc("v1beta1", "Build",
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
	err = api.Scheme.AddFieldLabelConversionFunc("v1beta1", "BuildConfig",
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
