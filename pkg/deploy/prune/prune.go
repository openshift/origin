package prune

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// PruneFunc is a function that is invoked for each item during Prune
type PruneFunc func(item *kapi.ReplicationController) error

type PruneTasker interface {
	// PruneTask is an object that knows how to execute a single iteration of a Prune
	PruneTask() error
}

// pruneTask is an object that knows how to prune a data set
type pruneTask struct {
	resolver Resolver
	handler  PruneFunc
}

// NewPruneTasker returns a PruneTasker over specified data using specified flags
// keepYoungerThan will filter out all objects from prune data set that are younger than the specified time duration
// orphans if true will include inactive orphan deployments in candidate prune set
// keepComplete is per DeploymentConfig how many of the most recent deployments should be preserved
// keepFailed is per DeploymentConfig how many of the most recent failed deployments should be preserved
func NewPruneTasker(deploymentConfigs []*deployapi.DeploymentConfig, deployments []*kapi.ReplicationController, keepYoungerThan time.Duration, orphans bool, keepComplete int, keepFailed int, handler PruneFunc) PruneTasker {
	filter := &andFilter{
		filterPredicates: []FilterPredicate{
			FilterDeploymentsPredicate,
			FilterZeroReplicaSize,
			NewFilterBeforePredicate(keepYoungerThan),
		},
	}
	deployments = filter.Filter(deployments)
	dataSet := NewDataSet(deploymentConfigs, deployments)

	resolvers := []Resolver{}
	if orphans {
		inactiveDeploymentStatus := []deployapi.DeploymentStatus{
			deployapi.DeploymentStatusComplete,
			deployapi.DeploymentStatusFailed,
		}
		resolvers = append(resolvers, NewOrphanDeploymentResolver(dataSet, inactiveDeploymentStatus))
	}
	resolvers = append(resolvers, NewPerDeploymentConfigResolver(dataSet, keepComplete, keepFailed))
	return &pruneTask{
		resolver: &mergeResolver{resolvers: resolvers},
		handler:  handler,
	}
}

// PruneTask will visit each item in the prunable set and invoke the associated handler
func (t *pruneTask) PruneTask() error {
	deployments, err := t.resolver.Resolve()
	if err != nil {
		return err
	}
	for _, deployment := range deployments {
		err = t.handler(deployment)
		if err != nil {
			return err
		}
	}
	return nil
}
