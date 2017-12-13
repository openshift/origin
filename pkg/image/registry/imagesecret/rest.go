package imagesecret

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// REST implements the RESTStorage interface for ImageStreamImport
type REST struct {
	secrets kcoreclient.SecretsGetter
}

var _ rest.GetterWithOptions = &REST{}

// NewREST returns a new REST.
func NewREST(secrets kcoreclient.SecretsGetter) *REST {
	return &REST{secrets: secrets}
}

func (r *REST) New() runtime.Object {
	return &kapi.SecretList{}
}

func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &metav1.ListOptions{}, false, ""
}

// Get retrieves all pull type secrets in the current namespace. Name is currently ignored and
// reserved for future use.
func (r *REST) Get(ctx apirequest.Context, _ string, options runtime.Object) (runtime.Object, error) {
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
	filtered := make([]kapi.Secret, 0, len(secrets.Items))
	for i := range secrets.Items {
		if secrets.Items[i].Annotations[imageapi.ExcludeImageSecretAnnotation] == "true" {
			continue
		}
		switch secrets.Items[i].Type {
		case kapi.SecretTypeDockercfg, kapi.SecretTypeDockerConfigJson:
			filtered = append(filtered, secrets.Items[i])
		}
	}
	secrets.Items = filtered
	return secrets, nil
}
