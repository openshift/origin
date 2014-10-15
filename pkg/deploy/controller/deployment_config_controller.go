package controller

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// A DeploymentConfigController is responsible for implementing the triggers registered by DeploymentConfigs
type DeploymentConfigController struct {
	DeploymentInterface deploymentInterface

	// Blocks until the next DeploymentConfig is available
	NextDeploymentConfig func() *deployapi.DeploymentConfig
}

type deploymentInterface interface {
	GetDeployment(ctx kapi.Context, id string) (*deployapi.Deployment, error)
	CreateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

// Process deployment config events one at a time
func (c *DeploymentConfigController) Run() {
	go util.Forever(c.HandleDeploymentConfig, 0)
}

func (c *DeploymentConfigController) HandleDeploymentConfig() {
	config := c.NextDeploymentConfig()
	ctx := kapi.WithNamespace(kapi.NewContext(), config.Namespace)
	deploy, err := c.shouldDeploy(ctx, config)
	if err != nil {
		// TODO: better error handling
		glog.Errorf("Error determining whether to redeploy deploymentConfig %v: %#v", config.ID, err)
		return
	}

	if !deploy {
		glog.Infof("Won't deploy from config %s", config.ID)
		return
	}

	err = c.deploy(ctx, config)
	if err != nil {
		glog.Errorf("Error deploying config %s: %v", config.ID, err)
	}
}

func (c *DeploymentConfigController) shouldDeploy(ctx kapi.Context, config *deployapi.DeploymentConfig) (bool, error) {
	if config.LatestVersion == 0 {
		glog.Infof("Shouldn't deploy config %s with LatestVersion=0", config.ID)
		return false, nil
	}

	deployment, err := c.latestDeploymentForConfig(ctx, config)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Infof("Should deploy config %s because there's no latest deployment", config.ID)
			return true, nil
		} else {
			glog.Infof("Shouldn't deploy config %s because of an error looking up latest deployment", config.ID)
			return false, err
		}
	}

	return !deployutil.PodTemplatesEqual(deployment.ControllerTemplate.PodTemplate, config.Template.ControllerTemplate.PodTemplate), nil
}

// TODO: reduce code duplication between trigger and config controllers
func (c *DeploymentConfigController) latestDeploymentForConfig(ctx kapi.Context, config *deployapi.DeploymentConfig) (*deployapi.Deployment, error) {
	latestDeploymentId := deployutil.LatestDeploymentIDForConfig(config)
	deployment, err := c.DeploymentInterface.GetDeployment(ctx, latestDeploymentId)
	if err != nil {
		// TODO: probably some error / race handling to do here
		return nil, err
	}

	return deployment, nil
}

func (c *DeploymentConfigController) deploy(ctx kapi.Context, config *deployapi.DeploymentConfig) error {
	labels := make(map[string]string)
	for k, v := range config.Labels {
		labels[k] = v
	}
	labels[deployapi.DeploymentConfigIDLabel] = config.ID

	deployment := &deployapi.Deployment{
		JSONBase: kapi.JSONBase{
			ID: deployutil.LatestDeploymentIDForConfig(config),
		},
		Labels:             labels,
		Strategy:           config.Template.Strategy,
		ControllerTemplate: config.Template.ControllerTemplate,
	}

	glog.Infof("Creating new deployment from config %s", config.ID)
	_, err := c.DeploymentInterface.CreateDeployment(ctx, deployment)

	return err
}
