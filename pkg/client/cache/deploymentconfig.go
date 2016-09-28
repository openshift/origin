package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// StoreToDeploymentConfigLister gives a store List and Exists methods. The store must contain only deploymentconfigs.
type StoreToDeploymentConfigLister struct {
	cache.Indexer
}

// List all deploymentconfigs in the store.
func (s *StoreToDeploymentConfigLister) List() ([]*deployapi.DeploymentConfig, error) {
	configs := []*deployapi.DeploymentConfig{}
	for _, c := range s.Indexer.List() {
		configs = append(configs, c.(*deployapi.DeploymentConfig))
	}
	return configs, nil
}

// GetConfigForController returns the managing deployment config for the provided replication controller.
func (s *StoreToDeploymentConfigLister) GetConfigForController(rc *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
	dcName := deployutil.DeploymentConfigNameFor(rc)
	obj, exists, err := s.Indexer.GetByKey(rc.Namespace + "/" + dcName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, kapierrors.NewNotFound(deployapi.Resource("deploymentconfig"), dcName)
	}
	return obj.(*deployapi.DeploymentConfig), nil
}

// GetConfigForPod returns the managing deployment config for the provided pod.
func (s *StoreToDeploymentConfigLister) GetConfigForPod(pod *kapi.Pod) (*deployapi.DeploymentConfig, error) {
	dcName := deployutil.DeploymentConfigNameFor(pod)
	obj, exists, err := s.Indexer.GetByKey(pod.Namespace + "/" + dcName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, kapierrors.NewNotFound(deployapi.Resource("deploymentconfig"), dcName)
	}
	return obj.(*deployapi.DeploymentConfig), nil
}

// GetConfigsForImageStream returns all the deployment configs that point to the provided image stream
// by searching through using the ImageStreamReferenceIndex (deployment configs are indexed in the cache
// by namespace and by image stream references).
func (s *StoreToDeploymentConfigLister) GetConfigsForImageStream(stream *imageapi.ImageStream) ([]*deployapi.DeploymentConfig, error) {
	items, err := s.Indexer.ByIndex(ImageStreamReferenceIndex, stream.Namespace+"/"+stream.Name)
	if err != nil {
		return nil, err
	}

	var configs []*deployapi.DeploymentConfig

	for _, obj := range items {
		config := obj.(*deployapi.DeploymentConfig)
		configs = append(configs, config)
	}

	return configs, nil
}

func (s *StoreToDeploymentConfigLister) DeploymentConfigs(namespace string) storeDeploymentConfigsNamespacer {
	return storeDeploymentConfigsNamespacer{s.Indexer, namespace}
}

// storeDeploymentConfigsNamespacer provides a way to get and list DeploymentConfigs from a specific namespace.
type storeDeploymentConfigsNamespacer struct {
	indexer   cache.Indexer
	namespace string
}

// Get the deployment config matching the name from the cache.
func (s storeDeploymentConfigsNamespacer) Get(name string) (*deployapi.DeploymentConfig, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, kapierrors.NewNotFound(deployapi.Resource("deploymentconfig"), name)
	}
	return obj.(*deployapi.DeploymentConfig), nil
}

// List all the deploymentconfigs that match the provided selector using a namespace index.
// If the indexed list fails then we will fallback to listing from all namespaces and filter
// by the namespace we want.
func (s storeDeploymentConfigsNamespacer) List(selector labels.Selector) ([]*deployapi.DeploymentConfig, error) {
	configs := []*deployapi.DeploymentConfig{}

	if s.namespace == kapi.NamespaceAll {
		for _, obj := range s.indexer.List() {
			dc := obj.(*deployapi.DeploymentConfig)
			if selector.Matches(labels.Set(dc.Labels)) {
				configs = append(configs, dc)
			}
		}
		return configs, nil
	}

	items, err := s.indexer.ByIndex(cache.NamespaceIndex, s.namespace)
	if err != nil {
		return nil, err
	}
	for _, obj := range items {
		dc := obj.(*deployapi.DeploymentConfig)
		if selector.Matches(labels.Set(dc.Labels)) {
			configs = append(configs, dc)
		}
	}
	return configs, nil
}
