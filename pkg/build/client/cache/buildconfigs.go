package cache

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/openshift/origin/pkg/build/api"
	cacheclient "github.com/openshift/origin/pkg/client/cache"
)

// NewBuildConfigGetter returns an object that implements the buildclient BuildConfigGetter interface
// using a StoreToBuildConfigLister
func NewBuildConfigGetter(lister cacheclient.StoreToBuildConfigLister) *buildConfigGetter {
	return &buildConfigGetter{
		lister: lister,
	}
}

type buildConfigGetter struct {
	lister cacheclient.StoreToBuildConfigLister
}

// Get retrieves a buildconfig from the cache
func (g *buildConfigGetter) Get(namespace, name string, options metav1.GetOptions) (*buildapi.BuildConfig, error) {
	return g.lister.BuildConfigs(namespace).Get(name, options)
}
