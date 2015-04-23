package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func init() {
	mkintp := func(i int) *int64 {
		v := int64(i)
		return &v
	}

	err := api.Scheme.AddDefaultingFuncs(
		func(obj *deployapi.DeploymentStrategy) {
			if len(obj.Type) == 0 {
				obj.Type = deployapi.DeploymentStrategyTypeRolling
			}

			if obj.Type == deployapi.DeploymentStrategyTypeRolling && obj.RollingParams == nil {
				obj.RollingParams = &deployapi.RollingDeploymentStrategyParams{
					IntervalSeconds:     mkintp(1),
					UpdatePeriodSeconds: mkintp(1),
					TimeoutSeconds:      mkintp(120),
				}
			}
		},
		func(obj *deployapi.RollingDeploymentStrategyParams) {
			if obj.IntervalSeconds == nil {
				obj.IntervalSeconds = mkintp(1)
			}

			if obj.UpdatePeriodSeconds == nil {
				obj.UpdatePeriodSeconds = mkintp(1)
			}

			if obj.TimeoutSeconds == nil {
				obj.TimeoutSeconds = mkintp(120)
			}
		},
	)
	if err != nil {
		panic(err)
	}
}
