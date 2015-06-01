package v1beta3

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	mkintp := func(i int) *int64 {
		v := int64(i)
		return &v
	}

	err := api.Scheme.AddDefaultingFuncs(
		func(obj *DeploymentStrategy) {
			if len(obj.Type) == 0 {
				obj.Type = DeploymentStrategyTypeRolling
			}

			if obj.Type == DeploymentStrategyTypeRolling && obj.RollingParams == nil {
				obj.RollingParams = &RollingDeploymentStrategyParams{
					IntervalSeconds:     mkintp(1),
					UpdatePeriodSeconds: mkintp(1),
					TimeoutSeconds:      mkintp(120),
				}
			}
		},
		func(obj *RollingDeploymentStrategyParams) {
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
		func(obj *DeploymentTriggerImageChangeParams) {
			if len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
	)
	if err != nil {
		panic(err)
	}
}
