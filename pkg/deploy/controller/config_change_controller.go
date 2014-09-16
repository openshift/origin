package controller

import (
  kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
  cache "github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
  util "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

  deployapi "github.com/openshift/origin/pkg/deploy/api"
  deployutil "github.com/openshift/origin/pkg/deploy/util"

  "github.com/golang/glog"
)

type ConfigChangeController struct {
  DeploymentConfigInterface deploymentConfigInterface
  NextDeploymentConfig      func() *deployapi.DeploymentConfig
  DeploymentStore           cache.Store
}

type deploymentConfigInterface interface {
  GenerateDeploymentConfig(kapi.Context, string) (*deployapi.DeploymentConfig, error)
  UpdateDeploymentConfig(kapi.Context, *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

func (dc *ConfigChangeController) Run() {
  go util.Forever(func() { dc.HandleDeploymentConfig() }, 0)
}

func (dc *ConfigChangeController) HandleDeploymentConfig() error {
  config := dc.NextDeploymentConfig()

  hasChangeTrigger := false
  for _, trigger := range config.Triggers {
    if trigger.Type == deployapi.DeploymentTriggerOnConfigChange {
      hasChangeTrigger = true
      break
    }
  }

  if !hasChangeTrigger {
    return nil
  }

  latestDeploymentId := deployutil.LatestDeploymentIDForConfig(config)
  obj, exists := dc.DeploymentStore.Get(latestDeploymentId)

  if !exists || !deployutil.PodTemplatesEqual(config.Template.ControllerTemplate.PodTemplate,
    obj.(*deployapi.Deployment).ControllerTemplate.PodTemplate) {
    ctx := kapi.WithNamespace(kapi.NewContext(), config.Namespace)
    newConfig, err := dc.DeploymentConfigInterface.GenerateDeploymentConfig(ctx, config.ID)
    if err != nil {
      glog.Errorf("Error generating new version of deploymentConfig %v", config.ID)
      return err
    }

    _, err = dc.DeploymentConfigInterface.UpdateDeploymentConfig(ctx, newConfig)
    if err != nil {
      glog.Errorf("Error updating deploymentConfig %v", config.ID)
      return err
    }
  }

  return nil
}
