package deploymentconfig

import (
	"time"

	"github.com/golang/glog"
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

// DeploymentConfigControllerFactory can create a DeploymentConfigController which obtains
// DeploymentConfigs from a queue populated from a watch of all DeploymentConfigs.
type DeploymentConfigControllerFactory struct {
	// Client is an OpenShift client.
	Client osclient.Interface
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
	// Codec is used to encode/decode.
	Codec runtime.Codec
}

// Create creates a DeploymentConfigController.
func (factory *DeploymentConfigControllerFactory) Create() controller.RunnableController {
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

	configController := &DeploymentConfigController{
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Create(deployment)
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				sel := deployutil.ConfigSelector(configName)
				list, err := factory.KubeClient.ReplicationControllers(namespace).List(sel)
				if err != nil {
					return nil, err
				}
				return list, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Update(deployment)
			},
		},
		osClient: factory.Client,
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, factory.Codec)
		},
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				config := obj.(*deployapi.DeploymentConfig)
				// no retries for a fatal error
				if _, isFatal := err.(fatalError); isFatal {
					glog.V(4).Infof("Will not retry fatal error for deploymentConfig %s/%s: %v", config.Namespace, config.Name, err)
					kutil.HandleError(err)
					return false
				}
				// infinite retries for a transient error
				if _, isTransient := err.(transientError); isTransient {
					glog.V(4).Infof("Retrying deploymentConfig %s/%s with error: %v", config.Namespace, config.Name, err)
					return true
				}
				kutil.HandleError(err)
				// no retries for anything else
				if retries.Count > 0 {
					return false
				}
				return true
			},
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			config := obj.(*deployapi.DeploymentConfig)
			return configController.Handle(config)
		},
	}
}
