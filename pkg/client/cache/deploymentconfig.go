package cache

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// StoreToDeploymentConfigLister gives a store List and Exists methods. The store must contain only deploymentconfigs.
type StoreToDeploymentConfigLister struct {
	cache.Indexer
}

// Exists checks if the given deploymentconfig exists in the store.
func (s *StoreToDeploymentConfigLister) Exists(dc *deployapi.DeploymentConfig) (bool, error) {
	_, exists, err := s.Indexer.Get(dc)
	return exists, err
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
		return nil, fmt.Errorf("deployment config %q not found", dcName)
	}
	return obj.(*deployapi.DeploymentConfig), nil
}

func (s *StoreToDeploymentConfigLister) DeploymentConfigs(namespace string) storeDeploymentConfigsNamespacer {
	return storeDeploymentConfigsNamespacer{s.Indexer, namespace}
}

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
		return nil, fmt.Errorf("deployment config %q not found", name)
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

	key := &deployapi.DeploymentConfig{ObjectMeta: kapi.ObjectMeta{Namespace: s.namespace}}
	items, err := s.indexer.Index(cache.NamespaceIndex, key)
	if err != nil {
		// Ignore error; do slow search without index.
		glog.Warningf("can not retrieve list of objects using index : %v", err)
		for _, obj := range s.indexer.List() {
			dc := obj.(*deployapi.DeploymentConfig)
			if s.namespace == dc.Namespace && selector.Matches(labels.Set(dc.Labels)) {
				configs = append(configs, dc)
			}
		}
		return configs, nil
	}
	for _, obj := range items {
		dc := obj.(*deployapi.DeploymentConfig)
		if selector.Matches(labels.Set(dc.Labels)) {
			configs = append(configs, dc)
		}
	}
	return configs, nil
}
