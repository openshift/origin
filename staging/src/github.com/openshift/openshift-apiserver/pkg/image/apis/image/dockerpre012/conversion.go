package dockerpre012

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/api/image/dockerpre012"
	newer "github.com/openshift/origin/pkg/image/apis/image"
)

// Convert docker client object to internal object, but only when this package is included
func Convert_dockerpre012_ImagePre_012_to_api_DockerImage(in *dockerpre012.ImagePre012, out *newer.DockerImage, s conversion.Scope) error {
	if err := s.Convert(in.Config, &out.Config, conversion.AllowDifferentFieldTypeNames); err != nil {
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
}
func Convert_api_DockerImage_to_dockerpre012_ImagePre_012(in *newer.DockerImage, out *dockerpre012.ImagePre012, s conversion.Scope) error {
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
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddConversionFuncs(
		Convert_dockerpre012_ImagePre_012_to_api_DockerImage,
		Convert_api_DockerImage_to_dockerpre012_ImagePre_012,
	)
}
