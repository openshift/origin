package imagesecret

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
)

// REST implements the RESTStorage interface for ImageStreamImport
type REST struct {
	secrets client.SecretsNamespacer
}

// NewREST returns a new REST.
func NewREST(secrets client.SecretsNamespacer) *REST {
	return &REST{secrets: secrets}
}

func (r *REST) New() runtime.Object {
	return &kapi.SecretList{}
}

func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &kapi.ListOptions{}, false, ""
}

// Get retrieves all pull type secrets in the current namespace. Name is currently ignored and
// reserved for future use.
func (r *REST) Get(ctx kapi.Context, _ string, options runtime.Object) (runtime.Object, error) {
	listOptions, ok := options.(*kapi.ListOptions)
	if !ok {
		return nil, fmt.Errorf("unexpected options: %v", listOptions)
	}
	ns, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		ns = kapi.NamespaceAll
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
