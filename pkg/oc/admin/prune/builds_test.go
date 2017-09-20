package prune

import (
	"io/ioutil"
	"testing"

	buildfake "github.com/openshift/origin/pkg/build/generated/internalclientset/fake"
)

func TestBuildPruneNamespaced(t *testing.T) {
	osFake := buildfake.NewSimpleClientset()
	opts := &PruneBuildsOptions{
		Namespace: "foo",

		BuildClient: osFake,
		Out:         ioutil.Discard,
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
