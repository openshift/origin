package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	runtime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	util "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigChangeController watches for changes to DeploymentConfigs and regenerates them only
// when detecting a change to the PodTemplate of a DeploymentConfig containing a ConfigChange trigger.
type DeploymentConfigChangeController struct {
	ChangeStrategy       ChangeStrategy
	NextDeploymentConfig func() *deployapi.DeploymentConfig
	Codec                runtime.Codec
	// Stop is an optional channel that controls when the controller exits
	Stop <-chan struct{}
}

// Run watches for config change events.
func (dc *DeploymentConfigChangeController) Run() {
	go util.Until(func() {
		err := dc.HandleDeploymentConfig(dc.NextDeploymentConfig())
		if err != nil {
			glog.Errorf("%v", err)
		}
	}, 0, dc.Stop)
}

// HandleDeploymentConfig handles the next DeploymentConfig change that happens.
func (dc *DeploymentConfigChangeController) HandleDeploymentConfig(config *deployapi.DeploymentConfig) error {
	hasChangeTrigger := false
	for _, trigger := range config.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnConfigChange {
			hasChangeTrigger = true
			break
		}
	}

	if !hasChangeTrigger {
		glog.V(4).Infof("Ignoring config %s; no change triggers detected", labelFor(config))
		return nil
	}

	if config.LatestVersion == 0 {
		_, _, err := dc.generateDeployment(config)
		if err != nil {
			return fmt.Errorf("couldn't create initial deployment for config %s: %v", labelFor(config), err)
		}
		glog.V(4).Infof("Created initial deployment for config %s", labelFor(config))
		return nil
	}

	latestDeploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := dc.ChangeStrategy.GetDeployment(config.Namespace, latestDeploymentName)
	if err != nil {
		if kerrors.IsNotFound(err) {
			glog.V(4).Info("Ignoring config change for %s; no existing deployment found", labelFor(config))
			return nil
		}
		return fmt.Errorf("couldn't retrieve deployment for %s: %v", labelFor(config), err)
	}

	deployedConfig, err := deployutil.DecodeDeploymentConfig(deployment, dc.Codec)
	if err != nil {
		return fmt.Errorf("error decoding deploymentConfig from deployment %s for config %s: %v", labelForDeployment(deployment), labelFor(config), err)
	}

	if deployutil.PodSpecsEqual(config.Template.ControllerTemplate.Template.Spec, deployedConfig.Template.ControllerTemplate.Template.Spec) {
		glog.V(4).Infof("Ignoring config change for %s (latestVersion=%d); same as deployment %s", labelFor(config), config.LatestVersion, labelForDeployment(deployment))
		return nil
	}

	fromVersion, toVersion, err := dc.generateDeployment(config)
	if err != nil {
		return fmt.Errorf("couldn't generate deployment for config %s: %v", labelFor(config), err)
	}
	glog.V(4).Infof("Updated config %s from version %d to %d for existing deployment %s", labelFor(config), fromVersion, toVersion, labelForDeployment(deployment))
	return nil
}

func (dc *DeploymentConfigChangeController) generateDeployment(config *deployapi.DeploymentConfig) (int, int, error) {
	newConfig, err := dc.ChangeStrategy.GenerateDeploymentConfig(config.Namespace, config.Name)
	if err != nil {
		return config.LatestVersion, 0, fmt.Errorf("Error generating new version of deploymentConfig %s: %v", labelFor(config), err)
	}

	if newConfig.LatestVersion == config.LatestVersion {
		newConfig.LatestVersion++
	}

	// set the trigger details for the new deployment config
	causes := []*deployapi.DeploymentCause{}
	causes = append(causes,
		&deployapi.DeploymentCause{
			Type: deployapi.DeploymentTriggerOnConfigChange,
		})
	newConfig.Details = &deployapi.DeploymentDetails{
		Causes: causes,
	}

	// This update is atomic. If it fails because a newer resource was already persisted, that's
	// okay - we can just ignore the update for the old resource and any changes to the more
	// current config will be captured in future events.
	if _, err = dc.ChangeStrategy.UpdateDeploymentConfig(config.Namespace, newConfig); err != nil {
		return config.LatestVersion, newConfig.LatestVersion, fmt.Errorf("couldn't update deploymentConfig %s: %v", labelFor(config), err)
	}

	return config.LatestVersion, newConfig.LatestVersion, nil
}

// ChangeStrategy knows how to generate and update DeploymentConfigs.
type ChangeStrategy interface {
	GetDeployment(namespace, name string) (*kapi.ReplicationController, error)
	GenerateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

// ChangeStrategyImpl is a pluggable ChangeStrategy.
type ChangeStrategyImpl struct {
	GetDeploymentFunc            func(namespace, name string) (*kapi.ReplicationController, error)
	GenerateDeploymentConfigFunc func(namespace, name string) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentConfigFunc   func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

func (i *ChangeStrategyImpl) GetDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.GetDeploymentFunc(namespace, name)
}

func (i *ChangeStrategyImpl) GenerateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error) {
	return i.GenerateDeploymentConfigFunc(namespace, name)
}

func (i *ChangeStrategyImpl) UpdateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return i.UpdateDeploymentConfigFunc(namespace, config)
}
