package rollback

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// REST provides a rollback generation endpoint. Only the Create method is implemented.
type REST struct {
	generator RollbackGenerator
	dn        client.DeploymentConfigsNamespacer
	rn        kclient.ReplicationControllersNamespacer
	codec     runtime.Codec
}

// NewREST safely creates a new REST.
func NewREST(oc client.Interface, kc kclient.Interface, codec runtime.Codec) *REST {
	return &REST{
		generator: NewRollbackGenerator(),
		dn:        oc,
		rn:        kc,
		codec:     codec,
	}
}

// New creates an empty DeploymentConfigRollback resource
func (r *REST) New() runtime.Object {
	return &deployapi.DeploymentConfigRollback{}
}

// Create generates a new DeploymentConfig representing a rollback.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	namespace, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, kerrors.NewBadRequest("namespace parameter required.")
	}
	rollback, ok := obj.(*deployapi.DeploymentConfigRollback)
	if !ok {
		return nil, kerrors.NewBadRequest(fmt.Sprintf("not a rollback spec: %#v", obj))
	}

	if errs := validation.ValidateDeploymentConfigRollback(rollback); len(errs) > 0 {
		return nil, kerrors.NewInvalid(deployapi.Kind("DeploymentConfigRollback"), rollback.Name, errs)
	}

	from, err := r.dn.DeploymentConfigs(namespace).Get(rollback.Name)
	if err != nil {
		return nil, newInvalidError(rollback, fmt.Sprintf("cannot get deployment config %q: %v", rollback.Name, err))
	}

	switch from.Status.LatestVersion {
	case 0:
		return nil, newInvalidError(rollback, "cannot rollback an undeployed config")
	case 1:
		return nil, newInvalidError(rollback, fmt.Sprintf("no previous deployment exists for %q", deployutil.LabelForDeploymentConfig(from)))
	}

	revision := from.Status.LatestVersion - 1
	if rollback.Spec.Revision > 0 {
		revision = rollback.Spec.Revision
	}

	// Find the target deployment and decode its config.
	name := deployutil.DeploymentNameForConfigVersion(from.Name, revision)
	targetDeployment, err := r.rn.ReplicationControllers(namespace).Get(name)
	if err != nil {
		return nil, newInvalidError(rollback, err.Error())
	}

	to, err := deployutil.DecodeDeploymentConfig(targetDeployment, r.codec)
	if err != nil {
		return nil, newInvalidError(rollback, fmt.Sprintf("couldn't decode deployment config from deployment: %v", err))
	}

	if from.Annotations == nil && len(rollback.UpdatedAnnotations) > 0 {
		from.Annotations = make(map[string]string)
	}
	for key, value := range rollback.UpdatedAnnotations {
		from.Annotations[key] = value
	}

	return r.generator.GenerateRollback(from, to, &rollback.Spec)
}

func newInvalidError(rollback *deployapi.DeploymentConfigRollback, reason string) error {
	err := field.Invalid(field.NewPath("name"), rollback.Name, reason)
	return kerrors.NewInvalid(deployapi.Kind("DeploymentConfigRollback"), rollback.Name, field.ErrorList{err})
}
