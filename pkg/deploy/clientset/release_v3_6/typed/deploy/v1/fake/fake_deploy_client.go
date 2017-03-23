package fake

import (
	v1 "github.com/openshift/origin/pkg/deploy/clientset/release_v3_6/typed/deploy/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeDeployV1 struct {
	*core.Fake
}

func (c *FakeDeployV1) DeploymentConfigs(namespace string) v1.DeploymentConfigInterface {
	return &FakeDeploymentConfigs{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeDeployV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
