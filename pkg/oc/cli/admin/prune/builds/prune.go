package builds

import (
	"time"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildv1 "github.com/openshift/api/build/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
)

type Pruner interface {
	// Prune is responsible for actual removal of builds identified as candidates
	// for pruning based on pruning algorithm.
	Prune(deleter BuildDeleter) error
}

type BuildDeleter interface {
	DeleteBuild(build *buildv1.Build) error
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
	BuildConfigs []*buildv1.BuildConfig
	// Builds is the entire list of builds across all namespaces in the cluster.
	Builds []*buildv1.Build
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
		inactiveBuildStatus := []buildv1.BuildPhase{
			buildv1.BuildPhaseCancelled,
			buildv1.BuildPhaseComplete,
			buildv1.BuildPhaseError,
			buildv1.BuildPhaseFailed,
		}
		resolvers = append(resolvers, NewOrphanBuildResolver(dataSet, inactiveBuildStatus))
	}
	resolvers = append(resolvers, NewPerBuildConfigResolver(dataSet, options.KeepComplete, options.KeepFailed))

	return &pruner{
		resolver: &mergeResolver{resolvers: resolvers},
	}
}

// Prune will visit each item in the prunable set and invoke the associated BuildDeleter.
func (p *pruner) Prune(deleter BuildDeleter) error {
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

// NewBuildDeleter creates a new buildDeleter.
func NewBuildDeleter(client buildv1client.BuildsGetter) BuildDeleter {
	return &buildDeleter{
		client: client,
	}
}

type buildDeleter struct {
	client buildv1client.BuildsGetter
}

var _ BuildDeleter = &buildDeleter{}

func (c *buildDeleter) DeleteBuild(build *buildv1.Build) error {
	return c.client.Builds(build.Namespace).Delete(build.Name, &metav1.DeleteOptions{})
}
