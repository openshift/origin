package imagesecret

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
	corev1conversion "k8s.io/kubernetes/pkg/apis/core/v1"

	imagev1 "github.com/openshift/api/image/v1"
)

// REST implements the RESTStorage interface for ImageStreamImport
type REST struct {
	secrets corev1client.SecretsGetter
}

var _ rest.GetterWithOptions = &REST{}

// NewREST returns a new REST.
func NewREST(secrets corev1client.SecretsGetter) *REST {
	return &REST{secrets: secrets}
}

func (r *REST) New() runtime.Object {
	return &coreapi.SecretList{}
}

func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &metav1.ListOptions{}, false, ""
}

// Get retrieves all pull type secrets in the current namespace. Name is currently ignored and
// reserved for future use.
func (r *REST) Get(ctx context.Context, _ string, options runtime.Object) (runtime.Object, error) {
	listOptions, ok := options.(*metav1.ListOptions)
	if !ok {
		return nil, fmt.Errorf("unexpected options: %T", options)
	}
	var opts metav1.ListOptions
	if listOptions != nil {
		opts = *listOptions
	}
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		ns = metav1.NamespaceAll
	}
	secrets, err := r.secrets.Secrets(ns).List(opts)
	if err != nil {
		return nil, err
	}
	filtered := make([]coreapi.Secret, 0, len(secrets.Items))
	for i := range secrets.Items {
		if secrets.Items[i].Annotations[imagev1.ExcludeImageSecretAnnotation] == "true" {
			continue
		}
		switch secrets.Items[i].Type {
		case corev1.SecretTypeDockercfg, corev1.SecretTypeDockerConfigJson:
			internalSecret := &coreapi.Secret{}
			if err := corev1conversion.Convert_v1_Secret_To_core_Secret(&secrets.Items[i], internalSecret, nil); err != nil {
				return nil, err
			}
			filtered = append(filtered, *internalSecret)
		}
	}
	// clear the external content and convert
	secrets.Items = nil
	internalSecretList := &coreapi.SecretList{}
	if err := corev1conversion.Convert_v1_SecretList_To_core_SecretList(secrets, internalSecretList, nil); err != nil {
		return nil, err
	}
	internalSecretList.Items = filtered
	return internalSecretList, nil
}
