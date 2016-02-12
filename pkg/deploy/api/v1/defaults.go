package v1

import (
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/intstr"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func defaultTagImagesHookContainerName(hook *LifecycleHook, containerName string) {
	if hook == nil {
		return
	}
	for i := range hook.TagImages {
		if len(hook.TagImages[i].ContainerName) == 0 {
			hook.TagImages[i].ContainerName = containerName
		}
	}
}

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

			// if you only specify a single container, default the TagImages hook to the container name
			if obj.Template != nil && len(obj.Template.Spec.Containers) == 1 {
				containerName := obj.Template.Spec.Containers[0].Name
				if p := obj.Strategy.RecreateParams; p != nil {
					defaultTagImagesHookContainerName(p.Pre, containerName)
					defaultTagImagesHookContainerName(p.Mid, containerName)
					defaultTagImagesHookContainerName(p.Post, containerName)
				}
				if p := obj.Strategy.RollingParams; p != nil {
					defaultTagImagesHookContainerName(p.Pre, containerName)
					defaultTagImagesHookContainerName(p.Post, containerName)
				}
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
