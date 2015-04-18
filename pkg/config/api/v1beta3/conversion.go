package v1beta3

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	newer "github.com/openshift/origin/pkg/config/api"
)

func init() {
	api.Scheme.AddConversionFuncs(
		func(in *newer.Config, out *kapi.List, s conversion.Scope) error {
			out.ResourceVersion = in.ListMeta.ResourceVersion
			out.SelfLink = in.ListMeta.SelfLink
			return s.Convert(&in.Items, &out.Items, conversion.DestFromSource)
		},
		func(in *kapi.List, out *newer.Config, s conversion.Scope) error {
			out.ListMeta.ResourceVersion = in.ResourceVersion
			out.ListMeta.SelfLink = in.SelfLink
			return s.Convert(&in.Items, &out.Items, conversion.DestFromSource)
		},
	)
}
