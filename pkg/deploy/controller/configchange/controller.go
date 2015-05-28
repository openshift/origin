package configchange

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigChangeController increments the version of a
// DeploymentConfig which has a config change trigger when a pod template
// change is detected.
//
// Use the DeploymentConfigChangeControllerFactory to create this controller.
type DeploymentConfigChangeController struct {
	// changeStrategy knows how to generate and update DeploymentConfigs.
	changeStrategy changeStrategy
	// decodeConfig knows how to decode the deploymentConfig from a deployment's annotations.
	decodeConfig func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error)
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string {
	return fmt.Sprintf("fatal error handling configuration: %s", string(e))
}

// Handle processes change triggers for config.
func (c *DeploymentConfigChangeController) Handle(config *deployapi.DeploymentConfig) error {
	hasChangeTrigger := false
	for _, trigger := range config.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnConfigChange {
			hasChangeTrigger = true
			break
		}
	}

	if !hasChangeTrigger {
		glog.V(4).Infof("Ignoring DeploymentConfig %s/%s; no change triggers detected", config.Namespace, deployutil.LabelForDeploymentConfig(config))
		return nil
	}

	if config.LatestVersion == 0 {
		_, _, err := c.generateDeployment(config)
		if err != nil {
			if kerrors.IsConflict(err) {
				return fatalError(fmt.Sprintf("DeploymentConfig %s/%s updated since retrieval; aborting trigger: %v", config.Namespace, deployutil.LabelForDeploymentConfig(config), err))
			}
			return fmt.Errorf("couldn't create initial Deployment for DeploymentConfig %s/%s: %v", config.Namespace, deployutil.LabelForDeploymentConfig(config), err)
		}
		glog.V(4).Infof("Created initial Deployment for DeploymentConfig %s/%s", config.Namespace, deployutil.LabelForDeploymentConfig(config))
		return nil
	}

	latestDeploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := c.changeStrategy.getDeployment(config.Namespace, latestDeploymentName)
	if err != nil {
		if kerrors.IsNotFound(err) {
			glog.V(2).Infof("Ignoring change for DeploymentConfig %s/%s; no existing Deployment found", config.Namespace, deployutil.LabelForDeploymentConfig(config))
			return nil
		}
		return fmt.Errorf("couldn't retrieve Deployment for DeploymentConfig %s/%s: %v", config.Namespace, deployutil.LabelForDeploymentConfig(config), err)
	}

	deployedConfig, err := c.decodeConfig(deployment)
	if err != nil {
		return fatalError(fmt.Sprintf("error decoding DeploymentConfig from Deployment %s for DeploymentConfig %s/%s: %v", deployutil.LabelForDeployment(deployment), config.Namespace, deployutil.LabelForDeploymentConfig(config), err))
	}

	if deployutil.PodSpecsEqual(config.Template.ControllerTemplate.Template.Spec, deployedConfig.Template.ControllerTemplate.Template.Spec) {
		glog.V(2).Infof("Ignoring DeploymentConfig change for %s (latestVersion=%d); same as Deployment %s", deployutil.LabelForDeploymentConfig(config), config.LatestVersion, deployutil.LabelForDeployment(deployment))
		return nil
	}

	fromVersion, toVersion, err := c.generateDeployment(config)
	if err != nil {
		if kerrors.IsConflict(err) {
			return fatalError(fmt.Sprintf("DeploymentConfig %s/%s updated since retrieval; aborting trigger: %v", config.Namespace, deployutil.LabelForDeploymentConfig(config), err))
		}
		return fmt.Errorf("couldn't generate deployment for DeploymentConfig %s/%s: %v", config.Namespace, deployutil.LabelForDeploymentConfig(config), err)
	}
	glog.V(4).Infof("Updated DeploymentConfig %s/%s from version %d to %d for existing deployment %s", config.Namespace, deployutil.LabelForDeploymentConfig(config), fromVersion, toVersion, deployutil.LabelForDeployment(deployment))
	return nil
}

func (c *DeploymentConfigChangeController) generateDeployment(config *deployapi.DeploymentConfig) (int, int, error) {
	newConfig, err := c.changeStrategy.generateDeploymentConfig(config.Namespace, config.Name)
	if err != nil {
		return config.LatestVersion, 0, err
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
	updatedConfig, err := c.changeStrategy.updateDeploymentConfig(config.Namespace, newConfig)
	if err != nil {
		return config.LatestVersion, newConfig.LatestVersion, err
	}

	return config.LatestVersion, updatedConfig.LatestVersion, nil
}

// changeStrategy knows how to generate and update DeploymentConfigs.
type changeStrategy interface {
	getDeployment(namespace, name string) (*kapi.ReplicationController, error)
	generateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error)
	updateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

// changeStrategyImpl is a pluggable changeStrategy.
type changeStrategyImpl struct {
	getDeploymentFunc            func(namespace, name string) (*kapi.ReplicationController, error)
	generateDeploymentConfigFunc func(namespace, name string) (*deployapi.DeploymentConfig, error)
	updateDeploymentConfigFunc   func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

func (i *changeStrategyImpl) getDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return i.getDeploymentFunc(namespace, name)
}

func (i *changeStrategyImpl) generateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error) {
	return i.generateDeploymentConfigFunc(namespace, name)
}

func (i *changeStrategyImpl) updateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return i.updateDeploymentConfigFunc(namespace, config)
}
