package testclient

import (
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"

	osclient "github.com/openshift/origin/pkg/client"
)

// NewFixtureClients returns mocks of the OpenShift and Kubernetes clients
func NewFixtureClients(o testclient.ObjectRetriever) (osclient.Interface, kclient.Interface) {
	oc := &Fake{}
	oc.AddReactor("*", "*", testclient.ObjectReaction(o, registered.RESTMapper()))

	kc := &testclient.Fake{}
	kc.AddReactor("*", "*", testclient.ObjectReaction(o, registered.RESTMapper()))

	return oc, kc
}
