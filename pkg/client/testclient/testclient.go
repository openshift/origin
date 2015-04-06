package testclient

import (
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	"github.com/openshift/origin/pkg/api/latest"
	osclient "github.com/openshift/origin/pkg/client"
)

func NewFixtureClients(o testclient.ObjectRetriever) (osclient.Interface, kclient.Interface) {
	osc := &osclient.Fake{
		ReactFn: testclient.ObjectReaction(o, latest.RESTMapper),
	}
	kcc := &testclient.Fake{
		ReactFn: testclient.ObjectReaction(o, latest.RESTMapper),
	}
	return osc, kcc
}
