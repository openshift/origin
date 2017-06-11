package cache

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
	cacheclient "github.com/openshift/origin/pkg/client/cache"
)

// NewBuildLister returns an object that implements the buildclient BuildLister interface
// using a StoreToBuildLister
func NewBuildLister(lister *cacheclient.StoreToBuildLister) *buildLister {
	return &buildLister{
		lister: lister,
	}
}

type buildLister struct {
	lister *cacheclient.StoreToBuildLister
}

// List returns a BuildList with the given namespace and get options. Only the LabelSelector
// from the ListOptions is honored.
func (l *buildLister) List(namespace string, opts metav1.ListOptions) (*buildapi.BuildList, error) {
	selector, err := labels.Parse(opts.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector %q: %v", opts.LabelSelector, err)
	}
	builds, err := l.lister.Builds(namespace).List(selector)
	if err != nil {
		return nil, err
	}
	return buildList(builds), nil
}

func buildList(builds []*buildapi.Build) *buildapi.BuildList {
	items := []buildapi.Build{}
	for _, b := range builds {
		if b != nil {
			items = append(items, *b)
		}
	}
	return &buildapi.BuildList{
		Items: items,
	}
}
