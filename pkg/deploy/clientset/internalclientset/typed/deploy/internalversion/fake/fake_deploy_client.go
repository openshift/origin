package fake

import (
	internalversion "github.com/openshift/origin/pkg/deploy/clientset/internalclientset/typed/deploy/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeDeploy struct {
	*core.Fake
}

func (c *FakeDeploy) DeploymentConfigs(namespace string) internalversion.DeploymentConfigInterface {
	return &FakeDeploymentConfigs{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeDeploy) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
