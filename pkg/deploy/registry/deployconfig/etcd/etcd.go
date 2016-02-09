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
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	"github.com/openshift/origin/pkg/deploy/util"
	extvalidation "k8s.io/kubernetes/pkg/apis/extensions/validation"
)

// REST contains the REST storage for DeploymentConfig objects.
type REST struct {
	*etcdgeneric.Etcd
}

// NewStorage returns a DeploymentConfigStorage containing the REST storage for
// DeploymentConfig objects and their Scale subresources.
func NewREST(s storage.Interface, rcNamespacer kclient.ReplicationControllersNamespacer) (*REST, *ScaleREST) {
	prefix := "/deploymentconfigs"

	store := &etcdgeneric.Etcd{
		NewFunc:           func() runtime.Object { return &api.DeploymentConfig{} },
		NewListFunc:       func() runtime.Object { return &api.DeploymentConfigList{} },
		QualifiedResource: api.Resource("deploymentconfigs"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, id)
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
		Storage:             s,
	}

	deploymentConfigREST := &REST{store}
	scaleREST := &ScaleREST{
		registry:     deployconfig.NewRegistry(deploymentConfigREST),
		rcNamespacer: rcNamespacer,
	}

	return deploymentConfigREST, scaleREST
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

	// TODO(directxman12): this is going to be a bit out of sync, since we are calculating it
	// here and not as part of the deploymentconfig loop -- is there a better way of doing it?
	totalReplicas, err := r.replicasForDeploymentConfig(deploymentConfig.Namespace, deploymentConfig.Name)
	if err != nil {
		return nil, err
	}

	return &extensions.Scale{
		ObjectMeta: kapi.ObjectMeta{
			Name:              name,
			Namespace:         deploymentConfig.Namespace,
			CreationTimestamp: deploymentConfig.CreationTimestamp,
		},
		Spec: extensions.ScaleSpec{
			Replicas: deploymentConfig.Spec.Replicas,
		},
		Status: extensions.ScaleStatus{
			Replicas: totalReplicas,
			Selector: deploymentConfig.Spec.Selector,
		},
	}, nil
}

// Update scales the DeploymentConfig for the given Scale subresource, returning the updated Scale.
func (r *ScaleREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	if obj == nil {
		return nil, false, errors.NewBadRequest(fmt.Sprintf("nil update passed to Scale"))
	}
	scale, ok := obj.(*extensions.Scale)
	if !ok {
		return nil, false, errors.NewBadRequest(fmt.Sprintf("wrong object passed to Scale update: %v", obj))
	}

	if errs := extvalidation.ValidateScale(scale); len(errs) > 0 {
		return nil, false, errors.NewInvalid(extensions.Kind("Scale"), scale.Name, errs)
	}

	deploymentConfig, err := r.registry.GetDeploymentConfig(ctx, scale.Name)
	if err != nil {
		return nil, false, errors.NewNotFound(extensions.Resource("scale"), scale.Name)
	}

	scaleRet := &extensions.Scale{
		ObjectMeta: kapi.ObjectMeta{
			Name:              deploymentConfig.Name,
			Namespace:         deploymentConfig.Namespace,
			CreationTimestamp: deploymentConfig.CreationTimestamp,
		},
		Spec: extensions.ScaleSpec{
			Replicas: scale.Spec.Replicas,
		},
		Status: extensions.ScaleStatus{
			Selector: deploymentConfig.Spec.Selector,
		},
	}

	// TODO(directxman12): this is going to be a bit out of sync, since we are calculating it
	// here and not as part of the deploymentconfig loop -- is there a better way of doing it?
	totalReplicas, err := r.replicasForDeploymentConfig(deploymentConfig.Namespace, deploymentConfig.Name)
	if err != nil {
		return nil, false, err
	}

	oldReplicas := deploymentConfig.Spec.Replicas
	deploymentConfig.Spec.Replicas = scale.Spec.Replicas
	if err := r.registry.UpdateDeploymentConfig(ctx, deploymentConfig); err != nil {
		return nil, false, err
	}
	scaleRet.Status.Replicas = totalReplicas + (scale.Spec.Replicas - oldReplicas)

	return scaleRet, false, nil
}

func (r *ScaleREST) replicasForDeploymentConfig(namespace, configName string) (int, error) {
	options := kapi.ListOptions{LabelSelector: util.ConfigSelector(configName)}
	rcList, err := r.rcNamespacer.ReplicationControllers(namespace).List(options)
	if err != nil {
		return 0, err
	}

	replicas := 0
	for _, rc := range rcList.Items {
		replicas += rc.Spec.Replicas
	}

	return replicas, nil
}
