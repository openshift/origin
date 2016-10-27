package etcd

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/apis/extensions"
	extvalidation "k8s.io/kubernetes/pkg/apis/extensions/validation"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST contains the REST storage for DeploymentConfig objects.
type REST struct {
	*registry.Store
}

// NewREST returns a deploymentConfigREST containing the REST storage for DeploymentConfig objects,
// a statusREST containing the REST storage for changing the status of a DeploymentConfig,
// and a scaleREST containing the REST storage for the Scale subresources of DeploymentConfigs.
func NewREST(optsGetter restoptions.Getter) (*REST, *StatusREST, *ScaleREST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.DeploymentConfig{} },
		NewListFunc:       func() runtime.Object { return &api.DeploymentConfigList{} },
		QualifiedResource: api.Resource("deploymentconfigs"),
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.DeploymentConfig).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
			return deployconfig.Matcher(label, field)
		},
		CreateStrategy:      deployconfig.Strategy,
		UpdateStrategy:      deployconfig.Strategy,
		DeleteStrategy:      deployconfig.Strategy,
		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, true, storage.NoTriggerPublisher); err != nil {
		return nil, nil, nil, err
	}

	deploymentConfigREST := &REST{store}
	statusStore := *store
	statusStore.UpdateStrategy = deployconfig.StatusStrategy
	statusREST := &StatusREST{store: &statusStore}
	scaleREST := &ScaleREST{registry: deployconfig.NewRegistry(deploymentConfigREST)}

	return deploymentConfigREST, statusREST, scaleREST, nil
}

// ScaleREST contains the REST storage for the Scale subresource of DeploymentConfigs.
type ScaleREST struct {
	registry deployconfig.Registry
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
