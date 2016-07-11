package etcd

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	"github.com/openshift/origin/pkg/util/restoptions"
	extvalidation "k8s.io/kubernetes/pkg/apis/extensions/validation"
)

// REST contains the REST storage for DeploymentConfig objects.
type REST struct {
	*registry.Store
}

// NewStorage returns a DeploymentConfigStorage containing the REST storage for
// DeploymentConfig objects and their Scale subresources.
func NewREST(optsGetter restoptions.Getter, rcNamespacer kclient.ReplicationControllersNamespacer) (*REST, *StatusREST, *ScaleREST, error) {
	prefix := "/deploymentconfigs"

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.DeploymentConfig{} },
		NewListFunc:       func() runtime.Object { return &api.DeploymentConfigList{} },
		QualifiedResource: api.Resource("deploymentconfigs"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return registry.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return registry.NamespaceKeyFunc(ctx, prefix, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.DeploymentConfig).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return deployconfig.Matcher(label, field)
		},
		CreateStrategy:      deployconfig.Strategy,
		UpdateStrategy:      deployconfig.Strategy,
		DeleteStrategy:      deployconfig.Strategy,
		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, prefix); err != nil {
		return nil, nil, nil, err
	}

	deploymentConfigREST := &REST{store}
	statusStore := *store
	statusStore.UpdateStrategy = deployconfig.StatusStrategy
	statusREST := &StatusREST{store: &statusStore}
	scaleREST := &ScaleREST{
		registry:     deployconfig.NewRegistry(deploymentConfigREST),
		rcNamespacer: rcNamespacer,
	}

	return deploymentConfigREST, statusREST, scaleREST, nil
}

// ScaleREST contains the REST storage for the Scale subresource of DeploymentConfigs.
type ScaleREST struct {
	registry     deployconfig.Registry
	rcNamespacer kclient.ReplicationControllersNamespacer
}

// ScaleREST implements Patcher
var _ = rest.Patcher(&ScaleREST{})

// New creates a new Scale object
func (r *ScaleREST) New() runtime.Object {
	return &extensions.Scale{}
}

// Get retrieves (computes) the Scale subresource for the given DeploymentConfig name.
func (r *ScaleREST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	deploymentConfig, err := r.registry.GetDeploymentConfig(ctx, name)
	if err != nil {
		return nil, err
	}

	return api.ScaleFromConfig(deploymentConfig), nil
}

// Update scales the DeploymentConfig for the given Scale subresource, returning the updated Scale.
func (r *ScaleREST) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	deploymentConfig, err := r.registry.GetDeploymentConfig(ctx, name)
	if err != nil {
		return nil, false, errors.NewNotFound(extensions.Resource("scale"), name)
	}

	old := api.ScaleFromConfig(deploymentConfig)
	obj, err := objInfo.UpdatedObject(ctx, old)
	if err != nil {
		return nil, false, err
	}

	scale, ok := obj.(*extensions.Scale)
	if !ok {
		return nil, false, errors.NewBadRequest(fmt.Sprintf("wrong object passed to Scale update: %v", obj))
	}

	if errs := extvalidation.ValidateScale(scale); len(errs) > 0 {
		return nil, false, errors.NewInvalid(extensions.Kind("Scale"), scale.Name, errs)
	}

	deploymentConfig.Spec.Replicas = scale.Spec.Replicas
	if err := r.registry.UpdateDeploymentConfig(ctx, deploymentConfig); err != nil {
		return nil, false, err
	}

	return scale, false, nil
}

// StatusREST implements the REST endpoint for changing the status of a DeploymentConfig.
type StatusREST struct {
	store *registry.Store
}

// StatusREST implements the Updater interface.
var _ = rest.Updater(&StatusREST{})

func (r *StatusREST) New() runtime.Object {
	return &api.DeploymentConfig{}
}

// Update alters the status subset of an deploymentConfig.
func (r *StatusREST) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}
