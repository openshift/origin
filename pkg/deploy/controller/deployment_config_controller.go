package controller

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigController is responsible for creating a Deployment when a DeploymentConfig is
// updated with a new LatestVersion. Any deployment created is correlated to a DeploymentConfig
// by setting the DeploymentConfigLabel on the deployment.
type DeploymentConfigController struct {
	// DeploymentInterface provides access to Deployments.
	DeploymentInterface dccDeploymentInterface

	// NextDeploymentConfig blocks until the next DeploymentConfig is available.
	NextDeploymentConfig func() *deployapi.DeploymentConfig
}

// dccDeploymentInterface is a small private interface for dealing with Deployments.
type dccDeploymentInterface interface {
	GetDeployment(ctx kapi.Context, id string) (*deployapi.Deployment, error)
	CreateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

// Process DeploymentConfig events one at a time.
func (c *DeploymentConfigController) Run() {
	go util.Forever(c.HandleDeploymentConfig, 0)
}

// Process a single DeploymentConfig event.
func (c *DeploymentConfigController) HandleDeploymentConfig() {
	config := c.NextDeploymentConfig()
	ctx := kapi.WithNamespace(kapi.NewContext(), config.Namespace)
	deploy, err := c.shouldDeploy(ctx, config)
	if err != nil {
		// TODO: better error handling
		glog.V(2).Infof("Error determining whether to redeploy deploymentConfig %v: %#v", config.Name, err)
		return
	}

	if !deploy {
		glog.V(4).Infof("Won't deploy from config %s", config.Name)
		return
	}

	err = c.deploy(ctx, config)
	if err != nil {
		glog.V(2).Infof("Error deploying config %s: %v", config.Name, err)
	}
}

// shouldDeploy returns true if the DeploymentConfig should have a new Deployment created.
func (c *DeploymentConfigController) shouldDeploy(ctx kapi.Context, config *deployapi.DeploymentConfig) (bool, error) {
	if config.LatestVersion == 0 {
		glog.V(4).Infof("Shouldn't deploy config %s with LatestVersion=0", config.Name)
		return false, nil
	}

	deployment, err := c.latestDeploymentForConfig(ctx, config)
	if deployment != nil {
		glog.V(4).Infof("Shouldn't deploy because a deployment '%s' already exists for latest config %s", deployment.Name, config.Name)
		return false, nil
	}

	if err != nil {
		if errors.IsNotFound(err) {
			glog.V(4).Infof("Should deploy config %s because there's no latest deployment", config.Name)
			return true, nil
		} else {
			glog.V(4).Infof("Shouldn't deploy config %s because of an error looking up latest deployment", config.Name)
			return false, err
		}
	}

	// TODO: what state would this represent?
	return false, nil
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

// deploy performs the work of actually creating a Deployment from the given DeploymentConfig.
func (c *DeploymentConfigController) deploy(ctx kapi.Context, config *deployapi.DeploymentConfig) error {
	labels := make(map[string]string)
	for k, v := range config.Labels {
		labels[k] = v
	}
	labels[deployapi.DeploymentConfigLabel] = config.Name

	deployment := &deployapi.Deployment{

		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.LatestDeploymentIDForConfig(config),
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation: config.Name,
			},
		},
		Labels:             config.Labels,
		Strategy:           config.Template.Strategy,
		ControllerTemplate: config.Template.ControllerTemplate,
		Details:            config.Details,
	}

	glog.V(4).Infof("Creating new deployment from config %s", config.Name)
	_, err := c.DeploymentInterface.CreateDeployment(ctx, deployment)
	return err
}
