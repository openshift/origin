package client

import (
	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
)

// BuildConfigGetter provides methods for getting BuildConfigs
type BuildConfigGetter interface {
	Get(namespace, name string) (*buildapi.BuildConfig, error)
}

// BuildConfigUpdater provides methods for updating BuildConfigs
type BuildConfigUpdater interface {
	Update(buildConfig *buildapi.BuildConfig) error
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

// BuildCreator provides methods for creating new Builds
type BuildCreator interface {
	Create(namespace string, build *buildapi.Build) error
}

// BuildUpdater provides methods for updating existing Builds.
type BuildUpdater interface {
	Update(namespace string, build *buildapi.Build) error
}

// OSClientBuildClient deletes build create and update operations to the OpenShift client interface
type OSClientBuildClient struct {
	Client osclient.Interface
}

// NewOSClientBuildClient creates a new build client that uses an openshift client to create builds
func NewOSClientBuildClient(client osclient.Interface) *OSClientBuildClient {
	return &OSClientBuildClient{Client: client}
}

// Create creates builds using the OpenShift client.
func (c OSClientBuildClient) Create(namespace string, build *buildapi.Build) error {
	_, e := c.Client.Builds(namespace).Create(build)
	return e
}

// Update updates builds using the OpenShift client.
func (c OSClientBuildClient) Update(namespace string, build *buildapi.Build) error {
	_, e := c.Client.Builds(namespace).Update(build)
	return e
}
