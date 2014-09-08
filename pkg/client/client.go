package client

import (
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/v1beta1"
)

// Interface exposes methods on OpenShift resources.
type Interface interface {
	BuildInterface
}

// BuildInterface exposes methods on Build resources.
type BuildInterface interface {
	ListBuilds(selector labels.Selector) (buildapi.BuildList, error)
	UpdateBuild(buildapi.Build) (buildapi.Build, error)
}

// Client is an OpenShift client object
type Client struct {
	*kubeclient.RESTClient
}

// New creates and returns a new Client.
func New(host string, auth *kubeclient.AuthInfo) (*Client, error) {
	restClient, err := kubeclient.NewRESTClient(host, auth, "/osapi/v1beta1")
	if err != nil {
		return nil, err
	}
	return &Client{restClient}, nil
}

// ListBuilds returns a list of builds.
func (c *Client) ListBuilds(selector labels.Selector) (result buildapi.BuildList, err error) {
	err = c.Get().Path("builds").SelectorParam("labels", selector).Do().Into(&result)
	return
}

// UpdateBuild updates an existing build.
func (c *Client) UpdateBuild(build buildapi.Build) (result buildapi.Build, err error) {
	err = c.Put().Path("builds").Path(build.ID).Body(build).Do().Into(&result)
	return
}
