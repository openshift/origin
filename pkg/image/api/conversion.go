package api

import (
	"github.com/fsouza/go-dockerclient"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

func init() {
	err := kapi.Scheme.AddConversionFuncs(
		// Convert docker client object to internal object
		func(in *docker.Image, out *DockerImage, s conversion.Scope) error {
			if err := s.Convert(in.Config, &out.Config, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			if err := s.Convert(&in.ContainerConfig, &out.ContainerConfig, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			out.ID = in.ID
			out.Parent = in.Parent
			out.Comment = in.Comment
			out.Created = util.NewTime(in.Created)
			out.Container = in.Container
			out.DockerVersion = in.DockerVersion
			out.Author = in.Author
			out.Architecture = in.Architecture
			out.Size = in.Size
			return nil
		},
		func(in *DockerImage, out *docker.Image, s conversion.Scope) error {
			if err := s.Convert(&in.Config, &out.Config, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			if err := s.Convert(&in.ContainerConfig, &out.ContainerConfig, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			out.ID = in.ID
			out.Parent = in.Parent
			out.Comment = in.Comment
			out.Created = in.Created.Time
			out.Container = in.Container
			out.DockerVersion = in.DockerVersion
			out.Author = in.Author
			out.Architecture = in.Architecture
			out.Size = in.Size
			return nil
		},
		func(in *ImageStream, out *ImageRepository, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}

			out.DockerImageRepository = in.Spec.DockerImageRepository

			if in.Spec.Tags != nil && out.Tags == nil {
				out.Tags = make(map[string]string)
			}
			for tag, tagRef := range in.Spec.Tags {
				out.Tags[tag] = tagRef.DockerImageReference
			}

			return s.Convert(&in.Status, &out.Status, 0)
		},
		func(in *ImageRepository, out *ImageStream, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}

			out.Spec.DockerImageRepository = in.DockerImageRepository

			if in.Tags != nil && out.Spec.Tags == nil {
				out.Spec.Tags = make(map[string]TagReference)
			}
			for tag, value := range in.Tags {
				out.Spec.Tags[tag] = TagReference{DockerImageReference: value}
			}
			return s.Convert(&in.Status, &out.Status, 0)
		},
		func(in *ImageStreamStatus, out *ImageRepositoryStatus, s conversion.Scope) error {
			out.DockerImageRepository = in.DockerImageRepository
			out.Tags = in.Tags
			return nil
		},
		func(in *ImageRepositoryStatus, out *ImageStreamStatus, s conversion.Scope) error {
			out.DockerImageRepository = in.DockerImageRepository
			out.Tags = in.Tags
			return nil
		},
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
