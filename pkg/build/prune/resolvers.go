package prune

import (
	"sort"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

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

// NewOrphanBuildResolver returns a Resolver that matches Build objects with no associated BuildConfig and has a BuildStatus in filter
func NewOrphanBuildResolver(dataSet DataSet, buildStatusFilter []buildapi.BuildStatus) Resolver {
	filter := util.NewStringSet()
	for _, buildStatus := range buildStatusFilter {
		filter.Insert(string(buildStatus))
	}
	return &orphanBuildResolver{
		dataSet:           dataSet,
		buildStatusFilter: filter,
	}
}

// orphanBuildResolver resolves orphan builds that match the specified filter
type orphanBuildResolver struct {
	dataSet           DataSet
	buildStatusFilter util.StringSet
}

// Resolve the matching set of Build objects
func (o *orphanBuildResolver) Resolve() ([]*buildapi.Build, error) {
	builds, err := o.dataSet.ListBuilds()
	if err != nil {
		return nil, err
	}

	results := []*buildapi.Build{}
	for _, build := range builds {
		if !o.buildStatusFilter.Has(string(build.Status)) {
			continue
		}
		isOrphan := false
		if build.Config == nil {
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

	completeStates := util.NewStringSet(string(buildapi.BuildStatusComplete))
	failedStates := util.NewStringSet(string(buildapi.BuildStatusFailed), string(buildapi.BuildStatusError), string(buildapi.BuildStatusCancelled))

	results := []*buildapi.Build{}
	for _, buildConfig := range buildConfigs {
		builds, err := o.dataSet.ListBuildsByBuildConfig(buildConfig)
		if err != nil {
			return nil, err
		}

		completeBuilds, failedBuilds := []*buildapi.Build{}, []*buildapi.Build{}
		for _, build := range builds {
			if completeStates.Has(string(build.Status)) {
				completeBuilds = append(completeBuilds, build)
			} else if failedStates.Has(string(build.Status)) {
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
