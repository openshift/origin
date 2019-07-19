package imagesecret

import (
	"testing"

	imagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/fake"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
)

func TestGetSecrets(t *testing.T) {
	fake := fake.NewSimpleClientset(&corev1.SecretList{
		Items: []corev1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-1", Namespace: "default"},
				Type:       corev1.SecretTypeDockercfg,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-2", Annotations: map[string]string{imagev1.ExcludeImageSecretAnnotation: "true"}, Namespace: "default"},
				Type:       corev1.SecretTypeDockercfg,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-3", Namespace: "default"},
				Type:       corev1.SecretTypeOpaque,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-4", Namespace: "default"},
				Type:       corev1.SecretTypeServiceAccountToken,
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-5", Namespace: "default"},
				Type:       corev1.SecretTypeDockerConfigJson,
			},
		},
	})
	rest := NewREST(fake.CoreV1())
	opts, _, _ := rest.NewGetOptions()
	obj, err := rest.Get(apirequest.NewDefaultContext(), "", opts)
	if err != nil {
		t.Fatal(err)
	}
	list := obj.(*coreapi.SecretList)
	if len(list.Items) != 2 {
		t.Fatal(list)
	}
	if list.Items[0].Name != "secret-1" || list.Items[1].Name != "secret-5" {
		t.Fatal(list)
	}
}
