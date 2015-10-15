package configchange

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	controller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigChangeControllerFactory can create a
// DeploymentConfigChangeController that watches all DeploymentConfigs.
type DeploymentConfigChangeControllerFactory struct {
	// Client is an OpenShift client.
	Client osclient.Interface
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
	// Codec is used for encoding/decoding.
	Codec runtime.Codec
}

// Create creates a DeploymentConfigChangeController.
func (factory *DeploymentConfigChangeControllerFactory) Create() controller.RunnableController {
	deploymentConfigLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentConfigLW, &deployapi.DeploymentConfig{}, queue, 2*time.Minute).Run()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(factory.KubeClient.Events(""))

	changeController := &DeploymentConfigChangeController{
		changeStrategy: &changeStrategyImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Get(name)
			},
			generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return factory.Client.DeploymentConfigs(namespace).Generate(name)
			},
			updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				return factory.Client.DeploymentConfigs(namespace).Update(config)
			},
		},
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, factory.Codec)
		},
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				kutil.HandleError(err)
				if _, isFatal := err.(fatalError); isFatal {
					return false
				}
				if retries.Count > 0 {
					return false
				}
				return true
			},
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			config := obj.(*deployapi.DeploymentConfig)
			return changeController.Handle(config)
		},
	}
}
