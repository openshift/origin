package fake

import (
	internalversion "github.com/openshift/origin/pkg/template/clientset/internalclientset/typed/template/internalversion"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeTemplate struct {
	*core.Fake
}

func (c *FakeTemplate) Templates(namespace string) internalversion.TemplateResourceInterface {
	return &FakeTemplates{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeTemplate) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
