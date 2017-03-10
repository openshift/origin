package imagesecret

import (
	"fmt"

	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	"github.com/openshift/origin/pkg/image/api"
)

// REST implements the RESTStorage interface for ImageStreamImport
type REST struct {
	secrets kcoreclient.SecretsGetter
}

// NewREST returns a new REST.
func NewREST(secrets kcoreclient.SecretsGetter) *REST {
	return &REST{secrets: secrets}
}

func (r *REST) New() runtime.Object {
	return &kapi.SecretList{}
}

func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &metainternal.ListOptions{}, false, ""
}

// Get retrieves all pull type secrets in the current namespace. Name is currently ignored and
// reserved for future use.
func (r *REST) Get(ctx apirequest.Context, _ string, options runtime.Object) (runtime.Object, error) {
	listOptions, ok := options.(*metainternal.ListOptions)
	if !ok {
		return nil, fmt.Errorf("unexpected options: %v", listOptions)
	}
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		ns = metav1.NamespaceAll
	}
	secrets, err := r.secrets.Secrets(ns).List(*listOptions)
	if err != nil {
		return nil, err
	}
	filtered := make([]kapi.Secret, 0, len(secrets.Items))
	for i := range secrets.Items {
		if secrets.Items[i].Annotations[api.ExcludeImageSecretAnnotation] == "true" {
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
