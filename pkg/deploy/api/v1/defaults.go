package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func init() {
	mkintp := func(i int64) *int64 {
		return &i
	}

	err := api.Scheme.AddDefaultingFuncs(
		func(obj *DeploymentStrategy) {
			if len(obj.Type) == 0 {
				obj.Type = DeploymentStrategyTypeRolling
			}

			if obj.Type == DeploymentStrategyTypeRolling && obj.RollingParams == nil {
				obj.RollingParams = &RollingDeploymentStrategyParams{
					IntervalSeconds:     mkintp(deployapi.DefaultRollingIntervalSeconds),
					UpdatePeriodSeconds: mkintp(deployapi.DefaultRollingUpdatePeriodSeconds),
					TimeoutSeconds:      mkintp(deployapi.DefaultRollingTimeoutSeconds),
				}
			}
		},
		func(obj *RollingDeploymentStrategyParams) {
			if obj.IntervalSeconds == nil {
				obj.IntervalSeconds = mkintp(deployapi.DefaultRollingIntervalSeconds)
			}

			if obj.UpdatePeriodSeconds == nil {
				obj.UpdatePeriodSeconds = mkintp(deployapi.DefaultRollingUpdatePeriodSeconds)
			}

			if obj.TimeoutSeconds == nil {
				obj.TimeoutSeconds = mkintp(deployapi.DefaultRollingTimeoutSeconds)
			}
		},
	)
	if err != nil {
		panic(err)
	}
}
