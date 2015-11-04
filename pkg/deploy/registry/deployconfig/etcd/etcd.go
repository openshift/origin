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

const DeploymentConfigPath string = "/deploymentconfigs"

// DeploymentConfigStorage contains the REST storage information for both DeploymentConfigs
// and their Scale subresources.
type DeploymentConfigStorage struct {
	DeploymentConfig *REST
	Scale            *ScaleREST
}

// NewStorage returns a DeploymentConfigStorage containing the REST storage for
// DeploymentConfig objects and their Scale subresources.
func NewStorage(s storage.Interface, rcNamespacer kclient.ReplicationControllersNamespacer) DeploymentConfigStorage {
	deploymentConfigREST := newREST(s)
	deploymentConfigRegistry := deployconfig.NewRegistry(deploymentConfigREST)
	return DeploymentConfigStorage{
		DeploymentConfig: deploymentConfigREST,
		Scale: &ScaleREST{
			registry:     deploymentConfigRegistry,
			rcNamespacer: rcNamespacer,
		},
	}
}

// REST contains the REST storage for DeploymentConfig objects.
type REST struct {
	*etcdgeneric.Etcd
}

// newREST returns a RESTStorage object that will work against DeploymentConfig objects.
func newREST(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:      func() runtime.Object { return &api.DeploymentConfig{} },
		NewListFunc:  func() runtime.Object { return &api.DeploymentConfigList{} },
		EndpointName: "deploymentConfig",
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, DeploymentConfigPath)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, DeploymentConfigPath, id)
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

	return &REST{store}
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

	scaleRet := &extensions.Scale{
		ObjectMeta: kapi.ObjectMeta{
			Name:              name,
			Namespace:         deploymentConfig.Namespace,
			CreationTimestamp: deploymentConfig.CreationTimestamp,
		},
		Spec: extensions.ScaleSpec{
			Replicas: deploymentConfig.Template.ControllerTemplate.Replicas,
		},
		Status: extensions.ScaleStatus{
			Replicas: totalReplicas,
			Selector: deploymentConfig.Template.ControllerTemplate.Selector,
		},
	}

	// current replicas reflects either the scale of the current deployment,
	// or the scale of the RC template if no current deployment exists
	controller, err := r.rcNamespacer.ReplicationControllers(deploymentConfig.Namespace).Get(util.LatestDeploymentNameForConfig(deploymentConfig))
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		return scaleRet, nil

	}

	scaleRet.Spec.Replicas = controller.Spec.Replicas
	return scaleRet, nil

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

	// fake an existing object to validate
	existing := &extensions.Scale{
		ObjectMeta: kapi.ObjectMeta{
			Name:              scale.Name,
			CreationTimestamp: scale.CreationTimestamp,
		},
	}

	if existing.Namespace, ok = kapi.NamespaceFrom(ctx); !ok {
		existing.Namespace = scale.Namespace
	}

	if errs := extvalidation.ValidateScaleUpdate(scale, existing); len(errs) > 0 {
		return nil, false, errors.NewInvalid("scale", scale.Name, errs)
	}

	deploymentConfig, err := r.registry.GetDeploymentConfig(ctx, scale.Name)
	if err != nil {
		return nil, false, errors.NewNotFound("scale", scale.Name)
	}

	// This code tries to mitigate any race conditions caused by the DC controller
	// not actually basing RC replicas on the template after the first deploy.
	//
	// If the DC controller isn't doing anything, and isn't about to start anything,
	// then we'll have updated the DC or deployment RC correctly (the easy case)
	//
	// If we are dealing with a deployment, the following happens on updates to resources:
	// 1. The DC controller catches an update to a DC, and creates a new RC, naming it based on the value of dc.LatestVersion
	// 2. The Deployment Controller then picks up a change to the RC, launches a deployer pod, and updates the status annotation of the RC to pending
	// 3a. The Deployer Pod Controller picks up the change to the pod, and updates the status annotation of the RC to running
	// 3b. The deployer pod itself runs the deployment strategy
	// 4. the deployer pod finishes or errors out, and the Deployer Pod Controller picks this up and updates the status annotation of the RC to either complete or failed
	//
	// We try to refuse to scale while a deployment is in progress, based on the annotation of the latest RC.
	// Unfortunately, we can have a case where we pick up a DC, calculate the name of the latest DC using its
	// LatestVersion field, and then someone updates that before twe run the scale.  In this case, it is possible
	// that we may try to scale the wrong RC.  However, since /scale is used mainly by the HPA,
	// we should see the updated values and annotations the next HPA cycle, and cease the incorrect behavior.

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
			Replicas: 0,
			Selector: deploymentConfig.Template.ControllerTemplate.Selector,
		},
	}

	// this tries to update the current deployment RC first, since updating
	// Replicas on a DeploymentConfig doesn't do anything after the first deployment
	// has been made
	controller, err := r.rcNamespacer.ReplicationControllers(deploymentConfig.Namespace).Get(util.LatestDeploymentNameForConfig(deploymentConfig))
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, false, err
		}

		deploymentConfig.Template.ControllerTemplate.Replicas = scale.Spec.Replicas
		if err = r.registry.UpdateDeploymentConfig(ctx, deploymentConfig); err != nil {
			return nil, false, err
		}

		return scaleRet, false, nil
	}

	if deploymentStatus := util.DeploymentStatusFor(controller); deploymentStatus != api.DeploymentStatusComplete {
		return nil, false, err
	}

	// TODO(directxman12): this is going to be a bit out of sync, since we are calculating it
	// here and not as part of the deploymentconfig loop -- is there a better way of doing it?
	totalReplicas, err := r.replicasForDeploymentConfig(deploymentConfig.Namespace, deploymentConfig.Name)
	if err != nil {
		return nil, false, err
	}

	oldReplicas := controller.Spec.Replicas
	controller.Spec.Replicas = scale.Spec.Replicas
	if _, err = r.rcNamespacer.ReplicationControllers(deploymentConfig.Namespace).Update(controller); err != nil {
		return nil, false, err
	}
	scaleRet.Status.Replicas = totalReplicas + (scale.Spec.Replicas - oldReplicas)

	return scaleRet, false, nil
}

func (r *ScaleREST) deploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error) {
	selector := util.ConfigSelector(configName)
	return r.rcNamespacer.ReplicationControllers(namespace).List(selector, fields.Everything())
}

func (r *ScaleREST) replicasForDeploymentConfig(namespace, configName string) (int, error) {
	rcList, err := r.deploymentsForConfig(namespace, configName)
	if err != nil {
		return 0, err
	}

	replicas := 0
	for _, rc := range rcList.Items {
		replicas += rc.Spec.Replicas
	}

	return replicas, nil
}
