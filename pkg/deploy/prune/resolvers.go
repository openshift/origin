package prune

import (
	"sort"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// Resolver knows how to resolve the set of candidate objects to prune
type Resolver interface {
	Resolve() ([]*kapi.ReplicationController, error)
}

// mergeResolver merges the set of results from multiple resolvers
type mergeResolver struct {
	resolvers []Resolver
}

func (m *mergeResolver) Resolve() ([]*kapi.ReplicationController, error) {
	results := []*kapi.ReplicationController{}
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
func NewOrphanDeploymentResolver(dataSet DataSet, deploymentStatusFilter []deployapi.DeploymentStatus) Resolver {
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
func (o *orphanDeploymentResolver) Resolve() ([]*kapi.ReplicationController, error) {
	deployments, err := o.dataSet.ListDeployments()
	if err != nil {
		return nil, err
	}

	results := []*kapi.ReplicationController{}
	for _, deployment := range deployments {
		deploymentStatus := deployutil.DeploymentStatusFor(deployment)
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

func (o *perDeploymentConfigResolver) Resolve() ([]*kapi.ReplicationController, error) {
	deploymentConfigs, err := o.dataSet.ListDeploymentConfigs()
	if err != nil {
		return nil, err
	}

	completeStates := sets.NewString(string(deployapi.DeploymentStatusComplete))
	failedStates := sets.NewString(string(deployapi.DeploymentStatusFailed))

	results := []*kapi.ReplicationController{}
	for _, deploymentConfig := range deploymentConfigs {
		deployments, err := o.dataSet.ListDeploymentsByDeploymentConfig(deploymentConfig)
		if err != nil {
			return nil, err
		}

		completeDeployments, failedDeployments := []*kapi.ReplicationController{}, []*kapi.ReplicationController{}
		for _, deployment := range deployments {
			status := deployutil.DeploymentStatusFor(deployment)
			if completeStates.Has(string(status)) {
				completeDeployments = append(completeDeployments, deployment)
			} else if failedStates.Has(string(status)) {
				failedDeployments = append(failedDeployments, deployment)
			}
		}
		sort.Sort(sortableReplicationControllers(completeDeployments))
		sort.Sort(sortableReplicationControllers(failedDeployments))

		if o.keepComplete >= 0 && o.keepComplete < len(completeDeployments) {
			results = append(results, completeDeployments[o.keepComplete:]...)
		}
		if o.keepFailed >= 0 && o.keepFailed < len(failedDeployments) {
			results = append(results, failedDeployments[o.keepFailed:]...)
		}
	}
	return results, nil
}
