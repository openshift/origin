package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/image/api"
)

func init() {
	err := kapi.Scheme.AddConversionFuncs(
		// The docker metadata must be cast to a version
		func(in *newer.Image, out *Image, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}

			out.DockerImageReference = in.DockerImageReference

			version := in.DockerImageMetadataVersion
			if len(version) == 0 {
				version = "1.0"
			}
			data, err := kapi.Scheme.EncodeToVersion(&in.DockerImageMetadata, version)
			if err != nil {
				return err
			}
			out.DockerImageMetadata.RawJSON = data
			out.DockerImageMetadataVersion = version

			return nil
		},
		func(in *Image, out *newer.Image, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}

			out.DockerImageReference = in.DockerImageReference

			version := in.DockerImageMetadataVersion
			if len(version) == 0 {
				version = "1.0"
			}
			if len(in.DockerImageMetadata.RawJSON) > 0 {
				// TODO: add a way to default the expected kind and version of an object if not set
				obj, err := kapi.Scheme.New(version, "DockerImage")
				if err != nil {
					return err
				}
				if err := kapi.Scheme.DecodeInto(in.DockerImageMetadata.RawJSON, obj); err != nil {
					return err
				}
				if err := s.Convert(obj, &out.DockerImageMetadata, 0); err != nil {
					return err
				}
			}
			out.DockerImageMetadataVersion = version

			return nil
		},
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
