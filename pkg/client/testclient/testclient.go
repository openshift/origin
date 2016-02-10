package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"

	osclient "github.com/openshift/origin/pkg/client"
)

// NewFixtureClients returns mocks of the OpenShift and Kubernetes clients
func NewFixtureClients(o testclient.ObjectRetriever) (osclient.Interface, kclient.Interface) {
	oc := &Fake{}
	oc.AddReactor("*", "*", testclient.ObjectReaction(o, kapi.RESTMapper))

	kc := &testclient.Fake{}
	kc.AddReactor("*", "*", testclient.ObjectReaction(o, kapi.RESTMapper))

	return oc, kc
}
