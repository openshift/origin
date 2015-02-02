package rollback

import (
	"errors"
	"fmt"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

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

	channel, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
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

	if channel == nil {
		t.Errorf("Expected a result channel")
	}

	select {
	case result := <-channel:
		if _, ok := result.Object.(*deployapi.DeploymentConfig); !ok {
			t.Errorf("expected a DeploymentConfig, got a %#v", result.Object)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
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

	channel, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
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

	if channel == nil {
		t.Errorf("Expected a result channel")
	}

	select {
	case result := <-channel:
		status, ok := result.Object.(*kapi.Status)
		if !ok {
			t.Errorf("Expected status, got %#v", result)
		}
		if status.Status != kapi.StatusFailure {
			t.Errorf("Expected status=failure, message=foo, got %#v", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
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

	channel, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
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

	if channel != nil {
		t.Error("Unexpected result channel")
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

	channel, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
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

	if channel != nil {
		t.Error("Unexpected result channel")
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

	channel, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
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

	if channel != nil {
		t.Error("Unexpected result channel")
	}
}

func TestNew(t *testing.T) {
	// :)
	rest := NewREST(Client{}, api.Codec)
	rest.New()
}
