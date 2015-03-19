package deployment

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	controller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentControllerFactory can create a DeploymentController that creates
// deployer pods in a configurable way.
type DeploymentControllerFactory struct {
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
	// Codec is used for encoding/decoding.
	Codec runtime.Codec
	// Environment is a set of environment which should be injected into all deployer pod containers.
	Environment []kapi.EnvVar
	// RecreateStrategyImage specifies which Docker image which should implement the Recreate strategy.
	RecreateStrategyImage string
}

// Create creates a DeploymentController.
func (factory *DeploymentControllerFactory) Create() controller.RunnableController {
	deploymentLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	deploymentQueue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentQueue).Run()

	deployController := &DeploymentController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Get(name)
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Update(deployment)
			},
		},
		podClient: &podClientImpl{
			createPodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return factory.KubeClient.Pods(namespace).Create(pod)
			},
			deletePodFunc: func(namespace, name string) error {
				return factory.KubeClient.Pods(namespace).Delete(name)
			},
		},
		makeContainer: func(strategy *deployapi.DeploymentStrategy) (*kapi.Container, error) {
			return factory.makeContainer(strategy)
		},
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, factory.Codec)
		},
	}

	return &controller.RetryController{
		Queue: deploymentQueue,
		RetryManager: controller.NewQueueRetryManager(
			deploymentQueue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, count int) bool {
				if _, isFatal := err.(fatalError); isFatal {
					kutil.HandleError(err)
					return false
				}
				if count > 1 {
					return false
				}
				return true
			},
		),
		Handle: func(obj interface{}) error {
			deployment := obj.(*kapi.ReplicationController)
			return deployController.Handle(deployment)
		},
	}
}

// makeContainer creates containers in the following way:
//
//   1. For the Recreate strategy, use the factory's RecreateStrategyImage as
//      the container image, and the factory's Environment as the container
//      environment.
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
	case deployapi.DeploymentStrategyTypeRecreate:
		// Use the factory-configured image.
		return &kapi.Container{
			Image: factory.RecreateStrategyImage,
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
