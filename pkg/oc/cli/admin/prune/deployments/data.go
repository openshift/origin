package deployments

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	appsv1 "github.com/openshift/api/apps/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

// DeploymentByDeploymentConfigIndexFunc indexes Deployment items by their associated DeploymentConfig, if none, index with key "orphan"
func DeploymentByDeploymentConfigIndexFunc(obj interface{}) ([]string, error) {
	controller, ok := obj.(*corev1.ReplicationController)
	if !ok {
		return nil, fmt.Errorf("not a replication controller: %v", obj)
	}
	name := appsutil.DeploymentConfigNameFor(controller)
	if len(name) == 0 {
		return []string{"orphan"}, nil
	}
	return []string{controller.Namespace + "/" + name}, nil
}

// Filter filters the set of objects
type Filter interface {
	Filter(items []*corev1.ReplicationController) []*corev1.ReplicationController
}

// andFilter ands a set of predicate functions to know if it should be included in the return set
type andFilter struct {
	filterPredicates []FilterPredicate
}

// Filter ands the set of predicates evaluated against each item to make a filtered set
func (a *andFilter) Filter(items []*corev1.ReplicationController) []*corev1.ReplicationController {
	results := []*corev1.ReplicationController{}
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
type FilterPredicate func(item *corev1.ReplicationController) bool

// NewFilterBeforePredicate is a function that returns true if the build was created before the current time minus specified duration
func NewFilterBeforePredicate(d time.Duration) FilterPredicate {
	now := metav1.Now()
	before := metav1.NewTime(now.Time.Add(-1 * d))
	return func(item *corev1.ReplicationController) bool {
		return item.CreationTimestamp.Before(&before)
	}
}

// FilterDeploymentsPredicate is a function that returns true if the replication controller is associated with a DeploymentConfig
func FilterDeploymentsPredicate(item *corev1.ReplicationController) bool {
	return len(appsutil.DeploymentConfigNameFor(item)) > 0
}

// FilterZeroReplicaSize is a function that returns true if the replication controller size is 0
func FilterZeroReplicaSize(item *corev1.ReplicationController) bool {
	return *item.Spec.Replicas == 0 && item.Status.Replicas == 0
}

// DataSet provides functions for working with deployment data
type DataSet interface {
	GetDeploymentConfig(deployment *corev1.ReplicationController) (*appsv1.DeploymentConfig, bool, error)
	ListDeploymentConfigs() ([]*appsv1.DeploymentConfig, error)
	ListDeployments() ([]*corev1.ReplicationController, error)
	ListDeploymentsByDeploymentConfig(config *appsv1.DeploymentConfig) ([]*corev1.ReplicationController, error)
}

type dataSet struct {
	deploymentConfigStore cache.Store
	deploymentIndexer     cache.Indexer
}

// NewDataSet returns a DataSet over the specified items
func NewDataSet(deploymentConfigs []*appsv1.DeploymentConfig, deployments []*corev1.ReplicationController) DataSet {
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
func (d *dataSet) GetDeploymentConfig(controller *corev1.ReplicationController) (*appsv1.DeploymentConfig, bool, error) {
	name := appsutil.DeploymentConfigNameFor(controller)
	if len(name) == 0 {
		return nil, false, nil
	}

	var deploymentConfig *appsv1.DeploymentConfig
	key := &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: controller.Namespace}}
	item, exists, err := d.deploymentConfigStore.Get(key)
	if exists {
		deploymentConfig = item.(*appsv1.DeploymentConfig)
	}
	return deploymentConfig, exists, err
}

// ListDeploymentConfigs returns a list of DeploymentConfigs
func (d *dataSet) ListDeploymentConfigs() ([]*appsv1.DeploymentConfig, error) {
	results := []*appsv1.DeploymentConfig{}
	for _, item := range d.deploymentConfigStore.List() {
		results = append(results, item.(*appsv1.DeploymentConfig))
	}
	return results, nil
}

// ListDeployments returns a list of deployments
func (d *dataSet) ListDeployments() ([]*corev1.ReplicationController, error) {
	results := []*corev1.ReplicationController{}
	for _, item := range d.deploymentIndexer.List() {
		results = append(results, item.(*corev1.ReplicationController))
	}
	return results, nil
}

// ListDeploymentsByDeploymentConfig returns a list of deployments for the provided configuration
func (d *dataSet) ListDeploymentsByDeploymentConfig(deploymentConfig *appsv1.DeploymentConfig) ([]*corev1.ReplicationController, error) {
	results := []*corev1.ReplicationController{}
	key := &corev1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   deploymentConfig.Namespace,
			Annotations: map[string]string{appsv1.DeploymentConfigAnnotation: deploymentConfig.Name},
		},
	}
	items, err := d.deploymentIndexer.Index("deploymentConfig", key)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		results = append(results, item.(*corev1.ReplicationController))
	}
	return results, nil
}
