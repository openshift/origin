package controller

import (
	"strings"

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
	configNames := []string{}
	firedTriggersForConfig := make(map[string][]deployapi.DeploymentTriggerImageChangeParams)

	for _, c := range c.DeploymentConfigStore.List() {
		config := c.(*deployapi.DeploymentConfig)
		glog.V(4).Infof("Detecting changed images for deploymentConfig %s", config.Name)

		// Extract relevant triggers for this imageRepo for this config
		triggersForConfig := []deployapi.DeploymentTriggerImageChangeParams{}
		for _, trigger := range config.Triggers {
			if trigger.Type == deployapi.DeploymentTriggerOnImageChange &&
				trigger.ImageChangeParams.Automatic &&
				trigger.ImageChangeParams.RepositoryName == imageRepo.DockerImageRepository {
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
				_, containerImageID := parseImage(container.Image)
				if repoImageID, repoHasTag := imageRepo.Tags[params.Tag]; repoHasTag && repoImageID != containerImageID {
					configNames = append(configNames, config.Name)
					firedTriggersForConfig[config.Name] = append(firedTriggersForConfig[config.Name], params)
				}
			}
		}
	}

	for _, configName := range configNames {
		glog.V(4).Infof("Regenerating deploymentConfig %s", configName)
		err := c.regenerate(imageRepo.Namespace, configName, firedTriggersForConfig[configName])
		if err != nil {
			glog.V(2).Infof("Error regenerating deploymentConfig %v: %v", configName, err)
		}
	}
}

func (c *ImageChangeController) regenerate(namespace, configName string, triggers []deployapi.DeploymentTriggerImageChangeParams) error {
	newConfig, err := c.DeploymentConfigInterface.GenerateDeploymentConfig(namespace, configName)
	if err != nil {
		glog.V(2).Infof("Error generating new version of deploymentConfig %v", configName)
		return err
	}

	// update the deployment config with the trigger that resulted in the new config being generated
	newConfig.Details = generateTriggerDetails(triggers)

	_, err = c.DeploymentConfigInterface.UpdateDeploymentConfig(newConfig.Namespace, newConfig)
	if err != nil {
		glog.V(2).Infof("Error updating deploymentConfig %v", configName)
		return err
	}

	return nil
}

func parseImage(name string) (string, string) {
	index := strings.LastIndex(name, ":")
	if index == -1 {
		return "", ""
	}

	return name[:index], name[index+1:]
}

func generateTriggerDetails(triggers []deployapi.DeploymentTriggerImageChangeParams) *deployapi.DeploymentDetails {
	// Generate the DeploymentCause objects from each DeploymentTriggerImageChangeParams object
	// Using separate structs to ensure flexibility in the future if these structs need to diverge
	causes := []*deployapi.DeploymentCause{}
	for _, trigger := range triggers {
		causes = append(causes,
			&deployapi.DeploymentCause{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageTrigger: &deployapi.DeploymentCauseImageTrigger{
					RepositoryName: trigger.RepositoryName,
					Tag:            trigger.Tag,
				},
			})
	}
	return &deployapi.DeploymentDetails{
		Causes: causes,
	}
}
