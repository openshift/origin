package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	buildv1 "github.com/openshift/api/build/v1"
	buildclient "github.com/openshift/client-go/build/clientset/versioned"
	buildclienttyped "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"
)

// BuildConfigGetter provides methods for getting BuildConfigs
type BuildConfigGetter interface {
	Get(namespace, name string, options metav1.GetOptions) (*buildv1.BuildConfig, error)
}

// BuildConfigUpdater provides methods for updating BuildConfigs
type BuildConfigUpdater interface {
	Update(buildConfig *buildv1.BuildConfig) error
}

// ClientBuildConfigClient delegates get and update operations to the OpenShift client interface
type ClientBuildConfigClient struct {
	Client buildclient.Interface
}

// NewClientBuildConfigClient creates a new build config client that uses an openshift client to create and get BuildConfigs
func NewClientBuildConfigClient(client buildclient.Interface) *ClientBuildConfigClient {
	return &ClientBuildConfigClient{Client: client}
}

// Get returns a BuildConfig using the OpenShift client.
func (c ClientBuildConfigClient) Get(namespace, name string, options metav1.GetOptions) (*buildv1.BuildConfig, error) {
	return c.Client.Build().BuildConfigs(namespace).Get(name, options)
}

// Update updates a BuildConfig using the OpenShift client.
func (c ClientBuildConfigClient) Update(buildConfig *buildv1.BuildConfig) error {
	_, err := c.Client.Build().BuildConfigs(buildConfig.Namespace).Update(buildConfig)
	return err
}

// BuildUpdater provides methods for updating existing Builds.
type BuildUpdater interface {
	Update(namespace string, build *buildv1.Build) error
}

type BuildPatcher interface {
	Patch(namespace, name string, patch []byte) (*buildv1.Build, error)
}

// BuildLister provides methods for listing the Builds.
type BuildLister interface {
	List(namespace string, opts metav1.ListOptions) (*buildv1.BuildList, error)
}

// BuildDeleter knows how to delete buildclient from OpenShift.
type BuildDeleter interface {
	// DeleteBuild removes the build from OpenShift's storage.
	DeleteBuild(build *buildv1.Build) error
}

// ClientBuildClient delegates build create and update operations to the OpenShift client interface
type ClientBuildClient struct {
	Client buildclient.Interface
}

// NewClientBuildClient creates a new build client that uses an openshift client to update buildclient
func NewClientBuildClient(client buildclient.Interface) *ClientBuildClient {
	return &ClientBuildClient{Client: client}
}

// Update updates buildclient using the OpenShift client.
func (c ClientBuildClient) Update(namespace string, build *buildv1.Build) error {
	_, e := c.Client.Build().Builds(namespace).Update(build)
	return e
}

// Patch patches buildclient using the OpenShift client.
func (c ClientBuildClient) Patch(namespace, name string, patch []byte) (*buildv1.Build, error) {
	return c.Client.Build().Builds(namespace).Patch(name, types.StrategicMergePatchType, patch)
}

// List lists the buildclient using the OpenShift client.
func (c ClientBuildClient) List(namespace string, opts metav1.ListOptions) (*buildv1.BuildList, error) {
	return c.Client.Build().Builds(namespace).List(opts)
}

// DeleteBuild deletes a build from OpenShift.
func (c ClientBuildClient) DeleteBuild(build *buildv1.Build) error {
	return c.Client.Build().Builds(build.Namespace).Delete(build.Name, &metav1.DeleteOptions{})
}

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

// BuildCloner provides methods for cloning buildclient
type BuildCloner interface {
	Clone(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error)
}

// OSClientBuildClonerClient creates a new build client that uses an openshift client to clone buildclient
type ClientBuildClonerClient struct {
	Client buildclient.Interface
}

// NewOSClientBuildClonerClient creates a new build client that uses an openshift client to clone buildclient
func NewClientBuildClonerClient(client buildclient.Interface) *ClientBuildClonerClient {
	return &ClientBuildClonerClient{Client: client}
}

// Clone generates new build for given build name
func (c ClientBuildClonerClient) Clone(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error) {
	return c.Client.Build().Builds(namespace).Clone(request.Name, request)
}

// BuildConfigInstantiator provides methods for instantiating buildclient from build configs
type BuildConfigInstantiator interface {
	Instantiate(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error)
}

// ClientBuildConfigInstantiatorClient creates a new build client that uses an openshift client to create buildclient
type ClientBuildConfigInstantiatorClient struct {
	Client buildclient.Interface
}

// NewClientBuildConfigInstantiatorClient creates a new build client that uses an openshift client to create buildclient
func NewClientBuildConfigInstantiatorClient(client buildclient.Interface) *ClientBuildConfigInstantiatorClient {
	return &ClientBuildConfigInstantiatorClient{Client: client}
}

// Instantiate generates new build for given buildConfig
func (c ClientBuildConfigInstantiatorClient) Instantiate(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error) {
	return c.Client.Build().BuildConfigs(namespace).Instantiate(request.Name, request)
}

// TODO: Why we need this, seems like an copy of the client above
type BuildConfigInstantiatorClient struct {
	Client buildclienttyped.BuildV1Interface
}

func (c BuildConfigInstantiatorClient) Instantiate(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error) {
	return c.Client.BuildConfigs(namespace).Instantiate(request.Name, request)
}
