package testclient

import (
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"

	"github.com/openshift/origin/pkg/api/latest"
	osclient "github.com/openshift/origin/pkg/client"
)

// NewFixtureClients returns mocks of the OpenShift and Kubernetes clients
func NewFixtureClients(o testclient.ObjectRetriever) (osclient.Interface, kclient.Interface) {
	oc := &Fake{}
	oc.AddReactor("*", "*", testclient.ObjectReaction(o, latest.RESTMapper))

	kc := &testclient.Fake{}
	kc.AddReactor("*", "*", testclient.ObjectReaction(o, latest.RESTMapper))

	return oc, kc
}
