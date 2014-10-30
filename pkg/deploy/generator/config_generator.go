package generator

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// DeploymentConfigGenerator reconciles a DeploymentConfig with other pieces of deployment-related state
// and produces a DeploymentConfig which represents a potential future DeploymentConfig. If the generated
// state differs from the input state, the LatestVersion field of the output is incremented.
type DeploymentConfigGenerator struct {
	DeploymentInterface       deploymentInterface
	DeploymentConfigInterface deploymentConfigInterface
	ImageRepositoryInterface  imageRepositoryInterface
}

type deploymentInterface interface {
	GetDeployment(ctx kapi.Context, id string) (*deployapi.Deployment, error)
}

type deploymentConfigInterface interface {
	GetDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error)
}

type imageRepositoryInterface interface {
	ListImageRepositories(ctx kapi.Context, labels labels.Selector) (*imageapi.ImageRepositoryList, error)
}

// Generate returns a potential future DeploymentConfig based on the DeploymentConfig specified
// by deploymentConfigID.
func (g *DeploymentConfigGenerator) Generate(ctx kapi.Context, deploymentConfigID string) (*deployapi.DeploymentConfig, error) {
	glog.V(4).Infof("Generating new deployment config from deploymentConfig %v", deploymentConfigID)

	deploymentConfig, err := g.DeploymentConfigInterface.GetDeploymentConfig(ctx, deploymentConfigID)
	if err != nil {
		glog.V(4).Infof("Error getting deploymentConfig for id %v", deploymentConfigID)
		return nil, err
	}

	deploymentID := deployutil.LatestDeploymentIDForConfig(deploymentConfig)

	deployment, err := g.DeploymentInterface.GetDeployment(ctx, deploymentID)
	if err != nil && !errors.IsNotFound(err) {
		glog.V(2).Infof("Error getting deployment: %#v", err)
		return nil, err
	}

	configPodTemplate := deploymentConfig.Template.ControllerTemplate.PodTemplate

	referencedRepoNames := referencedRepoNames(deploymentConfig)
	referencedRepos := imageReposByDockerImageRepo(ctx, g.ImageRepositoryInterface, referencedRepoNames)

	for _, repoName := range referencedRepoNames.List() {
		params := deployutil.ParamsForImageChangeTrigger(deploymentConfig, repoName)
		repo, ok := referencedRepos[params.RepositoryName]
		if !ok {
			return nil, fmt.Errorf("Config references unknown ImageRepository '%s'", params.RepositoryName)
		}

		// TODO: If the tag is missing, what's the correct reaction?
		tag, tagExists := repo.Tags[params.Tag]
		if !tagExists {
			glog.V(4).Infof("No tag %s found for repository %s (potentially invalid DeploymentConfig status)", tag, repoName)
			continue
		}

		newImage := repo.DockerImageRepository + ":" + tag
		updateContainers(&configPodTemplate, util.NewStringSet(params.ContainerNames...), newImage)
	}

	if deployment == nil {
		if deploymentConfig.LatestVersion == 0 {
			// If the latest version is zero, and the generation's being called, bump it.
			deploymentConfig.LatestVersion = 1
			// reset the details of the deployment trigger for this deploymentConfig
			deploymentConfig.Details = nil
		}
	} else if !deployutil.PodTemplatesEqual(configPodTemplate, deployment.ControllerTemplate.PodTemplate) {
		deploymentConfig.LatestVersion += 1
		// reset the details of the deployment trigger for this deploymentConfig
		deploymentConfig.Details = nil
	}

	return deploymentConfig, nil
}

func updateContainers(template *kapi.PodTemplate, containers util.StringSet, newImage string) {
	for i, container := range template.DesiredState.Manifest.Containers {
		if !containers.Has(container.Name) {
			continue
		}

		// TODO: If we grow beyond this single mutation, diffing hashes of
		// a clone of the original config vs the mutation would be more generic.
		if newImage != container.Image {
			template.DesiredState.Manifest.Containers[i].Image = newImage
		}
	}
}

func imageReposByDockerImageRepo(ctx kapi.Context, imageRepoInterface imageRepositoryInterface, filter *util.StringSet) map[string]imageapi.ImageRepository {
	repos := make(map[string]imageapi.ImageRepository)

	imageRepos, err := imageRepoInterface.ListImageRepositories(ctx, labels.Everything())
	if err != nil {
		glog.V(2).Infof("Error listing imageRepositories: %#v", err)
		return repos
	}

	for _, repo := range imageRepos.Items {
		if filter.Has(repo.DockerImageRepository) {
			repos[repo.DockerImageRepository] = repo
		}
	}

	return repos
}

// Returns the image repositories names a config has triggers registered for
func referencedRepoNames(config *deployapi.DeploymentConfig) *util.StringSet {
	repoIDs := &util.StringSet{}

	if config == nil || config.Triggers == nil {
		return repoIDs
	}

	for _, trigger := range config.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnImageChange {
			repoIDs.Insert(trigger.ImageChangeParams.RepositoryName)
		}
	}

	return repoIDs
}
