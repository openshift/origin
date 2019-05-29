package client

import (
	buildv1 "github.com/openshift/api/build/v1"
	buildclienttyped "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ClientBuildLister implements the build lister interface over a client
type ClientBuildLister struct {
	client buildclienttyped.BuildsGetter
}

// NewClientBuildClient creates a new build client that uses an openshift client to update buildclient
func NewClientBuildLister(client buildclienttyped.BuildsGetter) buildlister.BuildLister {
	return &ClientBuildLister{client: client}
}

// List lists the buildclient using the OpenShift client.
func (c *ClientBuildLister) List(label labels.Selector) ([]*buildv1.Build, error) {
	list, err := c.client.Builds(metav1.NamespaceAll).List(metav1.ListOptions{LabelSelector: label.String()})
	return buildListToPointerArray(list), err
}

func (c *ClientBuildLister) Builds(ns string) buildlister.BuildNamespaceLister {
	return &ClientBuildListerNamespacer{client: c.client, ns: ns}
}

// ClientBuildClientNamespacer implements internalversion lister
type ClientBuildListerNamespacer struct {
	client buildclienttyped.BuildsGetter
	ns     string
}

// List lists the buildclient using the OpenShift client.
func (c ClientBuildListerNamespacer) List(label labels.Selector) ([]*buildv1.Build, error) {
	list, err := c.client.Builds(c.ns).List(metav1.ListOptions{LabelSelector: label.String()})
	return buildListToPointerArray(list), err
}

func (c ClientBuildListerNamespacer) Get(name string) (*buildv1.Build, error) {
	return c.client.Builds(c.ns).Get(name, metav1.GetOptions{})
}

func buildListToPointerArray(list *buildv1.BuildList) []*buildv1.Build {
	if list == nil {
		return nil
	}
	result := make([]*buildv1.Build, len(list.Items))
	for i := range list.Items {
		result[i] = &list.Items[i]
	}
	return result
}

// ClientBuildLister implements the build lister interface over a client
type ClientBuildConfigLister struct {
	client buildclienttyped.BuildConfigsGetter
}

// NewClientBuildConfigLister creates a new build config client that uses an openshift client.
func NewClientBuildConfigLister(client buildclienttyped.BuildConfigsGetter) buildlister.BuildConfigLister {
	return &ClientBuildConfigLister{client: client}
}

// List lists the buildclient using the OpenShift client.
func (c *ClientBuildConfigLister) List(label labels.Selector) ([]*buildv1.BuildConfig, error) {
	list, err := c.client.BuildConfigs(metav1.NamespaceAll).List(metav1.ListOptions{LabelSelector: label.String()})
	return buildConfigListToPointerArray(list), err
}

func (c *ClientBuildConfigLister) BuildConfigs(ns string) buildlister.BuildConfigNamespaceLister {
	return &ClientBuildConfigListerNamespacer{client: c.client, ns: ns}
}

// ClientBuildConfigListerNamespacer implements internalversion lister
type ClientBuildConfigListerNamespacer struct {
	client buildclienttyped.BuildConfigsGetter
	ns     string
}

// List lists the buildclient using the OpenShift client.
func (c ClientBuildConfigListerNamespacer) List(label labels.Selector) ([]*buildv1.BuildConfig, error) {
	list, err := c.client.BuildConfigs(c.ns).List(metav1.ListOptions{LabelSelector: label.String()})
	return buildConfigListToPointerArray(list), err
}

func (c ClientBuildConfigListerNamespacer) Get(name string) (*buildv1.BuildConfig, error) {
	return c.client.BuildConfigs(c.ns).Get(name, metav1.GetOptions{})
}

func buildConfigListToPointerArray(list *buildv1.BuildConfigList) []*buildv1.BuildConfig {
	if list == nil {
		return nil
	}
	result := make([]*buildv1.BuildConfig, len(list.Items))
	for i := range list.Items {
		result[i] = &list.Items[i]
	}
	return result
}
