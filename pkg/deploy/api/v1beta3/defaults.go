package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
	kutil "k8s.io/kubernetes/pkg/util"

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

			if obj.UpdatePercent == nil {
				// Apply defaults.
				if obj.MaxUnavailable == nil {
					maxUnavailable := kutil.NewIntOrStringFromString("25%")
					obj.MaxUnavailable = &maxUnavailable
				}
				if obj.MaxSurge == nil {
					maxSurge := kutil.NewIntOrStringFromString("25%")
					obj.MaxSurge = &maxSurge
				}
			}
		},
	)
	if err != nil {
		panic(err)
	}
}
