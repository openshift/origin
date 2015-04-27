package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/deploy/api"
)

func init() {
	err := api.Scheme.AddConversionFuncs(
		func(in *DeploymentStrategy, out *newer.DeploymentStrategy, s conversion.Scope) error {
			if err := s.Convert(&in.Type, &out.Type, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.CustomParams, &out.CustomParams, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.RecreateParams, &out.RecreateParams, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
				return err
			}
			return nil
		},
		func(in *newer.DeploymentStrategy, out *DeploymentStrategy, s conversion.Scope) error {
			if err := s.Convert(&in.Type, &out.Type, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.CustomParams, &out.CustomParams, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.RecreateParams, &out.RecreateParams, 0); err != nil {
				return err
			}
			if err := s.Convert(&in.Resources, &out.Resources, 0); err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		panic(err)
	}
}
