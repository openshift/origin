package secret

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/secret/api"
	"github.com/openshift/origin/pkg/secret/api/validation"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry Registry
}

// NewREST creates a new REST backed by the given registry.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{
		registry: registry,
	}
}

// New creates a new Secret for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.Secret{}
}

// List obtains a list of Secrets that match selector.
func (s *REST) List(ctx kapi.Context, label, field labels.Selector) (runtime.Object, error) {
	secrets, err := s.registry.ListSecrets(ctx, label, field)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}

// Get obtains the Secret specified by its name.
func (s *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	secret, err := s.registry.GetSecret(ctx, name)
	if err != nil {
		return nil, err
	}
	return secret, err
}

// Delete asynchronously deletes the Secret specified by its name.
func (s *REST) Delete(ctx kapi.Context, name string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteSecret(ctx, name)
	}), nil
}

// Create registers a given new Secret instance to s.registry.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	secret, ok := obj.(*api.Secret)
	if !ok {
		return nil, fmt.Errorf("not a secret: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &secret.ObjectMeta) {
		return nil, kerrors.NewConflict("secret", secret.Namespace, fmt.Errorf("Secret.Namespace does not match the provided context"))
	}

	kapi.FillObjectMetaSystemFields(ctx, &secret.ObjectMeta)

	if len(secret.Name) == 0 {
		secret.Name = uuid.NewUUID().String()
	}

	glog.Infof("Creating secret with namespace::Name: %v::%v", secret.Namespace, secret.Name)

	if errs := validation.ValidateSecret(secret); len(errs) > 0 {
		return nil, kerrors.NewInvalid("secret", secret.Name, errs)
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.CreateSecret(ctx, secret)
		if err != nil {
			return nil, err
		}
		return secret, nil
	}), nil
}

// Update replaces a given Secret instance with an existing instance in s.registry.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	secret, ok := obj.(*api.Secret)
	if !ok {
		return nil, fmt.Errorf("not a secret: %#v", obj)
	}
	if len(secret.Name) == 0 {
		return nil, fmt.Errorf("name is unspecified: %#v", secret)
	}
	if !kapi.ValidNamespace(ctx, &secret.ObjectMeta) {
		return nil, kerrors.NewConflict("secret", secret.Namespace, fmt.Errorf("Secret.Namespace does not match the provided context"))
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.UpdateSecret(ctx, secret)
		if err != nil {
			return nil, err
		}
		return secret, nil
	}), nil
}

// Watch sets up a watch on secrets with a specific selector
func (s *REST) Watch(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return s.registry.WatchSecrets(ctx, label, field, resourceVersion)
}
