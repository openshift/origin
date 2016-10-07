package prune

import (
	"io/ioutil"
	"testing"

	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	"github.com/openshift/origin/pkg/client/testclient"
)

func TestImagePruneNamespaced(t *testing.T) {
	kFake := ktestclient.NewSimpleFake()
	osFake := testclient.NewSimpleFake()
	opts := &PruneImagesOptions{
		Namespace: "foo",

		OSClient: osFake,
		KClient:  kFake,
		Out:      ioutil.Discard,
	}

	if err := opts.Run(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(osFake.Actions()) == 0 || len(kFake.Actions()) == 0 {
		t.Errorf("Missing get images actions")
	}
	for _, a := range osFake.Actions() {
		// images are non-namespaced
		if a.GetResource() == "images" {
			continue
		}
		if a.GetNamespace() != "foo" {
			t.Errorf("Unexpected namespace while pruning %s: %s", a.GetResource(), a.GetNamespace())
		}
	}
	for _, a := range kFake.Actions() {
		if a.GetNamespace() != "foo" {
			t.Errorf("Unexpected namespace while pruning %s: %s", a.GetResource(), a.GetNamespace())
		}
	}
}
