package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// FakeImageStreamSecrets implements ImageStreamSecretInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamSecrets struct {
	Fake      *Fake
	Namespace string
}

var _ client.ImageStreamSecretInterface = &FakeImageStreamSecrets{}

func (c *FakeImageStreamSecrets) Secrets(name string, options metav1.ListOptions) (*kapi.SecretList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(imageapi.SchemeGroupVersion.WithResource("imagestreams/secrets"), c.Namespace, name), &kapi.SecretList{})
	if obj == nil {
		return nil, err
	}
	return obj.(*kapi.SecretList), err
}
