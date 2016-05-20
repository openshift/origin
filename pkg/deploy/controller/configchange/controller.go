package configchange

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
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

	// Try to decode this deployment config from the encoded annotation found in
	// its latest deployment.
	decoded, err := c.decodeFromLatest(config)
	if err != nil {
		return err
	}

	// If this is the initial deployment, then wait for any images that need to be resolved, otherwise
	// automatically start a new deployment.
	if config.Status.LatestVersion == 0 {
		canTrigger, causes := canTrigger(config, decoded)
		if !canTrigger {
			// If we cannot trigger then we need to wait for the image change controller.
			glog.V(5).Infof("Ignoring deployment config %q; template image needs to be resolved by the image change controller", deployutil.LabelForDeploymentConfig(config))
			return nil
		}
		return c.updateStatus(config, causes)
	}

	// If this is not the initial deployment, check if there is any template difference between
	// this and the decoded deploymentconfig.
	if kapi.Semantic.DeepEqual(config.Spec.Template, decoded.Spec.Template) {
		return nil
	}

	_, causes := canTrigger(config, decoded)
	return c.updateStatus(config, causes)
}

// decodeFromLatest will try to return the decoded version of the current deploymentconfig found
// in the annotations of its latest deployment. If there is no previous deploymentconfig (ie.
// latestVersion == 0), the returned deploymentconfig will be the same.
func (c *DeploymentConfigChangeController) decodeFromLatest(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	if config.Status.LatestVersion == 0 {
		return config, nil
	}

	latestDeploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := c.kClient.ReplicationControllers(config.Namespace).Get(latestDeploymentName)
	if err != nil {
		// If there's no deployment for the latest config, we have no basis of
		// comparison. It's the responsibility of the deployment config controller
		// to make the deployment for the config, so return early.
		return nil, fmt.Errorf("couldn't retrieve deployment for deployment config %q: %v", deployutil.LabelForDeploymentConfig(config), err)
	}

	return c.decodeConfig(deployment)
}

// canTrigger is used by the config change controller to determine if the provided config can
// trigger its initial deployment. The only requirement is set for image change trigger (ICT)
// deployments - all of the ICTs need to have LastTriggedImage set which means that the image
// change controller did its job. The second return argument helps in separating between config
// change and image change causes.
func canTrigger(config, decoded *deployapi.DeploymentConfig) (bool, []deployapi.DeploymentCause) {
	ictCount, resolved := 0, 0
	var causes []deployapi.DeploymentCause

	for _, t := range config.Spec.Triggers {
		if t.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}
		ictCount++

		// If this is the inital deployment then we need to wait for the image change controller
		// to resolve the image inside the pod template.
		lastTriggered := t.ImageChangeParams.LastTriggeredImage
		if len(lastTriggered) == 0 {
			continue
		}
		resolved++

		// We need stronger checks in order to validate that this template
		// change is an image change. Look at the deserialized config's
		// triggers and compare with the present trigger.
		if !triggeredByDifferentImage(*t.ImageChangeParams, *decoded) {
			continue
		}

		causes = append(causes, deployapi.DeploymentCause{
			Type: deployapi.DeploymentTriggerOnImageChange,
			ImageTrigger: &deployapi.DeploymentCauseImageTrigger{
				From: kapi.ObjectReference{
					Name:      t.ImageChangeParams.From.Name,
					Namespace: t.ImageChangeParams.From.Namespace,
					Kind:      "ImageStreamTag",
				},
			},
		})
	}

	if len(causes) == 0 {
		causes = []deployapi.DeploymentCause{{Type: deployapi.DeploymentTriggerOnConfigChange}}
	}

	return ictCount == resolved, causes
}

func triggeredByDifferentImage(ictParams deployapi.DeploymentTriggerImageChangeParams, previous deployapi.DeploymentConfig) bool {
	for _, t := range previous.Spec.Triggers {
		if t.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}

		if t.ImageChangeParams.From.Name != ictParams.From.Name &&
			t.ImageChangeParams.From.Namespace != ictParams.From.Namespace {
			continue
		}

		return t.ImageChangeParams.LastTriggeredImage != ictParams.LastTriggeredImage
	}
	return false
}

func (c *DeploymentConfigChangeController) updateStatus(config *deployapi.DeploymentConfig, causes []deployapi.DeploymentCause) error {
	config.Status.LatestVersion++
	config.Status.Details = new(deployapi.DeploymentDetails)
	config.Status.Details.Causes = causes
	_, err := c.client.DeploymentConfigs(config.Namespace).UpdateStatus(config)
	return err
}
