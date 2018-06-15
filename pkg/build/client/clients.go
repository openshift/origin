package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	builds "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
)

// BuildConfigGetter provides methods for getting BuildConfigs
type BuildConfigGetter interface {
	Get(namespace, name string, options metav1.GetOptions) (*buildapi.BuildConfig, error)
}

// BuildConfigUpdater provides methods for updating BuildConfigs
type BuildConfigUpdater interface {
	Update(buildConfig *buildapi.BuildConfig) error
}

// ClientBuildConfigClient delegates get and update operations to the OpenShift client interface
type ClientBuildConfigClient struct {
	Client builds.Interface
}

// NewClientBuildConfigClient creates a new build config client that uses an openshift client to create and get BuildConfigs
func NewClientBuildConfigClient(client builds.Interface) *ClientBuildConfigClient {
	return &ClientBuildConfigClient{Client: client}
}

// Get returns a BuildConfig using the OpenShift client.
func (c ClientBuildConfigClient) Get(namespace, name string, options metav1.GetOptions) (*buildapi.BuildConfig, error) {
	return c.Client.Build().BuildConfigs(namespace).Get(name, options)
}

// Update updates a BuildConfig using the OpenShift client.
func (c ClientBuildConfigClient) Update(buildConfig *buildapi.BuildConfig) error {
	_, err := c.Client.Build().BuildConfigs(buildConfig.Namespace).Update(buildConfig)
	return err
}

// BuildUpdater provides methods for updating existing Builds.
type BuildUpdater interface {
	Update(namespace string, build *buildapi.Build) error
}

type BuildPatcher interface {
	Patch(namespace, name string, patch []byte) (*buildapi.Build, error)
}

// BuildLister provides methods for listing the Builds.
type BuildLister interface {
	List(namespace string, opts metav1.ListOptions) (*buildapi.BuildList, error)
}

// BuildDeleter knows how to delete builds from OpenShift.
type BuildDeleter interface {
	// DeleteBuild removes the build from OpenShift's storage.
	DeleteBuild(build *buildapi.Build) error
}

// ClientBuildClient delegates build create and update operations to the OpenShift client interface
type ClientBuildClient struct {
	Client builds.Interface
}

// NewClientBuildClient creates a new build client that uses an openshift client to update builds
func NewClientBuildClient(client builds.Interface) *ClientBuildClient {
	return &ClientBuildClient{Client: client}
}

// Update updates builds using the OpenShift client.
func (c ClientBuildClient) Update(namespace string, build *buildapi.Build) error {
	_, e := c.Client.Build().Builds(namespace).Update(build)
	return e
}

// Patch patches builds using the OpenShift client.
func (c ClientBuildClient) Patch(namespace, name string, patch []byte) (*buildapi.Build, error) {
	return c.Client.Build().Builds(namespace).Patch(name, types.StrategicMergePatchType, patch)
}

// List lists the builds using the OpenShift client.
func (c ClientBuildClient) List(namespace string, opts metav1.ListOptions) (*buildapi.BuildList, error) {
	return c.Client.Build().Builds(namespace).List(opts)
}

// DeleteBuild deletes a build from OpenShift.
func (c ClientBuildClient) DeleteBuild(build *buildapi.Build) error {
	return c.Client.Build().Builds(build.Namespace).Delete(build.Name, &metav1.DeleteOptions{})
}

// ClientBuildLister implements the build lister interface over a client
type ClientBuildLister struct {
	client buildclient.BuildsGetter
}

// NewClientBuildClient creates a new build client that uses an openshift client to update builds
func NewClientBuildLister(client buildclient.BuildsGetter) buildlister.BuildLister {
	return &ClientBuildLister{client: client}
}

// List lists the builds using the OpenShift client.
func (c *ClientBuildLister) List(label labels.Selector) ([]*buildapi.Build, error) {
	list, err := c.client.Builds(metav1.NamespaceAll).List(metav1.ListOptions{LabelSelector: label.String()})
	return buildListToPointerArray(list), err
}

func (c *ClientBuildLister) Builds(ns string) buildlister.BuildNamespaceLister {
	return &ClientBuildListerNamespacer{client: c.client, ns: ns}
}

// ClientBuildClientNamespacer implements internalversion lister
type ClientBuildListerNamespacer struct {
	client buildclient.BuildsGetter
	ns     string
}

// List lists the builds using the OpenShift client.
func (c ClientBuildListerNamespacer) List(label labels.Selector) ([]*buildapi.Build, error) {
	list, err := c.client.Builds(c.ns).List(metav1.ListOptions{LabelSelector: label.String()})
	return buildListToPointerArray(list), err
}

func (c ClientBuildListerNamespacer) Get(name string) (*buildapi.Build, error) {
	return c.client.Builds(c.ns).Get(name, metav1.GetOptions{})
}

func buildListToPointerArray(list *buildapi.BuildList) []*buildapi.Build {
	if list == nil {
		return nil
	}
	result := make([]*buildapi.Build, len(list.Items))
	for i := range list.Items {
		result[i] = &list.Items[i]
	}
	return result
}

// ClientBuildLister implements the build lister interface over a client
type ClientBuildConfigLister struct {
	client buildclient.BuildConfigsGetter
}

// NewClientBuildConfigLister creates a new build config client that uses an openshift client.
func NewClientBuildConfigLister(client buildclient.BuildConfigsGetter) buildlister.BuildConfigLister {
	return &ClientBuildConfigLister{client: client}
}

// List lists the builds using the OpenShift client.
func (c *ClientBuildConfigLister) List(label labels.Selector) ([]*buildapi.BuildConfig, error) {
	list, err := c.client.BuildConfigs(metav1.NamespaceAll).List(metav1.ListOptions{LabelSelector: label.String()})
	return buildConfigListToPointerArray(list), err
}

func (c *ClientBuildConfigLister) BuildConfigs(ns string) buildlister.BuildConfigNamespaceLister {
	return &ClientBuildConfigListerNamespacer{client: c.client, ns: ns}
}

// ClientBuildConfigListerNamespacer implements internalversion lister
type ClientBuildConfigListerNamespacer struct {
	client buildclient.BuildConfigsGetter
	ns     string
}

// List lists the builds using the OpenShift client.
func (c ClientBuildConfigListerNamespacer) List(label labels.Selector) ([]*buildapi.BuildConfig, error) {
	list, err := c.client.BuildConfigs(c.ns).List(metav1.ListOptions{LabelSelector: label.String()})
	return buildConfigListToPointerArray(list), err
}

func (c ClientBuildConfigListerNamespacer) Get(name string) (*buildapi.BuildConfig, error) {
	return c.client.BuildConfigs(c.ns).Get(name, metav1.GetOptions{})
}

func buildConfigListToPointerArray(list *buildapi.BuildConfigList) []*buildapi.BuildConfig {
	if list == nil {
		return nil
	}
	result := make([]*buildapi.BuildConfig, len(list.Items))
	for i := range list.Items {
		result[i] = &list.Items[i]
	}
	return result
}

// BuildCloner provides methods for cloning builds
type BuildCloner interface {
	Clone(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error)
}

// OSClientBuildClonerClient creates a new build client that uses an openshift client to clone builds
type ClientBuildClonerClient struct {
	Client builds.Interface
}

// NewOSClientBuildClonerClient creates a new build client that uses an openshift client to clone builds
func NewClientBuildClonerClient(client builds.Interface) *ClientBuildClonerClient {
	return &ClientBuildClonerClient{Client: client}
}

// Clone generates new build for given build name
func (c ClientBuildClonerClient) Clone(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	return c.Client.Build().Builds(namespace).Clone(request.Name, request)
}

// BuildConfigInstantiator provides methods for instantiating builds from build configs
type BuildConfigInstantiator interface {
	Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error)
}

// ClientBuildConfigInstantiatorClient creates a new build client that uses an openshift client to create builds
type ClientBuildConfigInstantiatorClient struct {
	Client builds.Interface
}

// NewClientBuildConfigInstantiatorClient creates a new build client that uses an openshift client to create builds
func NewClientBuildConfigInstantiatorClient(client builds.Interface) *ClientBuildConfigInstantiatorClient {
	return &ClientBuildConfigInstantiatorClient{Client: client}
}

// Instantiate generates new build for given buildConfig
func (c ClientBuildConfigInstantiatorClient) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	return c.Client.Build().BuildConfigs(namespace).Instantiate(request.Name, request)
}

// TODO: Why we need this, seems like an copy of the client above
type BuildConfigInstantiatorClient struct {
	Client buildclient.BuildInterface
}

func (c BuildConfigInstantiatorClient) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	return c.Client.BuildConfigs(namespace).Instantiate(request.Name, request)
}
