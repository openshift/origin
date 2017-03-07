package prune

import (
	"io/ioutil"
	"testing"

	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
)

func TestJobPruneNamespaced(t *testing.T) {
	kFake := ktestclient.NewSimpleFake()
	opts := &PruneJobsOptions{
		Namespace: "foo",

		KClient: kFake,
		Out:     ioutil.Discard,
	}

	if err := opts.Run(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(kFake.Actions()) == 0 {
		t.Errorf("Missing get jobs actions")
	}
	for _, a := range kFake.Actions() {
		if a.GetNamespace() != "foo" {
			t.Errorf("Unexpected namespace while pruning %s: %s", a.GetResource(), a.GetNamespace())
		}
	}
}
