package rollback

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestCreateError(t *testing.T) {
	rest := REST{}
	obj, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfig{})

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Errorf("Unexpected non-nil object: %#v", obj)
	}
}

func TestCreateInvalid(t *testing.T) {
	rest := REST{}
	obj, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{})

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Errorf("Unexpected non-nil object: %#v", obj)
	}
}

func TestCreateOk(t *testing.T) {
	rest := REST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				return &deployapi.DeploymentConfig{}, nil
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
				return deployment, nil
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		codec: api.Codec,
	}

	obj, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: kapi.NamespaceDefault,
			},
		},
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if obj == nil {
		t.Errorf("Expected a result obj")
	}

	if _, ok := obj.(*deployapi.DeploymentConfig); !ok {
		t.Errorf("expected a DeploymentConfig, got a %#v", obj)
	}
}

func TestCreateGeneratorError(t *testing.T) {
	rest := REST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("something terrible happened"))
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
				return deployment, nil
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		codec: api.Codec,
	}

	_, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: kapi.NamespaceDefault,
			},
		},
	})

	if err == nil || !strings.Contains(err.Error(), "something terrible happened") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCreateMissingDeployment(t *testing.T) {
	rest := REST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound("replicationController", name)
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				namespace, _ := kapi.NamespaceFrom(ctx)
				t.Fatalf("unexpected call to GetDeploymentConfig(%s/%s)", namespace, name)
				return nil, kerrors.NewNotFound("deploymentConfig", name)
			},
		},
		codec: api.Codec,
	}

	obj, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: kapi.NamespaceDefault,
			},
		},
	})

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}

func TestCreateInvalidDeployment(t *testing.T) {
	rest := REST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				// invalidate the encoded config
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
				deployment.Annotations[deployapi.DeploymentEncodedConfigAnnotation] = ""
				return deployment, nil
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				namespace, _ := kapi.NamespaceFrom(ctx)
				t.Fatalf("unexpected call to GetDeploymentConfig(%s/%s)", namespace, name)
				return nil, kerrors.NewNotFound("deploymentConfig", name)
			},
		},
		codec: api.Codec,
	}

	obj, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: kapi.NamespaceDefault,
			},
		},
	})

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}

func TestCreateMissingDeploymentConfig(t *testing.T) {
	rest := REST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
				return deployment, nil
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewNotFound("deploymentConfig", name)
			},
		},
		codec: api.Codec,
	}

	obj, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: kapi.NamespaceDefault,
			},
		},
	})

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}

func TestNew(t *testing.T) {
	// :)
	rest := NewREST(Client{}, api.Codec)
	rest.New()
}
