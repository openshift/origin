package configchange

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigChangeController increments the version of a
// DeploymentConfig which has a config change trigger when a pod template
// change is detected.
//
// Use the DeploymentConfigChangeControllerFactory to create this controller.
type DeploymentConfigChangeController struct {
	client  osclient.Interface
	kClient kclient.Interface

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
	if !deployutil.HasChangeTrigger(config) {
		glog.V(5).Infof("Ignoring deployment config %q; no change triggers detected", deployutil.LabelForDeploymentConfig(config))
		return nil
	}

	if config.Status.LatestVersion == 0 {
		_, _, abort, err := c.generateDeployment(config)
		if err != nil {
			if kerrors.IsConflict(err) {
				return fatalError(fmt.Sprintf("deployment config %q updated since retrieval; aborting trigger: %v", deployutil.LabelForDeploymentConfig(config), err))
			}
			glog.V(4).Infof("Couldn't create initial deployment for deployment config %q: %v", deployutil.LabelForDeploymentConfig(config), err)
			return nil
		}
		if !abort {
			glog.V(4).Infof("Created initial deployment for deployment config %q", deployutil.LabelForDeploymentConfig(config))
		}
		return nil
	}

	latestDeploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := c.kClient.ReplicationControllers(config.Namespace).Get(latestDeploymentName)
	if err != nil {
		// If there's no deployment for the latest config, we have no basis of
		// comparison. It's the responsibility of the deployment config controller
		// to make the deployment for the config, so return early.
		if kerrors.IsNotFound(err) {
			glog.V(5).Infof("Ignoring change for deployment config %q; no existing deployment found", deployutil.LabelForDeploymentConfig(config))
			return nil
		}
		return fmt.Errorf("couldn't retrieve deployment for deployment config %q: %v", deployutil.LabelForDeploymentConfig(config), err)
	}

	deployedConfig, err := c.decodeConfig(deployment)
	if err != nil {
		return fatalError(fmt.Sprintf("error decoding deployment config from deployment %q for deployment config %s: %v", deployutil.LabelForDeployment(deployment), deployutil.LabelForDeploymentConfig(config), err))
	}

	// Detect template diffs, and return early if there aren't any changes.
	if kapi.Semantic.DeepEqual(config.Spec.Template, deployedConfig.Spec.Template) {
		glog.V(5).Infof("Ignoring deployment config change for %q (latestVersion=%d); same as deployment %q", deployutil.LabelForDeploymentConfig(config), config.Status.LatestVersion, deployutil.LabelForDeployment(deployment))
		return nil
	}

	// There was a template diff, so generate a new config version.
	fromVersion, toVersion, abort, err := c.generateDeployment(config)
	if err != nil {
		if kerrors.IsConflict(err) {
			return fatalError(fmt.Sprintf("deployment config %q updated since retrieval; aborting trigger: %v", deployutil.LabelForDeploymentConfig(config), err))
		}
		return fmt.Errorf("couldn't generate deployment for deployment config %q: %v", deployutil.LabelForDeploymentConfig(config), err)
	}
	if !abort {
		glog.V(4).Infof("Updated deployment config %q from version %d to %d for existing deployment %s", deployutil.LabelForDeploymentConfig(config), fromVersion, toVersion, deployutil.LabelForDeployment(deployment))
	}
	return nil
}

func (c *DeploymentConfigChangeController) generateDeployment(config *deployapi.DeploymentConfig) (int, int, bool, error) {
	newConfig, err := c.client.DeploymentConfigs(config.Namespace).Generate(config.Name)
	if err != nil {
		return -1, -1, false, err
	}

	// The generator returns a cause only when there is an image change. If the configchange
	// controller detects an image change, it should just quit, otherwise it is racing with
	// the imagechange controller.
	if newConfig.Status.LatestVersion != config.Status.LatestVersion &&
		deployutil.CauseFromAutomaticImageChange(newConfig) {
		return -1, -1, true, nil
	}

	if newConfig.Status.LatestVersion == config.Status.LatestVersion {
		newConfig.Status.LatestVersion++
	}

	// set the trigger details for the new deployment config
	causes := []deployapi.DeploymentCause{
		{
			Type: deployapi.DeploymentTriggerOnConfigChange,
		},
	}
	newConfig.Status.Details = &deployapi.DeploymentDetails{
		Causes: causes,
	}

	// This update is atomic. If it fails because a newer resource was already persisted, that's
	// okay - we can just ignore the update for the old resource and any changes to the more
	// current config will be captured in future events.
	updatedConfig, err := c.client.DeploymentConfigs(config.Namespace).UpdateStatus(newConfig)
	if err != nil {
		return config.Status.LatestVersion, newConfig.Status.LatestVersion, false, err
	}

	return config.Status.LatestVersion, updatedConfig.Status.LatestVersion, false, nil
}
