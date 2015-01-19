package rollback

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// REST provides a rollback generation endpoint. Only the Create method is implemented.
type REST struct {
	generator              GeneratorClient
	deploymentGetter       DeploymentGetter
	deploymentConfigGetter DeploymentConfigGetter
	codec                  runtime.Codec
}

// GeneratorClient defines a local interface to a rollback generator for testability.
type GeneratorClient interface {
	Generate(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error)
}

// DeploymentGetter is a local interface to ReplicationControllers for testability.
type DeploymentGetter interface {
	GetDeployment(namespace, name string) (*kapi.ReplicationController, error)
}

// DeploymentConfigGetter is a local interface to DeploymentConfigs for testability.
type DeploymentConfigGetter interface {
	GetDeploymentConfig(namespace, name string) (*deployapi.DeploymentConfig, error)
}

// NewREST safely creates a new REST.
func NewREST(generator GeneratorClient, deploymentGetter DeploymentGetter, configGetter DeploymentConfigGetter, codec runtime.Codec) apiserver.RESTStorage {
	return &REST{
		generator:              generator,
		deploymentGetter:       deploymentGetter,
		deploymentConfigGetter: configGetter,
		codec: codec,
	}
}

func (s *REST) New() runtime.Object {
	return &deployapi.DeploymentConfigRollback{}
}

// Create generates a new DeploymentConfig representing a rollback.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	rollback, ok := obj.(*deployapi.DeploymentConfigRollback)
	if !ok {
		return nil, fmt.Errorf("not a rollback spec: %#v", obj)
	}

	if errs := validation.ValidateDeploymentConfigRollback(rollback); len(errs) > 0 {
		return nil, kerrors.NewInvalid("deploymentConfigRollback", "", errs)
	}

	namespace, namespaceOk := kapi.NamespaceFrom(ctx)
	if !namespaceOk {
		return nil, fmt.Errorf("namespace %s is invalid", ctx.Value)
	}

	// Roll back "from" the current deployment "to" a target deployment
	var from, to *deployapi.DeploymentConfig
	var err error

	// Find the target ("to") deployment and decode the DeploymentConfig
	if targetDeployment, err := s.deploymentGetter.GetDeployment(namespace, rollback.Spec.From.Name); err != nil {
		// TODO: correct error type?
		return nil, kerrors.NewBadRequest(fmt.Sprintf("Couldn't get specified deployment: %v", err))
	} else {
		if to, err = deployutil.DecodeDeploymentConfig(targetDeployment, s.codec); err != nil {
			// TODO: correct error type?
			return nil, kerrors.NewBadRequest(fmt.Sprintf("deploymentConfig on target deployment is invalid: %v", err))
		}
	}

	// Find the current ("from") version of the target deploymentConfig
	if from, err = s.deploymentConfigGetter.GetDeploymentConfig(namespace, to.Name); err != nil {
		// TODO: correct error type?
		return nil, kerrors.NewBadRequest(fmt.Sprintf("Couldn't find current deploymentConfig %s/%s: %v", namespace, to.Name, err))
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return s.generator.Generate(from, to, &rollback.Spec)
	}), nil
}
