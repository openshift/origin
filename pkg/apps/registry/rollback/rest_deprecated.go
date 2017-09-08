package rollback

import (
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/apps/apis/apps/validation"
	deployclient "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	deployutil "github.com/openshift/origin/pkg/apps/util"
)

// REST provides a rollback generation endpoint. Only the Create method is implemented.
type DeprecatedREST struct {
	generator GeneratorClient
	codec     runtime.Codec
}

var _ rest.Creater = &REST{}

// GeneratorClient defines a local interface to a rollback generator for testability.
type GeneratorClient interface {
	GenerateRollback(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error)
	GetDeployment(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error)
	GetDeploymentConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error)
}

type Client struct {
	GRFn                        func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error)
	DeploymentConfigGetter      deployclient.DeploymentConfigsGetter
	ReplicationControllerGetter coreclient.ReplicationControllersGetter
}

func (c Client) GetDeploymentConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
	return c.DeploymentConfigGetter.DeploymentConfigs(apirequest.NamespaceValue(ctx)).Get(name, *options)
}

func (c Client) GetDeployment(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error) {
	return c.ReplicationControllerGetter.ReplicationControllers(apirequest.NamespaceValue(ctx)).Get(name, *options)
}

func (c Client) GenerateRollback(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
	return c.GRFn(from, to, spec)
}

// NewDeprecatedREST safely creates a new REST.
func NewDeprecatedREST(generator GeneratorClient, codec runtime.Codec) *DeprecatedREST {
	return &DeprecatedREST{
		generator: generator,
		codec:     codec,
	}
}

// New creates an empty DeploymentConfigRollback resource
func (s *DeprecatedREST) New() runtime.Object {
	return &deployapi.DeploymentConfigRollback{}
}

// Create generates a new DeploymentConfig representing a rollback.
func (s *DeprecatedREST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	rollback, ok := obj.(*deployapi.DeploymentConfigRollback)
	if !ok {
		return nil, kerrors.NewBadRequest(fmt.Sprintf("not a rollback spec: %#v", obj))
	}

	if errs := validation.ValidateDeploymentConfigRollbackDeprecated(rollback); len(errs) > 0 {
		return nil, kerrors.NewInvalid(deployapi.Kind("DeploymentConfigRollback"), "", errs)
	}

	// Roll back "from" the current deployment "to" a target deployment

	// Find the target ("to") deployment and decode the DeploymentConfig
	targetDeployment, err := s.generator.GetDeployment(ctx, rollback.Spec.From.Name, &metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, newInvalidDeploymentError(rollback, "Deployment not found")
		}
		return nil, newInvalidDeploymentError(rollback, fmt.Sprintf("%v", err))
	}

	to, err := deployutil.DecodeDeploymentConfig(targetDeployment, s.codec)
	if err != nil {
		return nil, newInvalidDeploymentError(rollback,
			fmt.Sprintf("couldn't decode DeploymentConfig from Deployment: %v", err))
	}

	// Find the current ("from") version of the target deploymentConfig
	from, err := s.generator.GetDeploymentConfig(ctx, to.Name, &metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, newInvalidDeploymentError(rollback,
				fmt.Sprintf("couldn't find a current DeploymentConfig %s/%s", targetDeployment.Namespace, to.Name))
		}
		return nil, newInvalidDeploymentError(rollback,
			fmt.Sprintf("error finding current DeploymentConfig %s/%s: %v", targetDeployment.Namespace, to.Name, err))
	}

	return s.generator.GenerateRollback(from, to, &rollback.Spec)
}

func newInvalidDeploymentError(rollback *deployapi.DeploymentConfigRollback, reason string) error {
	err := field.Invalid(field.NewPath("spec").Child("from").Child("name"), rollback.Spec.From.Name, reason)
	return kerrors.NewInvalid(deployapi.Kind("DeploymentConfigRollback"), "", field.ErrorList{err})
}
