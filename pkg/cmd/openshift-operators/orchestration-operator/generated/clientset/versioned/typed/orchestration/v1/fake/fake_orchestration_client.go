package fake

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/generated/clientset/versioned/typed/orchestration/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeOrchestrationV1 struct {
	*testing.Fake
}

func (c *FakeOrchestrationV1) OpenShiftOrchestrationConfigs() v1.OpenShiftOrchestrationConfigInterface {
	return &FakeOpenShiftOrchestrationConfigs{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeOrchestrationV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
