package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	newer "github.com/openshift/origin/pkg/build/api"
)

func init() {
	api.Scheme.AddConversionFuncs(
		// Rename STIBuildStrategy.BuildImage to STIBuildStrategy.Image
		func(in *newer.STIBuildStrategy, out *STIBuildStrategy, s conversion.Scope) error {
			out.BuilderImage = in.Image
			out.Image = in.Image
			out.Scripts = in.Scripts
			out.Clean = in.Clean
			return nil
		},
		func(in *STIBuildStrategy, out *newer.STIBuildStrategy, s conversion.Scope) error {
			out.Scripts = in.Scripts
			out.Clean = in.Clean
			if len(in.Image) != 0 {
				out.Image = in.Image
			} else {
				out.Image = in.BuilderImage
			}
			return nil
		},
	)
}
