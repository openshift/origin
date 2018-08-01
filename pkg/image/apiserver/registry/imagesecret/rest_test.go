package imagesecret

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func TestGetSecrets(t *testing.T) {
	fake := fake.NewSimpleClientset(&kapi.SecretList{
		Items: []kapi.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-1", Namespace: "default"},
				Type:       kapi.SecretTypeDockercfg,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-2", Annotations: map[string]string{imageapi.ExcludeImageSecretAnnotation: "true"}, Namespace: "default"},
				Type:       kapi.SecretTypeDockercfg,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-3", Namespace: "default"},
				Type:       kapi.SecretTypeOpaque,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-4", Namespace: "default"},
				Type:       kapi.SecretTypeServiceAccountToken,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-5", Namespace: "default"},
				Type:       kapi.SecretTypeDockerConfigJson,
			},
		},
	})
	rest := NewREST(fake.Core())
	opts, _, _ := rest.NewGetOptions()
	obj, err := rest.Get(apirequest.NewDefaultContext(), "", opts)
	if err != nil {
		t.Fatal(err)
	}
	list := obj.(*kapi.SecretList)
	if len(list.Items) != 2 {
		t.Fatal(list)
	}
	if list.Items[0].Name != "secret-1" || list.Items[1].Name != "secret-5" {
		t.Fatal(list)
	}
}
