package deployment

import (
	"fmt"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	controller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentControllerFactory can create a DeploymentController that creates
// deployer pods in a configurable way.
type DeploymentControllerFactory struct {
	// KubeClient is a Kubernetes client.
	KubeClient *kclient.Client
	// Codec is used for encoding/decoding.
	Codec runtime.Codec
	// ServiceAccount is the service account name to run deployer pods as
	ServiceAccount string
	// Environment is a set of environment which should be injected into all deployer pod containers.
	Environment []kapi.EnvVar
	// DeployerImage specifies which Docker image can support the default strategies.
	DeployerImage string
}

// Create creates a DeploymentController.
func (factory *DeploymentControllerFactory) Create() controller.RunnableController {
	deploymentLW := cache.NewListWatchFromClient(factory.KubeClient, "replicationcontrollers", kapi.NamespaceAll, fields.Everything())
	deploymentQueue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentQueue, 2*time.Minute).Run()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(factory.KubeClient.Events(""))

	deployController := &DeploymentController{
		serviceAccount:   factory.ServiceAccount,
		deploymentClient: factory.KubeClient,
		podClient:        factory.KubeClient,
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return factory.makeContainer(strategy)
		},
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, factory.Codec)
		},
		recorder: eventBroadcaster.NewRecorder(kapi.EventSource{Component: "deployer"}),
	}

	return &controller.RetryController{
		Queue: deploymentQueue,
		RetryManager: controller.NewQueueRetryManager(
			deploymentQueue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				if _, isFatal := err.(fatalError); isFatal {
					kutil.HandleError(err)
					return false
				}
				if retries.Count > 1 {
					return false
				}
				return true
			},
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			deployment := obj.(*kapi.ReplicationController)
			return deployController.Handle(deployment)
		},
	}
}

// makeContainer creates containers in the following way:
//
//   1. For the Recreate and Rolling strategies, strategy, use the factory's
//      DeployerImage as the container image, and the factory's Environment
//      as the container environment.
//   2. For all Custom strategy, use the strategy's image for the container
//      image, and use the combination of the factory's Environment and the
//      strategy's environment as the container environment.
//
// An error is returned if the deployment strategy type is not supported.
func (factory *DeploymentControllerFactory) makeContainer(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
	// Set default environment values
	environment := []kapi.EnvVar{}
	for _, env := range factory.Environment {
		environment = append(environment, env)
	}

	// Every strategy type should be handled here.
	switch strategy.Type {
	case deployapi.DeploymentStrategyTypeRecreate, deployapi.DeploymentStrategyTypeRolling:
		// Use the factory-configured image.
		return &kapi.Container{
			Image: factory.DeployerImage,
			Env:   environment,
		}, nil
	case deployapi.DeploymentStrategyTypeCustom:
		// Use user-defined values from the strategy input.
		for _, env := range strategy.CustomParams.Environment {
			environment = append(environment, env)
		}
		return &kapi.Container{
			Image: strategy.CustomParams.Image,
			Env:   environment,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported deployment strategy type: %s", strategy.Type)
	}
}
