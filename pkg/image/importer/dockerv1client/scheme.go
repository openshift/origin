package dockerv1client

import (
	docker "github.com/fsouza/go-dockerclient"
	imagev1 "github.com/openshift/api/image/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

var (
	DockerClientScheme = runtime.NewScheme()
)

// this is the only entrypoint which deals in github.com/fsouza/go-dockerclient.Image and expects to use our conversion capability to coerce an external
// type into an api type.  Localize the crazy here.
func init() {
	utilruntime.Must(imagev1.Install(DockerClientScheme))
	utilruntime.Must(imageapi.Install(DockerClientScheme))
	utilruntime.Must(DockerClientScheme.AddConversionFuncs(
		// Convert docker client object to internal object
		func(in *docker.Image, out *imageapi.DockerImage, s conversion.Scope) error {
			if err := s.Convert(&in.Config, &out.Config, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			if err := s.Convert(&in.ContainerConfig, &out.ContainerConfig, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			out.ID = in.ID
			out.Parent = in.Parent
			out.Comment = in.Comment
			out.Created = metav1.NewTime(in.Created)
			out.Container = in.Container
			out.DockerVersion = in.DockerVersion
			out.Author = in.Author
			out.Architecture = in.Architecture
			out.Size = in.Size
			return nil
		},
		func(in *imageapi.DockerImage, out *docker.Image, s conversion.Scope) error {
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
	))
}
