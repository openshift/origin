package client

import (
	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

// BuildConfigGetter provides methods for getting BuildConfigs
type BuildConfigGetter interface {
	Get(namespace, name string) (*buildapi.BuildConfig, error)
}

// BuildConfigUpdater provides methods for updating BuildConfigs
type BuildConfigUpdater interface {
	Update(buildConfig *buildapi.BuildConfig) error
}

// BuildConfigDeleter provides methods for deleting BuildConfigs
type BuildConfigDeleter interface {
	Delete(namespace, name string) error
}

// OSClientBuildConfigClient delegates get and update operations to the OpenShift client interface
type OSClientBuildConfigClient struct {
	Client osclient.Interface
}

// NewOSClientBuildConfigClient creates a new build config client that uses an openshift client to create and get BuildConfigs
func NewOSClientBuildConfigClient(client osclient.Interface) *OSClientBuildConfigClient {
	return &OSClientBuildConfigClient{Client: client}
}

// Get returns a BuildConfig using the OpenShift client.
func (c OSClientBuildConfigClient) Get(namespace, name string) (*buildapi.BuildConfig, error) {
	return c.Client.BuildConfigs(namespace).Get(name)
}

// Update updates a BuildConfig using the OpenShift client.
func (c OSClientBuildConfigClient) Update(buildConfig *buildapi.BuildConfig) error {
	_, err := c.Client.BuildConfigs(buildConfig.Namespace).Update(buildConfig)
	return err
}

// Delete deletes a BuildConfig using the OpenShift client.
func (c OSClientBuildConfigClient) Delete(namespace, name string) error {
	return c.Client.BuildConfigs(namespace).Delete(name)
}

// BuildUpdater provides methods for updating existing Builds.
type BuildUpdater interface {
	Update(namespace string, build *buildapi.Build) error
}

// BuildDeleter provides methods for deleting Builds.
type BuildDeleter interface {
	Delete(namespace, name string) error
}

// BuildLister provides methods for listing Builds.
type BuildLister interface {
	List(namespace string, label labels.Selector, field fields.Selector) (*buildapi.BuildList, error)
}

// OSClientBuildClient deletes build create and update operations to the OpenShift client interface
type OSClientBuildClient struct {
	Client osclient.Interface
}

// NewOSClientBuildClient creates a new build client that uses an openshift client to update builds
func NewOSClientBuildClient(client osclient.Interface) *OSClientBuildClient {
	return &OSClientBuildClient{Client: client}
}

// Update updates builds using the OpenShift client.
func (c OSClientBuildClient) Update(namespace string, build *buildapi.Build) error {
	_, e := c.Client.Builds(namespace).Update(build)
	return e
}

// Delete deletes builds using the OpenShift client.
func (c OSClientBuildClient) Delete(namespace, name string) error {
	return c.Client.Builds(namespace).Delete(name)
}

// List lists builds using the OpenShift client.
func (c OSClientBuildClient) List(namespace string, label labels.Selector, field fields.Selector) (*buildapi.BuildList, error) {
	return c.Client.Builds(namespace).List(label, field)
}

// BuildCloner provides methods for cloning builds
type BuildCloner interface {
	Clone(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error)
}

// OSClientBuildClonerClient creates a new build client that uses an openshift client to clone builds
type OSClientBuildClonerClient struct {
	Client osclient.Interface
}

// NewOSClientBuildClonerClient creates a new build client that uses an openshift client to clone builds
func NewOSClientBuildClonerClient(client osclient.Interface) *OSClientBuildClonerClient {
	return &OSClientBuildClonerClient{Client: client}
}

// Clone generates new build for given build name
func (c OSClientBuildClonerClient) Clone(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	return c.Client.Builds(namespace).Clone(request)
}

// BuildConfigInstantiator provides methods for instantiating builds from build configs
type BuildConfigInstantiator interface {
	Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error)
}

// OSClientBuildConfigInstantiatorClient creates a new build client that uses an openshift client to create builds
type OSClientBuildConfigInstantiatorClient struct {
	Client osclient.Interface
}

// NewOSClientBuildConfigInstantiatorClient creates a new build client that uses an openshift client to create builds
func NewOSClientBuildConfigInstantiatorClient(client osclient.Interface) *OSClientBuildConfigInstantiatorClient {
	return &OSClientBuildConfigInstantiatorClient{Client: client}
}

// Instantiate generates new build for given buildConfig
func (c OSClientBuildConfigInstantiatorClient) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	return c.Client.BuildConfigs(namespace).Instantiate(request)
}
