package prune

import (
	"io/ioutil"
	"testing"

	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset/fake"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/fake"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
)

func TestImagePruneNamespaced(t *testing.T) {
	kFake := fake.NewSimpleClientset()
	imageFake := imageclient.NewSimpleClientset()
	opts := &PruneImagesOptions{
		Namespace: "foo",

		AppsClient:  appsclient.NewSimpleClientset().Apps(),
		BuildClient: buildclient.NewSimpleClientset().Build(),
		ImageClient: imageFake.Image(),
		KubeClient:  kFake,
		Out:         ioutil.Discard,
	}

	if err := opts.Run(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(imageFake.Actions()) == 0 || len(kFake.Actions()) == 0 {
		t.Errorf("Missing get images actions")
	}
	for _, a := range imageFake.Actions() {
		// images are non-namespaced
		if a.GetResource().Resource == "images" {
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
