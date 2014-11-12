package controller

import (
	"strings"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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
}

type icDeploymentConfigInterface interface {
	UpdateDeploymentConfig(ctx kapi.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	GenerateDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error)
}

// Run processes ImageRepository events one by one.
func (c *ImageChangeController) Run() {
	go util.Forever(c.HandleImageRepo, 0)
}

// HandleImageRepo processes the next ImageRepository event.
func (c *ImageChangeController) HandleImageRepo() {
	imageRepo := c.NextImageRepository()
	configIDs := []string{}
	firedTriggersForConfig := make(map[string][]deployapi.DeploymentTriggerImageChangeParams)

	imageRepoCtx := kapi.WithNamespace(kapi.NewContext(), imageRepo.Namespace)

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
			for _, container := range config.Template.ControllerTemplate.PodTemplate.DesiredState.Manifest.Containers {
				if !containerNames.Has(container.Name) {
					continue
				}

				_, tag := parseImage(container.Image)
				if tag != imageRepo.Tags[params.Tag] {
					configIDs = append(configIDs, config.Name)
					firedTriggersForConfig[config.Name] = append(firedTriggersForConfig[config.Name], params)
				}
			}
		}
	}

	for _, configID := range configIDs {
		glog.V(4).Infof("Regenerating deploymentConfig %s", configID)
		err := c.regenerate(imageRepoCtx, configID, firedTriggersForConfig[configID])
		if err != nil {
			glog.V(2).Infof("Error regenerating deploymentConfig %v: %v", configID, err)
		}
	}
}

func (c *ImageChangeController) regenerate(ctx kapi.Context, configID string, triggers []deployapi.DeploymentTriggerImageChangeParams) error {
	newConfig, err := c.DeploymentConfigInterface.GenerateDeploymentConfig(ctx, configID)
	if err != nil {
		glog.V(2).Infof("Error generating new version of deploymentConfig %v", configID)
		return err
	}

	// update the deployment config with the trigger that resulted in the new config being generated
	newConfig.Details = generateTriggerDetails(triggers)

	ctx = kapi.WithNamespace(ctx, newConfig.Namespace)
	_, err = c.DeploymentConfigInterface.UpdateDeploymentConfig(ctx, newConfig)
	if err != nil {
		glog.V(2).Infof("Error updating deploymentConfig %v", configID)
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
