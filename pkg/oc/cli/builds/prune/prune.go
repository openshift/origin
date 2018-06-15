package prune

import (
	"time"

	"github.com/golang/glog"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/client"
)

type Pruner interface {
	// Prune is responsible for actual removal of builds identified as candidates
	// for pruning based on pruning algorithm.
	Prune(deleter buildclient.BuildDeleter) error
}

// pruner is an object that knows how to prune a data set
type pruner struct {
	resolver Resolver
}

var _ Pruner = &pruner{}

// PrunerOptions contains the fields used to initialize a new Pruner.
type PrunerOptions struct {
	// KeepYoungerThan indicates the minimum age a BuildConfig must be to be a
	// candidate for pruning.
	KeepYoungerThan time.Duration
	// Orphans if true will include inactive orphan builds in candidate prune set
	Orphans bool
	// KeepComplete is per BuildConfig how many of the most recent builds should be preserved
	KeepComplete int
	// KeepFailed is per BuildConfig how many of the most recent failed builds should be preserved
	KeepFailed int
	// BuildConfigs is the entire list of buildconfigs across all namespaces in the cluster.
	BuildConfigs []*buildapi.BuildConfig
	// Builds is the entire list of builds across all namespaces in the cluster.
	Builds []*buildapi.Build
}

// NewPruner returns a Pruner over specified data using specified options.
func NewPruner(options PrunerOptions) Pruner {
	glog.V(1).Infof("Creating build pruner with keepYoungerThan=%v, orphans=%v, keepComplete=%v, keepFailed=%v",
		options.KeepYoungerThan, options.Orphans, options.KeepComplete, options.KeepFailed)

	filter := &andFilter{
		filterPredicates: []FilterPredicate{NewFilterBeforePredicate(options.KeepYoungerThan)},
	}
	builds := filter.Filter(options.Builds)
	dataSet := NewDataSet(options.BuildConfigs, builds)

	resolvers := []Resolver{}
	if options.Orphans {
		inactiveBuildStatus := []buildapi.BuildPhase{
			buildapi.BuildPhaseCancelled,
			buildapi.BuildPhaseComplete,
			buildapi.BuildPhaseError,
			buildapi.BuildPhaseFailed,
		}
		resolvers = append(resolvers, NewOrphanBuildResolver(dataSet, inactiveBuildStatus))
	}
	resolvers = append(resolvers, NewPerBuildConfigResolver(dataSet, options.KeepComplete, options.KeepFailed))

	return &pruner{
		resolver: &mergeResolver{resolvers: resolvers},
	}
}

// Prune will visit each item in the prunable set and invoke the associated BuildDeleter.
func (p *pruner) Prune(deleter buildclient.BuildDeleter) error {
	builds, err := p.resolver.Resolve()
	if err != nil {
		return err
	}
	for _, build := range builds {
		if err := deleter.DeleteBuild(build); err != nil {
			return err
		}
	}
	return nil
}
