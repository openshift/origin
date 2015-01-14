package etcd

import (
	"github.com/golang/glog"

	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	ktools "github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/secret/api"
)

const (
	// SecretPath is the path to secret resources in etcd
	SecretPath string = "/secrets"
)

// Etcd implements secret.Registry and secretconfig.Registry interfaces.
type Etcd struct {
	tools.EtcdHelper
}

// New creates an etcd registry.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		EtcdHelper: helper,
	}
}

// ListSecrets obtains a list of Secrets.
func (r *Etcd) ListSecrets(ctx kapi.Context, label, field labels.Selector) (*api.SecretList, error) {
	secrets := api.SecretList{}
	err := r.ExtractToList(makeSecretListKey(ctx), &secrets)
	if err != nil {
		return nil, err
	}

	filtered := []api.Secret{}
	for _, item := range secrets.Items {
		fields := labels.Set{
			"name": item.Name,
			"type": string(item.Type),
		}
		if label.Matches(labels.Set(item.Labels)) && field.Matches(fields) {
			filtered = append(filtered, item)
		}
	}

	secrets.Items = filtered
	return &secrets, err
}

func makeSecretListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, SecretPath)
}

func makeSecretKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, SecretPath, id)
}

// GetSecret gets a specific Secret specified by its Name.
func (r *Etcd) GetSecret(ctx kapi.Context, name string) (*api.Secret, error) {
	var secret api.Secret
	key, err := makeSecretKey(ctx, name)
	if err != nil {
		return nil, err
	}
	err = r.ExtractObj(key, &secret, false)
	if err != nil {
		return nil, etcderr.InterpretGetError(err, "secret", name)
	}
	return &secret, nil
}

// CreateSecret creates a new Secret.
func (r *Etcd) CreateSecret(ctx kapi.Context, secret *api.Secret) error {
	key, err := makeSecretKey(ctx, secret.Name)
	if err != nil {
		return err
	}
	err = r.CreateObj(key, secret, 0)
	return etcderr.InterpretCreateError(err, "secret", secret.Name)
}

// UpdateSecret replaces an existing Secret.
func (r *Etcd) UpdateSecret(ctx kapi.Context, secret *api.Secret) error {
	key, err := makeSecretKey(ctx, secret.Name)
	if err != nil {
		return err
	}
	err = r.SetObj(key, secret)
	return etcderr.InterpretUpdateError(err, "secret", secret.Name)
}

// DeleteSecret deletes a Secret specified by its ID.
func (r *Etcd) DeleteSecret(ctx kapi.Context, id string) error {
	key, err := makeSecretKey(ctx, id)
	if err != nil {
		return err
	}
	err = r.Delete(key, false)
	return etcderr.InterpretDeleteError(err, "secret", id)
}

// WatchSecrets begins watching for new, changed, or deleted Secrets.
func (r *Etcd) WatchSecrets(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := ktools.ParseWatchResourceVersion(resourceVersion, "secret")
	if err != nil {
		return nil, err
	}

	return r.WatchList(makeSecretListKey(ctx), version, func(obj runtime.Object) bool {
		secret, ok := obj.(*api.Secret)
		if !ok {
			glog.Errorf("Unexpected object during secret watch: %#v", obj)
			return false
		}
		fields := labels.Set{
			"name": secret.Name,
			"type": string(secret.Type),
		}
		return label.Matches(labels.Set(secret.Labels)) && field.Matches(fields)
	})
}
