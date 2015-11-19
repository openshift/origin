package imagechange

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	controller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageChangeControllerFactory can create an ImageChangeController which
// watches all ImageStream changes.
type ImageChangeControllerFactory struct {
	// Client is an OpenShift client.
	Client osclient.Interface
}

// Create creates an ImageChangeController.
func (factory *ImageChangeControllerFactory) Create() controller.RunnableController {
	imageStreamLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.Client.ImageStreams(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.Client.ImageStreams(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(imageStreamLW, &imageapi.ImageStream{}, queue, 2*time.Minute).Run()

	deploymentConfigLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentConfigLW, &deployapi.DeploymentConfig{}, store, 2*time.Minute).Run()

	changeController := &ImageChangeController{
		deploymentConfigClient: &deploymentConfigClientImpl{
			listDeploymentConfigsFunc: func() ([]*deployapi.DeploymentConfig, error) {
				configs := []*deployapi.DeploymentConfig{}
				objs := store.List()
				for _, obj := range objs {
					configs = append(configs, obj.(*deployapi.DeploymentConfig))
				}
				return configs, nil
			},
			generateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return factory.Client.DeploymentConfigs(namespace).Generate(name)
			},
			updateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				return factory.Client.DeploymentConfigs(namespace).Update(config)
			},
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
			repo := obj.(*imageapi.ImageStream)
			return changeController.Handle(repo)
		},
	}
}
