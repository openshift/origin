package builds

import (
	"testing"

	clienttesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	fakebuildv1client "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1/fake"
)

func TestBuildPruneNamespaced(t *testing.T) {
	osFake := &fakebuildv1client.FakeBuildV1{Fake: &clienttesting.Fake{}}
	opts := &PruneBuildsOptions{
		Namespace: "foo",

		BuildClient: osFake,
		IOStreams:   genericclioptions.NewTestIOStreamsDiscard(),
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
