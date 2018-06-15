package rollback

import (
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/apps/apis/apps/validation"
	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	apps "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

// REST provides a rollback generation endpoint. Only the Create method is implemented.
type REST struct {
	generator RollbackGenerator
	dn        apps.DeploymentConfigsGetter
	rn        kcoreclient.ReplicationControllersGetter
	codec     runtime.Codec
}

var _ rest.Creater = &REST{}

// NewREST safely creates a new REST.
func NewREST(appsclient appsclientinternal.Interface, kc kclientset.Interface, codec runtime.Codec) *REST {
	return &REST{
		generator: NewRollbackGenerator(),
		dn:        appsclient.Apps(),
		rn:        kc.Core(),
		codec:     codec,
	}
}

// New creates an empty DeploymentConfigRollback resource
func (r *REST) New() runtime.Object {
	return &appsapi.DeploymentConfigRollback{}
}

// Create generates a new DeploymentConfig representing a rollback.
func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, kerrors.NewBadRequest("namespace parameter required.")
	}
	rollback, ok := obj.(*appsapi.DeploymentConfigRollback)
	if !ok {
		return nil, kerrors.NewBadRequest(fmt.Sprintf("not a rollback spec: %#v", obj))
	}

	if errs := validation.ValidateDeploymentConfigRollback(rollback); len(errs) > 0 {
		return nil, kerrors.NewInvalid(appsapi.Kind("DeploymentConfigRollback"), rollback.Name, errs)
	}
	if err := createValidation(obj); err != nil {
		return nil, err
	}

	from, err := r.dn.DeploymentConfigs(namespace).Get(rollback.Name, metav1.GetOptions{})
	if err != nil {
		return nil, newInvalidError(rollback, fmt.Sprintf("cannot get deployment config %q: %v", rollback.Name, err))
	}

	switch from.Status.LatestVersion {
	case 0:
		return nil, newInvalidError(rollback, "cannot rollback an undeployed config")
	case 1:
		return nil, newInvalidError(rollback, fmt.Sprintf("no previous deployment exists for %q", appsutil.LabelForDeploymentConfig(from)))
	case rollback.Spec.Revision:
		return nil, newInvalidError(rollback, fmt.Sprintf("version %d is already the latest", rollback.Spec.Revision))
	}

	revision := from.Status.LatestVersion - 1
	if rollback.Spec.Revision > 0 {
		revision = rollback.Spec.Revision
	}

	// Find the target deployment and decode its config.
	name := appsutil.DeploymentNameForConfigVersion(from.Name, revision)
	targetDeployment, err := r.rn.ReplicationControllers(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, newInvalidError(rollback, err.Error())
	}

	to, err := appsutil.DecodeDeploymentConfig(targetDeployment, r.codec)
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

func newInvalidError(rollback *appsapi.DeploymentConfigRollback, reason string) error {
	err := field.Invalid(field.NewPath("name"), rollback.Name, reason)
	return kerrors.NewInvalid(appsapi.Kind("DeploymentConfigRollback"), rollback.Name, field.ErrorList{err})
}
