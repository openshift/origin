package etcd

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/scale/scheme/autoscalingv1"
	"k8s.io/client-go/scale/scheme/extensionsv1beta1"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	autoscalingvalidation "k8s.io/kubernetes/pkg/apis/autoscaling/validation"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/labels"

	appsapiv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/apps/registry/deployconfig"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST contains the REST storage for DeploymentConfig objects.
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}
var _ rest.ShortNamesProvider = &REST{}
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"dc"}
}

// NewREST returns a deploymentConfigREST containing the REST storage for DeploymentConfig objects,
// a statusREST containing the REST storage for changing the status of a DeploymentConfig,
// and a scaleREST containing the REST storage for the Scale subresources of DeploymentConfigs.
func NewREST(optsGetter restoptions.Getter) (*REST, *StatusREST, *ScaleREST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &appsapi.DeploymentConfig{} },
		NewListFunc:              func() runtime.Object { return &appsapi.DeploymentConfigList{} },
		DefaultQualifiedResource: appsapi.Resource("deploymentconfigs"),

		CreateStrategy: deployconfig.GroupStrategy,
		UpdateStrategy: deployconfig.GroupStrategy,
		DeleteStrategy: deployconfig.GroupStrategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, nil, err
	}

	deploymentConfigREST := &REST{store}

	statusStore := *store
	statusStore.UpdateStrategy = deployconfig.StatusStrategy
	statusREST := &StatusREST{store: &statusStore}

	scaleREST := &ScaleREST{store: store}

	return deploymentConfigREST, statusREST, scaleREST, nil
}

// ScaleREST contains the REST storage for the Scale subresource of DeploymentConfigs.
type ScaleREST struct {
	store *registry.Store
}

var _ = rest.Patcher(&ScaleREST{})
var _ = rest.GroupVersionKindProvider(&ScaleREST{})

// New creates a new Scale object
func (r *ScaleREST) New() runtime.Object {
	return &autoscaling.Scale{}
}

func (r *ScaleREST) GroupVersionKind(containingGV schema.GroupVersion) schema.GroupVersionKind {
	switch containingGV {
	case appsapiv1.SchemeGroupVersion,
		appsapiv1.LegacySchemeGroupVersion:
		return extensionsv1beta1.SchemeGroupVersion.WithKind("Scale")
	default:
		return autoscalingv1.SchemeGroupVersion.WithKind("Scale")
	}
}

// Get retrieves (computes) the Scale subresource for the given DeploymentConfig name.
func (r *ScaleREST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	deploymentConfig, err := r.store.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}

	return scaleFromConfig(deploymentConfig.(*appsapi.DeploymentConfig)), nil
}

// Update scales the DeploymentConfig for the given Scale subresource, returning the updated Scale.
func (r *ScaleREST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	uncastObj, err := r.store.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, false, errors.NewNotFound(extensions.Resource("scale"), name)
	}
	deploymentConfig := uncastObj.(*appsapi.DeploymentConfig)

	old := scaleFromConfig(deploymentConfig)
	obj, err := objInfo.UpdatedObject(ctx, old)
	if err != nil {
		return nil, false, err
	}

	scale, ok := obj.(*autoscaling.Scale)
	if !ok {
		return nil, false, errors.NewBadRequest(fmt.Sprintf("wrong object passed to Scale update: %v", obj))
	}

	if errs := autoscalingvalidation.ValidateScale(scale); len(errs) > 0 {
		return nil, false, errors.NewInvalid(extensions.Kind("Scale"), scale.Name, errs)
	}

	deploymentConfig.Spec.Replicas = scale.Spec.Replicas
	if _, _, err := r.store.Update(ctx, deploymentConfig.Name, rest.DefaultUpdatedObjectInfo(deploymentConfig), createValidation, updateValidation); err != nil {
		return nil, false, err
	}

	return scale, false, nil
}

// scaleFromConfig builds a scale resource out of a deployment config.
func scaleFromConfig(dc *appsapi.DeploymentConfig) *autoscaling.Scale {
	// We need to make sure that the implicit selector won't have invalid value specified by user.
	// Should be fixed globally in https://github.com/openshift/origin/pull/18640
	selector := map[string]string{}
	// Copy the map not to pollute the one on DC
	for k, v := range dc.Spec.Selector {
		selector[k] = v
	}
	selector[appsapi.DeploymentConfigLabel] = dc.Name

	return &autoscaling.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Name:              dc.Name,
			Namespace:         dc.Namespace,
			UID:               dc.UID,
			ResourceVersion:   dc.ResourceVersion,
			CreationTimestamp: dc.CreationTimestamp,
		},
		Spec: autoscaling.ScaleSpec{
			Replicas: dc.Spec.Replicas,
		},
		Status: autoscaling.ScaleStatus{
			Replicas: dc.Status.Replicas,
			Selector: labels.Set(selector).String(),
		},
	}
}

// StatusREST implements the REST endpoint for changing the status of a DeploymentConfig.
type StatusREST struct {
	store *registry.Store
}

// StatusREST implements Patcher
var _ = rest.Patcher(&StatusREST{})

func (r *StatusREST) New() runtime.Object {
	return &appsapi.DeploymentConfig{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an deploymentConfig.
func (r *StatusREST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation)
}

// LegacyREST allows us to wrap and alter some behavior
type LegacyREST struct {
	*REST
}

func (r *LegacyREST) Categories() []string {
	return []string{}
}
