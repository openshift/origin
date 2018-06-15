package v1

import (
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

// Keep this in sync with pkg/api/serialization_test.go#defaultHookContainerName
func defaultHookContainerName(hook *v1.LifecycleHook, containerName string) {
	if hook == nil {
		return
	}
	for i := range hook.TagImages {
		if len(hook.TagImages[i].ContainerName) == 0 {
			hook.TagImages[i].ContainerName = containerName
		}
	}
	if hook.ExecNewPod != nil {
		if len(hook.ExecNewPod.ContainerName) == 0 {
			hook.ExecNewPod.ContainerName = containerName
		}
	}
}

func SetDefaults_DeploymentConfigSpec(obj *v1.DeploymentConfigSpec) {
	if obj.Triggers == nil {
		obj.Triggers = []v1.DeploymentTriggerPolicy{
			{Type: v1.DeploymentTriggerOnConfigChange},
		}
	}
	if len(obj.Selector) == 0 && obj.Template != nil {
		obj.Selector = obj.Template.Labels
	}

	// if you only specify a single container, default the TagImages hook to the container name
	if obj.Template != nil && len(obj.Template.Spec.Containers) == 1 {
		containerName := obj.Template.Spec.Containers[0].Name
		if p := obj.Strategy.RecreateParams; p != nil {
			defaultHookContainerName(p.Pre, containerName)
			defaultHookContainerName(p.Mid, containerName)
			defaultHookContainerName(p.Post, containerName)
		}
		if p := obj.Strategy.RollingParams; p != nil {
			defaultHookContainerName(p.Pre, containerName)
			defaultHookContainerName(p.Post, containerName)
		}
	}
}

func SetDefaults_DeploymentStrategy(obj *v1.DeploymentStrategy) {
	if len(obj.Type) == 0 {
		obj.Type = v1.DeploymentStrategyTypeRolling
	}

	if obj.Type == v1.DeploymentStrategyTypeRolling && obj.RollingParams == nil {
		obj.RollingParams = &v1.RollingDeploymentStrategyParams{
			IntervalSeconds:     mkintp(appsapi.DefaultRollingIntervalSeconds),
			UpdatePeriodSeconds: mkintp(appsapi.DefaultRollingUpdatePeriodSeconds),
			TimeoutSeconds:      mkintp(appsapi.DefaultRollingTimeoutSeconds),
		}
	}
	if obj.Type == v1.DeploymentStrategyTypeRecreate && obj.RecreateParams == nil {
		obj.RecreateParams = &v1.RecreateDeploymentStrategyParams{}
	}

	if obj.ActiveDeadlineSeconds == nil {
		obj.ActiveDeadlineSeconds = mkintp(appsapi.MaxDeploymentDurationSeconds)
	}
}

func SetDefaults_RecreateDeploymentStrategyParams(obj *v1.RecreateDeploymentStrategyParams) {
	if obj.TimeoutSeconds == nil {
		obj.TimeoutSeconds = mkintp(appsapi.DefaultRecreateTimeoutSeconds)
	}
}

func SetDefaults_RollingDeploymentStrategyParams(obj *v1.RollingDeploymentStrategyParams) {
	if obj.IntervalSeconds == nil {
		obj.IntervalSeconds = mkintp(appsapi.DefaultRollingIntervalSeconds)
	}

	if obj.UpdatePeriodSeconds == nil {
		obj.UpdatePeriodSeconds = mkintp(appsapi.DefaultRollingUpdatePeriodSeconds)
	}

	if obj.TimeoutSeconds == nil {
		obj.TimeoutSeconds = mkintp(appsapi.DefaultRollingTimeoutSeconds)
	}

	if obj.MaxUnavailable == nil && obj.MaxSurge == nil {
		maxUnavailable := intstr.FromString("25%")
		obj.MaxUnavailable = &maxUnavailable

		maxSurge := intstr.FromString("25%")
		obj.MaxSurge = &maxSurge
	}

	if obj.MaxUnavailable == nil && obj.MaxSurge != nil &&
		(*obj.MaxSurge == intstr.FromInt(0) || *obj.MaxSurge == intstr.FromString("0%")) {
		maxUnavailable := intstr.FromString("25%")
		obj.MaxUnavailable = &maxUnavailable
	}

	if obj.MaxSurge == nil && obj.MaxUnavailable != nil &&
		(*obj.MaxUnavailable == intstr.FromInt(0) || *obj.MaxUnavailable == intstr.FromString("0%")) {
		maxSurge := intstr.FromString("25%")
		obj.MaxSurge = &maxSurge
	}
}

func SetDefaults_DeploymentConfig(obj *v1.DeploymentConfig) {
	for _, t := range obj.Spec.Triggers {
		if t.ImageChangeParams != nil {
			if len(t.ImageChangeParams.From.Kind) == 0 {
				t.ImageChangeParams.From.Kind = "ImageStreamTag"
			}
			if len(t.ImageChangeParams.From.Namespace) == 0 {
				t.ImageChangeParams.From.Namespace = obj.Namespace
			}
		}
	}
}

func mkintp(i int64) *int64 {
	return &i
}
