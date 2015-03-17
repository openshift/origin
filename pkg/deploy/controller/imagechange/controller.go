package imagechange

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageChangeController increments the version of a DeploymentConfig which has an image
// change trigger when a tag update to a triggered ImageRepository is detected.
//
// Use the ImageChangeControllerFactory to create this controller.
type ImageChangeController struct {
	deploymentConfigClient deploymentConfigClient
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string { return "fatal error handling imageRepository: " + string(e) }

// Handle processes image change triggers associated with imageRepo.
func (c *ImageChangeController) Handle(imageRepo *imageapi.ImageRepository) error {
	configsToGenerate := []*deployapi.DeploymentConfig{}
	firedTriggersForConfig := make(map[string][]deployapi.DeploymentTriggerImageChangeParams)

	configs, err := c.deploymentConfigClient.listDeploymentConfigs()
	if err != nil {
		return fmt.Errorf("couldn't get list of deploymentConfigs while handling imageRepo %s: %v", labelForRepo(imageRepo), err)
	}

	for _, config := range configs {
		glog.V(4).Infof("Detecting changed images for deploymentConfig %s", labelFor(config))

		// Extract relevant triggers for this imageRepo for this config
		triggersForConfig := []deployapi.DeploymentTriggerImageChangeParams{}
		for _, trigger := range config.Triggers {
			if trigger.Type != deployapi.DeploymentTriggerOnImageChange ||
				!trigger.ImageChangeParams.Automatic {
				continue
			}
			if triggerMatchesImage(config, trigger.ImageChangeParams, imageRepo) {
				glog.V(4).Infof("Found matching %s trigger for deploymentConfig %s: %#v", trigger.Type, labelFor(config), trigger.ImageChangeParams)
				triggersForConfig = append(triggersForConfig, *trigger.ImageChangeParams)
			}
		}

		for _, params := range triggersForConfig {
			glog.V(4).Infof("Processing image triggers for deploymentConfig %s", labelFor(config))
			containerNames := util.NewStringSet(params.ContainerNames...)
			for _, container := range config.Template.ControllerTemplate.Template.Spec.Containers {
				if !containerNames.Has(container.Name) {
					continue
				}

				ref, err := imageapi.ParseDockerImageReference(container.Image)
				if err != nil {
					glog.V(4).Infof("Skipping container %s for config %s; container's image is invalid: %v", container.Name, labelFor(config), err)
					continue
				}

				latest, err := imageapi.LatestTaggedImage(imageRepo, params.Tag)
				if err != nil {
					glog.V(4).Infof("Skipping container %s for config %s; %s", container.Name, labelFor(config), err)
					continue
				}

				containerImageID := ref.ID
				if len(containerImageID) == 0 {
					// For v1 images, the container image's tag name is by convention the same as the image ID it references
					containerImageID = ref.Tag
				}
				if latest.Image != containerImageID {
					glog.V(4).Infof("Container %s for config %s: image id changed from %q to %q; regenerating config", container.Name, labelFor(config), containerImageID, latest.Image)
					configsToGenerate = append(configsToGenerate, config)
					firedTriggersForConfig[config.Name] = append(firedTriggersForConfig[config.Name], params)
				}
			}
		}
	}

	anyFailed := false
	for _, config := range configsToGenerate {
		err := c.regenerate(imageRepo, config, firedTriggersForConfig[config.Name])
		if err != nil {
			anyFailed = true
			continue
		}
		glog.V(4).Infof("Updated deploymentConfig %s in response to image change trigger", labelFor(config))
	}

	if anyFailed {
		return fatalError(fmt.Sprintf("couldn't update some deploymentConfigs for trigger on imageRepo %s", labelForRepo(imageRepo)))
	}

	glog.V(4).Infof("Updated all configs for trigger on imageRepo %s", labelForRepo(imageRepo))
	return nil
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
	newConfig, err := c.deploymentConfigClient.generateDeploymentConfig(config.Namespace, config.Name)
	if err != nil {
		return fmt.Errorf("error generating new version of deploymentConfig %s: %v", labelFor(config), err)
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

			latest, err := imageapi.LatestTaggedImage(imageRepo, trigger.Tag)
			if err != nil {
				return fmt.Errorf("error generating new version of deploymentConfig: %s: %s", labelFor(config), err)
			}
			repoName = latest.DockerImageReference
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
	_, err = c.deploymentConfigClient.updateDeploymentConfig(newConfig.Namespace, newConfig)
	if err != nil {
		return err
	}

	return nil
}

func labelForRepo(imageRepo *imageapi.ImageRepository) string {
	return fmt.Sprintf("%s/%s", imageRepo.Namespace, imageRepo.Name)
}

// labelFor builds a string identifier for a DeploymentConfig.
func labelFor(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s/%s:%d", config.Namespace, config.Name, config.LatestVersion)
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
