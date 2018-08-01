package deployments

import (
	"testing"

	fakecorev1client "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	fakeappsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1/fake"
)

func TestDeploymentPruneNamespaced(t *testing.T) {
	osFake := &fakeappsv1client.FakeAppsV1{Fake: &clienttesting.Fake{}}
	coreFake := &fakecorev1client.FakeCoreV1{Fake: &clienttesting.Fake{}}
	opts := &PruneDeploymentsOptions{
		Namespace: "foo",

		AppsClient: osFake,
		KubeClient: coreFake,
		IOStreams:  genericclioptions.NewTestIOStreamsDiscard(),
	}

	if err := opts.Run(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(osFake.Actions()) == 0 || len(coreFake.Actions()) == 0 {
		t.Errorf("Missing get deployments actions")
	}
	for _, a := range osFake.Actions() {
		if a.GetNamespace() != "foo" {
			t.Errorf("Unexpected namespace while pruning %s: %s", a.GetResource(), a.GetNamespace())
		}
	}
	for _, a := range coreFake.Actions() {
		if a.GetNamespace() != "foo" {
			t.Errorf("Unexpected namespace while pruning %s: %s", a.GetResource(), a.GetNamespace())
		}
	}
}
