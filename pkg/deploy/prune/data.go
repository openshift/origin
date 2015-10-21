package prune

import (
	"fmt"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentByDeploymentConfigIndexFunc indexes Deployment items by their associated DeploymentConfig, if none, index with key "orphan"
func DeploymentByDeploymentConfigIndexFunc(obj interface{}) ([]string, error) {
	controller, ok := obj.(*kapi.ReplicationController)
	if !ok {
		return nil, fmt.Errorf("not a replication controller: %v", obj)
	}
	name := deployutil.DeploymentConfigNameFor(controller)
	if len(name) == 0 {
		return []string{"orphan"}, nil
	}
	return []string{controller.Namespace + "/" + name}, nil
}

// Filter filters the set of objects
type Filter interface {
	Filter(items []*kapi.ReplicationController) []*kapi.ReplicationController
}

// andFilter ands a set of predicate functions to know if it should be included in the return set
type andFilter struct {
	filterPredicates []FilterPredicate
}

// Filter ands the set of predicates evaluated against each item to make a filtered set
func (a *andFilter) Filter(items []*kapi.ReplicationController) []*kapi.ReplicationController {
	results := []*kapi.ReplicationController{}
	for _, item := range items {
		include := true
		for _, filterPredicate := range a.filterPredicates {
			include = include && filterPredicate(item)
		}
		if include {
			results = append(results, item)
		}
	}
	return results
}

// FilterPredicate is a function that returns true if the object should be included in the filtered set
type FilterPredicate func(item *kapi.ReplicationController) bool

// NewFilterBeforePredicate is a function that returns true if the build was created before the current time minus specified duration
func NewFilterBeforePredicate(d time.Duration) FilterPredicate {
	now := unversioned.Now()
	before := unversioned.NewTime(now.Time.Add(-1 * d))
	return func(item *kapi.ReplicationController) bool {
		return item.CreationTimestamp.Before(before)
	}
}

// FilterDeploymentsPredicate is a function that returns true if the replication controller is associated with a DeploymentConfig
func FilterDeploymentsPredicate(item *kapi.ReplicationController) bool {
	return len(deployutil.DeploymentConfigNameFor(item)) > 0
}

// FilterZeroReplicaSize is a function that returns true if the replication controller size is 0
func FilterZeroReplicaSize(item *kapi.ReplicationController) bool {
	return item.Spec.Replicas == 0 && item.Status.Replicas == 0
}

// DataSet provides functions for working with deployment data
type DataSet interface {
	GetDeploymentConfig(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, bool, error)
	ListDeploymentConfigs() ([]*deployapi.DeploymentConfig, error)
	ListDeployments() ([]*kapi.ReplicationController, error)
	ListDeploymentsByDeploymentConfig(config *deployapi.DeploymentConfig) ([]*kapi.ReplicationController, error)
}

type dataSet struct {
	deploymentConfigStore cache.Store
	deploymentIndexer     cache.Indexer
}

// NewDataSet returns a DataSet over the specified items
func NewDataSet(deploymentConfigs []*deployapi.DeploymentConfig, deployments []*kapi.ReplicationController) DataSet {
	deploymentConfigStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, deploymentConfig := range deploymentConfigs {
		deploymentConfigStore.Add(deploymentConfig)
	}

	deploymentIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		"deploymentConfig": DeploymentByDeploymentConfigIndexFunc,
	})
	for _, deployment := range deployments {
		deploymentIndexer.Add(deployment)
	}

	return &dataSet{
		deploymentConfigStore: deploymentConfigStore,
		deploymentIndexer:     deploymentIndexer,
	}
}

// GetDeploymentConfig gets the configuration for the given deployment
func (d *dataSet) GetDeploymentConfig(controller *kapi.ReplicationController) (*deployapi.DeploymentConfig, bool, error) {
	name := deployutil.DeploymentConfigNameFor(controller)
	if len(name) == 0 {
		return nil, false, nil
	}

	var deploymentConfig *deployapi.DeploymentConfig
	key := &deployapi.DeploymentConfig{ObjectMeta: kapi.ObjectMeta{Name: name, Namespace: controller.Namespace}}
	item, exists, err := d.deploymentConfigStore.Get(key)
	if exists {
		deploymentConfig = item.(*deployapi.DeploymentConfig)
	}
	return deploymentConfig, exists, err
}

// ListDeploymentConfigs returns a list of DeploymentConfigs
func (d *dataSet) ListDeploymentConfigs() ([]*deployapi.DeploymentConfig, error) {
	results := []*deployapi.DeploymentConfig{}
	for _, item := range d.deploymentConfigStore.List() {
		results = append(results, item.(*deployapi.DeploymentConfig))
	}
	return results, nil
}

// ListDeployments returns a list of deployments
func (d *dataSet) ListDeployments() ([]*kapi.ReplicationController, error) {
	results := []*kapi.ReplicationController{}
	for _, item := range d.deploymentIndexer.List() {
		results = append(results, item.(*kapi.ReplicationController))
	}
	return results, nil
}

// ListDeploymentsByDeploymentConfig returns a list of deployments for the provided configuration
func (d *dataSet) ListDeploymentsByDeploymentConfig(deploymentConfig *deployapi.DeploymentConfig) ([]*kapi.ReplicationController, error) {
	results := []*kapi.ReplicationController{}
	key := &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Namespace:   deploymentConfig.Namespace,
			Annotations: map[string]string{deployapi.DeploymentConfigAnnotation: deploymentConfig.Name},
		},
	}
	items, err := d.deploymentIndexer.Index("deploymentConfig", key)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		results = append(results, item.(*kapi.ReplicationController))
	}
	return results, nil
}
