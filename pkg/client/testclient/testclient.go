package testclient

import (
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	"github.com/openshift/origin/pkg/api/latest"
	osclient "github.com/openshift/origin/pkg/client"
)

// NewFixtureClients returns mocks of the OpenShift and Kubernetes clients
func NewFixtureClients(o testclient.ObjectRetriever) (osclient.Interface, kclient.Interface) {
	oc := &Fake{
		ReactFn: testclient.ObjectReaction(o, latest.RESTMapper),
	}
	kc := &testclient.Fake{
		ReactFn: testclient.ObjectReaction(o, latest.RESTMapper),
	}
	return oc, kc
}
