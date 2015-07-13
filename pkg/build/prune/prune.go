package prune

import (
	"time"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// PruneFunc is a function that is invoked for each item during Prune
type PruneFunc func(build *buildapi.Build) error

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
// orphans if true will include inactive orphan builds in candidate prune set
// keepComplete is per BuildConfig how many of the most recent builds should be preserved
// keepFailed is per BuildConfig how many of the most recent failed builds should be preserved
func NewPruneTasker(buildConfigs []*buildapi.BuildConfig, builds []*buildapi.Build, keepYoungerThan time.Duration, orphans bool, keepComplete int, keepFailed int, handler PruneFunc) PruneTasker {
	filter := &andFilter{
		filterPredicates: []FilterPredicate{NewFilterBeforePredicate(keepYoungerThan)},
	}
	builds = filter.Filter(builds)
	dataSet := NewDataSet(buildConfigs, builds)

	resolvers := []Resolver{}
	if orphans {
		inactiveBuildStatus := []buildapi.BuildPhase{
			buildapi.BuildPhaseCancelled,
			buildapi.BuildPhaseComplete,
			buildapi.BuildPhaseError,
			buildapi.BuildPhaseFailed,
		}
		resolvers = append(resolvers, NewOrphanBuildResolver(dataSet, inactiveBuildStatus))
	}
	resolvers = append(resolvers, NewPerBuildConfigResolver(dataSet, keepComplete, keepFailed))
	return &pruneTask{
		resolver: &mergeResolver{resolvers: resolvers},
		handler:  handler,
	}
}

// PruneTask will visit each item in the prunable set and invoke the associated handler
func (t *pruneTask) PruneTask() error {
	builds, err := t.resolver.Resolve()
	if err != nil {
		return err
	}
	for _, build := range builds {
		err = t.handler(build)
		if err != nil {
			return err
		}
	}
	return nil
}
