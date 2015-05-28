package imagechange

import (
	"fmt"

	"github.com/golang/glog"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageChangeController increments the version of a DeploymentConfig which has an image
// change trigger when a tag update to a triggered ImageStream is detected.
//
// Use the ImageChangeControllerFactory to create this controller.
type ImageChangeController struct {
	deploymentConfigClient deploymentConfigClient
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string {
	return fmt.Sprintf("fatal error handling ImageStream: %s", string(e))
}

// Handle processes image change triggers associated with imageRepo.
func (c *ImageChangeController) Handle(imageRepo *imageapi.ImageStream) error {
	configs, err := c.deploymentConfigClient.listDeploymentConfigs()
	if err != nil {
		return fmt.Errorf("couldn't get list of DeploymentConfig while handling ImageStream %s/%s: %v", imageRepo.Namespace, labelForRepo(imageRepo), err)
	}

	// Find any configs which should be updated based on the new image state
	configsToUpdate := map[string]*deployapi.DeploymentConfig{}
	for _, config := range configs {
		glog.V(4).Infof("Detecting changed images for DeploymentConfig %s/%s", config.Namespace, deployutil.LabelForDeploymentConfig(config))

		for _, trigger := range config.Triggers {
			params := trigger.ImageChangeParams

			// Only automatic image change triggers should fire
			if trigger.Type != deployapi.DeploymentTriggerOnImageChange || !params.Automatic {
				continue
			}

			// Check if the image repo matches the trigger
			if !triggerMatchesImage(config, params, imageRepo) {
				continue
			}

			// Find the latest tag event for the trigger tag
			latestEvent := imageapi.LatestTaggedImage(imageRepo, params.Tag)
			if latestEvent == nil {
				glog.V(2).Infof("Couldn't find latest tag event for tag %s in ImageStream %s/%s", params.Tag, imageRepo.Namespace, labelForRepo(imageRepo))
				continue
			}

			// Ensure a change occured
			if len(latestEvent.DockerImageReference) > 0 &&
				latestEvent.DockerImageReference != params.LastTriggeredImage {
				// Mark the config for regeneration
				configsToUpdate[config.Name] = config
			}
		}
	}

	// Attempt to regenerate all configs which may contain image updates
	anyFailed := false
	for _, config := range configsToUpdate {
		err := c.regenerate(config)
		if err != nil {
			anyFailed = true
			glog.V(2).Infof("Couldn't regenerate DeploymentConfig %s/%s: %s", config.Namespace, deployutil.LabelForDeploymentConfig(config), err)
			continue
		}

		glog.V(4).Infof("Regenerated DeploymentConfig %s/%s in response to image change trigger", config.Namespace, deployutil.LabelForDeploymentConfig(config))
	}

	if anyFailed {
		return fatalError(fmt.Sprintf("couldn't update some DeploymentConfig for trigger on ImageStream %s/%s", imageRepo.Namespace, labelForRepo(imageRepo)))
	}

	glog.V(4).Infof("Updated all DeploymentConfigs for trigger on ImageStream %s/%s", imageRepo.Namespace, labelForRepo(imageRepo))
	return nil
}

// triggerMatchesImages decides whether a given trigger for config matches the provided image repo.
// When matching:
// - The trigger From field is preferred over the deprecated RepositoryName field.
// - The namespace of the trigger is preferred over the config's namespace.
func triggerMatchesImage(config *deployapi.DeploymentConfig, params *deployapi.DeploymentTriggerImageChangeParams, repo *imageapi.ImageStream) bool {
	if len(params.From.Name) > 0 {
		namespace := params.From.Namespace
		if len(namespace) == 0 {
			namespace = config.Namespace
		}

		return repo.Namespace == namespace && repo.Name == params.From.Name
	}

	// This is an invalid state (as one of From.Name or RepositoryName is required), but
	// account for it anyway.
	if len(params.RepositoryName) == 0 {
		return false
	}

	// If the repo's repository information isn't yet available, we can't assume it'll match.
	return len(repo.Status.DockerImageRepository) > 0 &&
		params.RepositoryName == repo.Status.DockerImageRepository
}

// regenerate calls the generator to get a new config. If the newly generated
// config's version is newer, update the old config to be the new config.
// Otherwise do nothing.
func (c *ImageChangeController) regenerate(config *deployapi.DeploymentConfig) error {
	// Get a regenerated config which includes the new image repo references
	newConfig, err := c.deploymentConfigClient.generateDeploymentConfig(config.Namespace, config.Name)
	if err != nil {
		return fmt.Errorf("error generating new version of DeploymentConfig %s/%s: %v", config.Namespace, deployutil.LabelForDeploymentConfig(config), err)
	}

	// No update occured
	if config.LatestVersion == newConfig.LatestVersion {
		glog.V(4).Infof("No version difference for generated DeploymentConfig %s/%s", config.Namespace, deployutil.LabelForDeploymentConfig(config))
		return nil
	}

	// Persist the new config
	_, err = c.deploymentConfigClient.updateDeploymentConfig(newConfig.Namespace, newConfig)
	if err != nil {
		return err
	}

	glog.Infof("Regenerated DeploymentConfig %s/%s for image updates", config.Namespace, deployutil.LabelForDeploymentConfig(config))
	return nil
}

func labelForRepo(imageRepo *imageapi.ImageStream) string {
	return fmt.Sprintf("%s/%s", imageRepo.Namespace, imageRepo.Name)
}

// ImageChangeControllerDeploymentConfigClient abstracts access to DeploymentConfigs.
type deploymentConfigClient interface {
	listDeploymentConfigs() ([]*deployapi.DeploymentConfig, error)
	updateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	generateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error)
}

// ImageChangeControllerDeploymentConfigClientImpl is a pluggable ChangeStrategy.
type deploymentConfigClientImpl struct {
	listDeploymentConfigsFunc    func() ([]*deployapi.DeploymentConfig, error)
	generateDeploymentConfigFunc func(namespace, name string) (*deployapi.DeploymentConfig, error)
	updateDeploymentConfigFunc   func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

func (i *deploymentConfigClientImpl) listDeploymentConfigs() ([]*deployapi.DeploymentConfig, error) {
	return i.listDeploymentConfigsFunc()
}

func (i *deploymentConfigClientImpl) generateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error) {
	return i.generateDeploymentConfigFunc(namespace, name)
}

func (i *deploymentConfigClientImpl) updateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return i.updateDeploymentConfigFunc(namespace, config)
}
