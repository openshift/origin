package prune

import (
	"sort"

	"k8s.io/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// Resolver knows how to resolve the set of candidate objects to prune
type Resolver interface {
	Resolve() ([]*buildapi.Build, error)
}

// mergeResolver merges the set of results from multiple resolvers
type mergeResolver struct {
	resolvers []Resolver
}

func (m *mergeResolver) Resolve() ([]*buildapi.Build, error) {
	results := []*buildapi.Build{}
	for _, resolver := range m.resolvers {
		builds, err := resolver.Resolve()
		if err != nil {
			return nil, err
		}
		results = append(results, builds...)
	}
	return results, nil
}

// NewOrphanBuildResolver returns a Resolver that matches Build objects with no associated BuildConfig and has a BuildPhase in filter
func NewOrphanBuildResolver(dataSet DataSet, BuildPhaseFilter []buildapi.BuildPhase) Resolver {
	filter := util.NewStringSet()
	for _, BuildPhase := range BuildPhaseFilter {
		filter.Insert(string(BuildPhase))
	}
	return &orphanBuildResolver{
		dataSet:          dataSet,
		BuildPhaseFilter: filter,
	}
}

// orphanBuildResolver resolves orphan builds that match the specified filter
type orphanBuildResolver struct {
	dataSet          DataSet
	BuildPhaseFilter util.StringSet
}

// Resolve the matching set of Build objects
func (o *orphanBuildResolver) Resolve() ([]*buildapi.Build, error) {
	builds, err := o.dataSet.ListBuilds()
	if err != nil {
		return nil, err
	}

	results := []*buildapi.Build{}
	for _, build := range builds {
		if !o.BuildPhaseFilter.Has(string(build.Status.Phase)) {
			continue
		}
		isOrphan := false
		if build.Status.Config == nil {
			isOrphan = true
		} else {
			_, exists, _ := o.dataSet.GetBuildConfig(build)
			isOrphan = !exists
		}
		if isOrphan {
			results = append(results, build)
		}
	}
	return results, nil
}

type perBuildConfigResolver struct {
	dataSet      DataSet
	keepComplete int
	keepFailed   int
}

// NewPerBuildConfigResolver returns a Resolver that selects Builds to prune per BuildConfig
func NewPerBuildConfigResolver(dataSet DataSet, keepComplete int, keepFailed int) Resolver {
	return &perBuildConfigResolver{
		dataSet:      dataSet,
		keepComplete: keepComplete,
		keepFailed:   keepFailed,
	}
}

func (o *perBuildConfigResolver) Resolve() ([]*buildapi.Build, error) {
	buildConfigs, err := o.dataSet.ListBuildConfigs()
	if err != nil {
		return nil, err
	}

	completeStates := util.NewStringSet(string(buildapi.BuildPhaseComplete))
	failedStates := util.NewStringSet(string(buildapi.BuildPhaseFailed), string(buildapi.BuildPhaseError), string(buildapi.BuildPhaseCancelled))

	results := []*buildapi.Build{}
	for _, buildConfig := range buildConfigs {
		builds, err := o.dataSet.ListBuildsByBuildConfig(buildConfig)
		if err != nil {
			return nil, err
		}

		completeBuilds, failedBuilds := []*buildapi.Build{}, []*buildapi.Build{}
		for _, build := range builds {
			if completeStates.Has(string(build.Status.Phase)) {
				completeBuilds = append(completeBuilds, build)
			} else if failedStates.Has(string(build.Status.Phase)) {
				failedBuilds = append(failedBuilds, build)
			}
		}
		sort.Sort(sortableBuilds(completeBuilds))
		sort.Sort(sortableBuilds(failedBuilds))

		if o.keepComplete >= 0 && o.keepComplete < len(completeBuilds) {
			results = append(results, completeBuilds[o.keepComplete:]...)
		}
		if o.keepFailed >= 0 && o.keepFailed < len(failedBuilds) {
			results = append(results, failedBuilds[o.keepFailed:]...)
		}
	}
	return results, nil
}
