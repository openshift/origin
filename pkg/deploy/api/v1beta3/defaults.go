package v1beta3

import (
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/intstr"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func addDefaultingFuncs(scheme *runtime.Scheme) {
	mkintp := func(i int64) *int64 {
		return &i
	}

	err := scheme.AddDefaultingFuncs(
		func(obj *DeploymentConfigSpec) {
			if obj.Triggers == nil {
				obj.Triggers = []DeploymentTriggerPolicy{
					{Type: DeploymentTriggerOnConfigChange},
				}
			}
			if len(obj.Selector) == 0 && obj.Template != nil {
				obj.Selector = obj.Template.Labels
			}
		},
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
			if obj.Type == DeploymentStrategyTypeRecreate && obj.RecreateParams == nil {
				obj.RecreateParams = &RecreateDeploymentStrategyParams{}
			}
		},
		func(obj *RecreateDeploymentStrategyParams) {
			if obj.TimeoutSeconds == nil {
				obj.TimeoutSeconds = mkintp(deployapi.DefaultRollingTimeoutSeconds)
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
					maxUnavailable := intstr.FromString("25%")
					obj.MaxUnavailable = &maxUnavailable
				}
				if obj.MaxSurge == nil {
					maxSurge := intstr.FromString("25%")
					obj.MaxSurge = &maxSurge
				}
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
