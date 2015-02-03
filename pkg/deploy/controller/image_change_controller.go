package controller

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageChangeController watches for changes to ImageRepositories and regenerates
// DeploymentConfigs when a new version of a tag referenced by a DeploymentConfig
// is available.
type ImageChangeController struct {
	DeploymentConfigInterface icDeploymentConfigInterface
	NextImageRepository       func() *imageapi.ImageRepository
	DeploymentConfigStore     cache.Store
	// Stop is an optional channel that controls when the controller exits
	Stop <-chan struct{}
}

type icDeploymentConfigInterface interface {
	UpdateDeploymentConfig(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	GenerateDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error)
}

// Run processes ImageRepository events one by one.
func (c *ImageChangeController) Run() {
	go util.Until(c.HandleImageRepo, 0, c.Stop)
}

// HandleImageRepo processes the next ImageRepository event.
func (c *ImageChangeController) HandleImageRepo() {
	imageRepo := c.NextImageRepository()
	configsToGenerate := []*deployapi.DeploymentConfig{}
	firedTriggersForConfig := make(map[string][]deployapi.DeploymentTriggerImageChangeParams)

	for _, c := range c.DeploymentConfigStore.List() {
		config := c.(*deployapi.DeploymentConfig)
		glog.V(4).Infof("Detecting changed images for deploymentConfig %s", config.Name)

		// Extract relevant triggers for this imageRepo for this config
		triggersForConfig := []deployapi.DeploymentTriggerImageChangeParams{}
		for _, trigger := range config.Triggers {
			if trigger.Type != deployapi.DeploymentTriggerOnImageChange ||
				!trigger.ImageChangeParams.Automatic {
				continue
			}
			if triggerMatchesImage(config, trigger.ImageChangeParams, imageRepo) {
				glog.V(4).Infof("Found matching %s trigger for deploymentConfig %s: %#v", trigger.Type, config.Name, trigger.ImageChangeParams)
				triggersForConfig = append(triggersForConfig, *trigger.ImageChangeParams)
			}
		}

		for _, params := range triggersForConfig {
			glog.V(4).Infof("Processing image triggers for deploymentConfig %s", config.Name)
			containerNames := util.NewStringSet(params.ContainerNames...)
			for _, container := range config.Template.ControllerTemplate.Template.Spec.Containers {
				if !containerNames.Has(container.Name) {
					continue
				}

				// The container image's tag name is by convention the same as the image ID it references
				_, _, _, containerImageID, err := imageapi.SplitDockerPullSpec(container.Image)
				if err != nil {
					glog.V(4).Infof("Skipping container %s; container's image is invalid: %v", container.Name, err)
					continue
				}

				if repoImageID, repoHasTag := imageRepo.Tags[params.Tag]; repoHasTag && repoImageID != containerImageID {
					configsToGenerate = append(configsToGenerate, config)
					firedTriggersForConfig[config.Name] = append(firedTriggersForConfig[config.Name], params)
				}
			}
		}
	}

	for _, config := range configsToGenerate {
		glog.V(4).Infof("Regenerating deploymentConfig %s/%s", config.Namespace, config.Name)
		err := c.regenerate(imageRepo, config, firedTriggersForConfig[config.Name])
		if err != nil {
			glog.V(2).Infof("Error regenerating deploymentConfig %s/%s: %v", config.Namespace, config.Name, err)
		}
	}
}

// triggerMatchesImages decides whether a given trigger for config matches the provided image repo.
// When matching:
// - The trigger From field is preferred over the deprecated RepositoryName field.
// - The namespace of the trigger is preferred over the config's namespace.
func triggerMatchesImage(config *deployapi.DeploymentConfig, trigger *deployapi.DeploymentTriggerImageChangeParams, repo *imageapi.ImageRepository) bool {
	if len(trigger.From.Name) > 0 {
		namespace := trigger.From.Namespace
		if len(namespace) == 0 {
			namespace = config.Namespace
		}

		return repo.Namespace == namespace && repo.Name == trigger.From.Name
	}

	// This is an invalid state (as one of From.Name or RepositoryName is required), but
	// account for it anyway.
	if len(trigger.RepositoryName) == 0 {
		return false
	}

	// If the repo's repository information isn't yet available, we can't assume it'll match.
	return len(repo.Status.DockerImageRepository) > 0 &&
		trigger.RepositoryName == repo.Status.DockerImageRepository
}

func (c *ImageChangeController) regenerate(imageRepo *imageapi.ImageRepository, config *deployapi.DeploymentConfig, triggers []deployapi.DeploymentTriggerImageChangeParams) error {
	// Get a regenerated config which includes the new image repo references
	newConfig, err := c.DeploymentConfigInterface.GenerateDeploymentConfig(config.Namespace, config.Name)
	if err != nil {
		glog.V(2).Infof("Error generating new version of deploymentConfig %v", config.Name)
		return err
	}

	// Update the deployment config with the trigger that resulted in the new config
	causes := []*deployapi.DeploymentCause{}
	for _, trigger := range triggers {
		repoName := trigger.RepositoryName

		if len(repoName) == 0 {
			if len(imageRepo.Status.DockerImageRepository) == 0 {
				// If the trigger relies on a image repo reference, and we don't know what docker repo
				// it points at, we can't build a cause for the reference yet.
				continue
			}

			id, ok := imageRepo.Tags[trigger.Tag]
			if !ok {
				// TODO: not really sure what to do here
			}
			repoName = fmt.Sprintf("%s:%s", imageRepo.Status.DockerImageRepository, id)
		}

		causes = append(causes,
			&deployapi.DeploymentCause{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageTrigger: &deployapi.DeploymentCauseImageTrigger{
					RepositoryName: repoName,
					Tag:            trigger.Tag,
				},
			})
	}
	newConfig.Details = &deployapi.DeploymentDetails{
		Causes: causes,
	}

	// Persist the new config
	_, err = c.DeploymentConfigInterface.UpdateDeploymentConfig(newConfig.Namespace, newConfig)
	if err != nil {
		glog.V(2).Infof("Error updating deploymentConfig %v", newConfig.Name)
		return err
	}

	return nil
}
