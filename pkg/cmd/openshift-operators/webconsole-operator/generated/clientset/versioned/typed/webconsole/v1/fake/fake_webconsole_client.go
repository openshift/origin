package fake

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/generated/clientset/versioned/typed/webconsole/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeWebconsoleV1 struct {
	*testing.Fake
}

func (c *FakeWebconsoleV1) OpenShiftWebConsoleConfigs() v1.OpenShiftWebConsoleConfigInterface {
	return &FakeOpenShiftWebConsoleConfigs{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeWebconsoleV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
