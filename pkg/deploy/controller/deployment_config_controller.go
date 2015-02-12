package controller

import (
	"fmt"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	cache "github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigController is responsible for creating a deployment when a DeploymentConfig is
// updated with a new LatestVersion. Any deployment created is correlated to a DeploymentConfig
// by setting the DeploymentConfigLabel on the deployment.
//
// Deployments are represented by ReplicationControllers. The DeploymentConfig used to create the
// ReplicationController is encoded and stored in an annotation on the ReplicationController.
type DeploymentConfigController struct {
	deploymentConfigStore cache.Store
	deploymentStore       cache.Store
	client                deploymentConfigControllerClient
	codec                 runtime.Codec
	handleError           func(error)
	sync                  <-chan struct{}
	stop                  <-chan struct{}
}

// NewDeploymentConfigController creates a DeploymentConfigController using API clients. The
// controller returned will be configured to perform a sync:
//
//   1. Every fullSyncPeriod.
//   2. Any time a watch event for DeploymentConfig is observed.
//
// Errors are delegated to util.HandleError.
func NewDeploymentConfigController(osClient osclient.DeploymentConfigsNamespacer, kClient kclient.ReplicationControllersNamespacer,
	codec runtime.Codec, stop <-chan struct{}, fullSyncPeriod time.Duration) *DeploymentConfigController {
	// Make the ListWatcher for deploymentConfigs
	deploymentConfigLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return osClient.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return osClient.DeploymentConfigs(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
		},
	}

	// Make the ListWatcher for deployments
	deploymentLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return kClient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return kClient.ReplicationControllers(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
		},
	}

	// Initialize the stores with reflectors
	deploymentConfigStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentConfigLW, &deployapi.DeploymentConfig{}, deploymentConfigStore).RunUntil(stop)

	deploymentStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentStore).RunUntil(stop)

	// Create a deploymentClient impl backed by the kube client
	deploymentClient := &deploymentConfigControllerClientImpl{
		CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			return kClient.ReplicationControllers(namespace).Create(deployment)
		},
	}

	// Sync every fullSyncPeriod, or every time a deploymentConfig watch event is observed.
	// TODO: Compress events to prevent sync spamming.
	// TODO: Extract this into something more generic.
	sync := make(chan struct{})
	go func() {
		// TODO: error handling
		configWatch, _ := osClient.DeploymentConfigs(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), "")
		ticker := time.NewTicker(fullSyncPeriod)
		for {
			select {
			case <-ticker.C:
				sync <- struct{}{}
			case event, ok := <-configWatch.ResultChan():
				if !ok || event.Type == watch.Error {
					glog.Errorf("Re-establishing deploymentConfig watch")
					configWatch, _ = osClient.DeploymentConfigs(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), "")
					continue
				}
				sync <- struct{}{}
			case <-stop:
				return
			}
		}
	}()

	handleError := func(err error) {
		// TODO: This is the point where more intelligent error handling
		// decisions can be made after each iteration. For example, if the error
		// contained enough context, we could choose whether or not to sync
		// based on a stateful backoff construct. Or, if the errors were fatal,
		// we may choose to never retry, wait even longer, or even actively
		// purge invalid states or otherwise quarantine them (e.g. via labels)
		// to get them out of the processing dataset.
		glog.Errorf("Scheduling sync of deploymentConfigs because the last sync failed: %v", err)
		sync <- struct{}{}
	}

	return &DeploymentConfigController{
		client:                deploymentClient,
		deploymentConfigStore: deploymentConfigStore,
		deploymentStore:       deploymentStore,
		codec:                 codec,
		sync:                  sync,
		stop:                  stop,
		handleError:           handleError,
	}
}

// Run performs a full sync every period, or when a message is received on
// sync, until a message is received on the stop channel. If a sync iteration
// returns an error, the handleError function is called and receives the error
// for processing before the next iteration occurs.
func (c *DeploymentConfigController) Run() {
	go util.Until(func() {
		select {
		case <-c.sync:
			err := c.syncAll()
			if err != nil {
				c.handleError(err)
			}
		}
	}, 0, c.stop)
}

// syncAll performs a sync on every config in the store. If any individual
// sync operation returns an error, the syncAll call returns an error. Even
// when an individual sync operation fails, sync will stil be called for each
// config.
func (c *DeploymentConfigController) syncAll() error {
	failed := false
	for _, config := range c.deploymentConfigStore.List() {
		err := c.syncOne(config.(*deployapi.DeploymentConfig))
		if err != nil {
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("Failed to process one or more configs")
	}

	return nil
}

// sync examines the current state of a DeploymentConfig, and creates a new
// deployment for the config if the following conditions are true:
//
//   1. The config version is greater than 0
//   2. No deployment exists corresponding to  the config's version
//
// If the config can't be processed, an error is returned.
func (c *DeploymentConfigController) syncOne(config *deployapi.DeploymentConfig) error {
	// Only deploy when the version has advanced past 0.
	if config.LatestVersion == 0 {
		glog.V(5).Infof("Waiting for first version of %s", labelFor(config))
		return nil
	}

	var deploymentExists bool
	var err error
	var deployment *kapi.ReplicationController

	// Find any existing deployment.
	_, deploymentExists, err = c.deploymentStore.Get(&kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: config.Namespace,
			Name:      deployutil.LatestDeploymentNameForConfig(config),
		}})
	if err != nil {
		return fmt.Errorf("Couldn't retrieve deployment from store: %v", err)
	}

	// Only deploy if there's no existing deployment for this config.
	if deploymentExists {
		return nil
	}

	// Try and build a deployment for the config.
	deployment, err = deployutil.MakeDeployment(config, c.codec)
	if err != nil {
		return fmt.Errorf("Couldn't make deployment from (potentially invalid) config %s: %v", labelFor(config), err)
	}

	// Create the deployment.
	_, err = c.client.CreateDeployment(config.Namespace, deployment)
	if err != nil {
		// If the deployment was already created, just move on. The cache could be stale, or another
		// process could have already handled this update.
		if kerrors.IsAlreadyExists(err) {
			glog.V(4).Infof("Deployment already exists for config %s", labelFor(config))
			return nil
		}
		return fmt.Errorf("Couldn't create deployment for config %s: %v", labelFor(config), err)
	}

	glog.V(4).Infof("Created deployment for config %s", labelFor(config))
	return nil
}

// labelFor builds a string identifier for a DeploymentConfig.
func labelFor(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s/%s:%d", config.Namespace, config.Name, config.LatestVersion)
}

// deploymentConfigControllerClient is a private API client abstraction.
type deploymentConfigControllerClient interface {
	CreateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// deploymentConfigControllerClientImpl is a generic deploymentConfigControllerClient implementation.
type deploymentConfigControllerClientImpl struct {
	CreateDeploymentFunc func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *deploymentConfigControllerClientImpl) CreateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.CreateDeploymentFunc(namespace, deployment)
}
