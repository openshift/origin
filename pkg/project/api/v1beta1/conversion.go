package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/project/api"
)

func init() {
	err := kapi.Scheme.AddConversionFuncs(
		func(in *newer.Project, out *Project, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.Spec, &out.Spec, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
				return err
			}

			if in.Annotations != nil {
				for key, val := range in.Annotations {
					if key == "displayName" {
						out.DisplayName = val
					}
				}
			}
			return nil
		},
		func(in *Project, out *newer.Project, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.Spec, &out.Spec, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
				return err
			}
			if len(in.DisplayName) > 0 {
				if out.Annotations == nil {
					out.Annotations = map[string]string{}
				}
				out.Annotations["displayName"] = in.DisplayName
			}
			return nil
		},
	)
	if err != nil {
		panic(err)
	}
}
