package deployments

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/openshift/api/apps/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

// Resolver knows how to resolve the set of candidate objects to prune
type Resolver interface {
	Resolve() ([]*corev1.ReplicationController, error)
}

// mergeResolver merges the set of results from multiple resolvers
type mergeResolver struct {
	resolvers []Resolver
}

func (m *mergeResolver) Resolve() ([]*corev1.ReplicationController, error) {
	results := []*corev1.ReplicationController{}
	for _, resolver := range m.resolvers {
		items, err := resolver.Resolve()
		if err != nil {
			return nil, err
		}
		results = append(results, items...)
	}
	return results, nil
}

// NewOrphanDeploymentResolver returns a Resolver that matches objects with no associated DeploymentConfig and has a DeploymentStatus in filter
func NewOrphanDeploymentResolver(dataSet DataSet, deploymentStatusFilter []appsv1.DeploymentStatus) Resolver {
	filter := sets.NewString()
	for _, deploymentStatus := range deploymentStatusFilter {
		filter.Insert(string(deploymentStatus))
	}
	return &orphanDeploymentResolver{
		dataSet:                dataSet,
		deploymentStatusFilter: filter,
	}
}

// orphanDeploymentResolver resolves orphan deployments that match the specified filter
type orphanDeploymentResolver struct {
	dataSet                DataSet
	deploymentStatusFilter sets.String
}

// Resolve the matching set of objects
func (o *orphanDeploymentResolver) Resolve() ([]*corev1.ReplicationController, error) {
	deployments, err := o.dataSet.ListDeployments()
	if err != nil {
		return nil, err
	}

	results := []*corev1.ReplicationController{}
	for _, deployment := range deployments {
		deploymentStatus := appsutil.DeploymentStatusFor(deployment)
		if !o.deploymentStatusFilter.Has(string(deploymentStatus)) {
			continue
		}
		_, exists, _ := o.dataSet.GetDeploymentConfig(deployment)
		if !exists {
			results = append(results, deployment)
		}
	}
	return results, nil
}

type perDeploymentConfigResolver struct {
	dataSet      DataSet
	keepComplete int
	keepFailed   int
}

// NewPerDeploymentConfigResolver returns a Resolver that selects items to prune per config
func NewPerDeploymentConfigResolver(dataSet DataSet, keepComplete int, keepFailed int) Resolver {
	return &perDeploymentConfigResolver{
		dataSet:      dataSet,
		keepComplete: keepComplete,
		keepFailed:   keepFailed,
	}
}

// ByMostRecent sorts deployments by most recently created.
type ByMostRecent []*corev1.ReplicationController

func (s ByMostRecent) Len() int      { return len(s) }
func (s ByMostRecent) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByMostRecent) Less(i, j int) bool {
	return !s[i].CreationTimestamp.Before(&s[j].CreationTimestamp)
}

func (o *perDeploymentConfigResolver) Resolve() ([]*corev1.ReplicationController, error) {
	deploymentConfigs, err := o.dataSet.ListDeploymentConfigs()
	if err != nil {
		return nil, err
	}

	completeStates := sets.NewString(string(appsv1.DeploymentStatusComplete))
	failedStates := sets.NewString(string(appsv1.DeploymentStatusFailed))

	results := []*corev1.ReplicationController{}
	for _, deploymentConfig := range deploymentConfigs {
		deployments, err := o.dataSet.ListDeploymentsByDeploymentConfig(deploymentConfig)
		if err != nil {
			return nil, err
		}

		completeDeployments, failedDeployments := []*corev1.ReplicationController{}, []*corev1.ReplicationController{}
		for _, deployment := range deployments {
			status := appsutil.DeploymentStatusFor(deployment)
			if completeStates.Has(string(status)) {
				completeDeployments = append(completeDeployments, deployment)
			} else if failedStates.Has(string(status)) {
				failedDeployments = append(failedDeployments, deployment)
			}
		}
		sort.Sort(ByMostRecent(completeDeployments))
		sort.Sort(ByMostRecent(failedDeployments))

		if o.keepComplete >= 0 && o.keepComplete < len(completeDeployments) {
			results = append(results, completeDeployments[o.keepComplete:]...)
		}
		if o.keepFailed >= 0 && o.keepFailed < len(failedDeployments) {
			results = append(results, failedDeployments[o.keepFailed:]...)
		}
	}
	return results, nil
}
