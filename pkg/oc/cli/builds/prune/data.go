package prune

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

// BuildByBuildConfigIndexFunc indexes Build items by their associated BuildConfig, if none, index with key "orphan"
func BuildByBuildConfigIndexFunc(obj interface{}) ([]string, error) {
	build, ok := obj.(*buildapi.Build)
	if !ok {
		return nil, fmt.Errorf("not a build: %v", build)
	}
	config := build.Status.Config
	if config == nil {
		return []string{"orphan"}, nil
	}
	return []string{config.Namespace + "/" + config.Name}, nil
}

// Filter filters the set of objects
type Filter interface {
	Filter(builds []*buildapi.Build) []*buildapi.Build
}

// andFilter ands a set of predicate functions to know if it should be included in the return set
type andFilter struct {
	filterPredicates []FilterPredicate
}

// Filter ands the set of predicates evaluated against each Build to make a filtered set
func (a *andFilter) Filter(builds []*buildapi.Build) []*buildapi.Build {
	results := []*buildapi.Build{}
	for _, build := range builds {
		include := true
		for _, filterPredicate := range a.filterPredicates {
			include = include && filterPredicate(build)
		}
		if include {
			results = append(results, build)
		}
	}
	return results
}

// FilterPredicate is a function that returns true if the object should be included in the filtered set
type FilterPredicate func(build *buildapi.Build) bool

// NewFilterBeforePredicate is a function that returns true if the build was created before the current time minus specified duration
func NewFilterBeforePredicate(d time.Duration) FilterPredicate {
	now := metav1.Now()
	before := metav1.NewTime(now.Time.Add(-1 * d))
	return func(build *buildapi.Build) bool {
		return build.CreationTimestamp.Before(&before)
	}
}

// DataSet provides functions for working with build data
type DataSet interface {
	GetBuildConfig(build *buildapi.Build) (*buildapi.BuildConfig, bool, error)
	ListBuildConfigs() ([]*buildapi.BuildConfig, error)
	ListBuilds() ([]*buildapi.Build, error)
	ListBuildsByBuildConfig(buildConfig *buildapi.BuildConfig) ([]*buildapi.Build, error)
}

type dataSet struct {
	buildConfigStore cache.Store
	buildIndexer     cache.Indexer
}

// NewDataSet returns a DataSet over the specified items
func NewDataSet(buildConfigs []*buildapi.BuildConfig, builds []*buildapi.Build) DataSet {
	buildConfigStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, buildConfig := range buildConfigs {
		buildConfigStore.Add(buildConfig)
	}

	buildIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		"buildConfig": BuildByBuildConfigIndexFunc,
	})
	for _, build := range builds {
		buildIndexer.Add(build)
	}

	return &dataSet{
		buildConfigStore: buildConfigStore,
		buildIndexer:     buildIndexer,
	}
}

func (d *dataSet) GetBuildConfig(build *buildapi.Build) (*buildapi.BuildConfig, bool, error) {
	config := build.Status.Config
	if config == nil {
		return nil, false, nil
	}

	var buildConfig *buildapi.BuildConfig
	key := &buildapi.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: config.Name, Namespace: config.Namespace}}
	item, exists, err := d.buildConfigStore.Get(key)
	if exists {
		buildConfig = item.(*buildapi.BuildConfig)
	}
	return buildConfig, exists, err
}

func (d *dataSet) ListBuildConfigs() ([]*buildapi.BuildConfig, error) {
	results := []*buildapi.BuildConfig{}
	for _, item := range d.buildConfigStore.List() {
		results = append(results, item.(*buildapi.BuildConfig))
	}
	return results, nil
}

func (d *dataSet) ListBuilds() ([]*buildapi.Build, error) {
	results := []*buildapi.Build{}
	for _, item := range d.buildIndexer.List() {
		results = append(results, item.(*buildapi.Build))
	}
	return results, nil
}

func (d *dataSet) ListBuildsByBuildConfig(buildConfig *buildapi.BuildConfig) ([]*buildapi.Build, error) {
	results := []*buildapi.Build{}
	key := &buildapi.Build{}
	key.Status.Config = &kapi.ObjectReference{Name: buildConfig.Name, Namespace: buildConfig.Namespace}
	items, err := d.buildIndexer.Index("buildConfig", key)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		results = append(results, item.(*buildapi.Build))
	}
	return results, nil
}
