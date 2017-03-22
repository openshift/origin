package fake

import (
	v1 "github.com/openshift/origin/pkg/template/clientset/release_v3_6/typed/template/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	core "k8s.io/kubernetes/pkg/client/testing/core"
)

type FakeTemplateV1 struct {
	*core.Fake
}

func (c *FakeTemplateV1) Templates(namespace string) v1.TemplateResourceInterface {
	return &FakeTemplates{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeTemplateV1) RESTClient() restclient.Interface {
	var ret *restclient.RESTClient
	return ret
}
