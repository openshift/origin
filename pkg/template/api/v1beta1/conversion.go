package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/template/api"
)

func init() {
	err := api.Scheme.AddConversionFuncs(
		// TypeMeta must be split into two objects
		func(in *newer.Template, out *Template, s conversion.Scope) error {
			if err := s.Convert(&in.Objects, &out.Items, 0); err != nil {
				return err
			}
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *Template, out *newer.Template, s conversion.Scope) error {
			if err := s.Convert(&in.Items, &out.Objects, 0); err != nil {
				return err
			}
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
	)
	if err != nil {
		panic(err)
	}
}
