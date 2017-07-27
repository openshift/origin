package prune

import (
	"io/ioutil"
	"testing"

	"github.com/openshift/origin/pkg/client/testclient"
)

func TestBuildPruneNamespaced(t *testing.T) {
	osFake := testclient.NewSimpleFake()
	opts := &PruneBuildsOptions{
		Namespace: "foo",

		OSClient: osFake,
		Out:      ioutil.Discard,
	}

	if err := opts.Run(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(osFake.Actions()) == 0 {
		t.Errorf("Missing get build actions")
	}
	for _, a := range osFake.Actions() {
		if a.GetNamespace() != "foo" {
			t.Errorf("Unexpected namespace while pruning %s: %s", a.GetResource(), a.GetNamespace())
		}
	}
}
