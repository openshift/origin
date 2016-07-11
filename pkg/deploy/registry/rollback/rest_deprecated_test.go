package rollback

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestCreateErrorDepr(t *testing.T) {
	rest := DeprecatedREST{}
	obj, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfig{})

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Errorf("Unexpected non-nil object: %#v", obj)
	}
}

func TestCreateInvalidDepr(t *testing.T) {
	rest := DeprecatedREST{}
	obj, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{})

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Errorf("Unexpected non-nil object: %#v", obj)
	}
}

func TestCreateOkDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				return &deployapi.DeploymentConfig{}, nil
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
				return deployment, nil
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
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

func TestCreateGeneratorErrorDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("something terrible happened"))
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
				return deployment, nil
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
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

func TestCreateMissingDeploymentDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound(kapi.Resource("replicationController"), name)
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				namespace, _ := kapi.NamespaceFrom(ctx)
				t.Fatalf("unexpected call to GetDeploymentConfig(%s/%s)", namespace, name)
				return nil, kerrors.NewNotFound(deployapi.Resource("deploymentConfig"), name)
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
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

func TestCreateInvalidDeploymentDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				// invalidate the encoded config
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
				deployment.Annotations[deployapi.DeploymentEncodedConfigAnnotation] = ""
				return deployment, nil
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				namespace, _ := kapi.NamespaceFrom(ctx)
				t.Fatalf("unexpected call to GetDeploymentConfig(%s/%s)", namespace, name)
				return nil, kerrors.NewNotFound(deployapi.Resource("deploymentConfig"), name)
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
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

func TestCreateMissingDeploymentConfigDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: Client{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
				return deployment, nil
			},
			DCFn: func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewNotFound(deployapi.Resource("deploymentConfig"), name)
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
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

func TestNewDepr(t *testing.T) {
	// :)
	rest := NewDeprecatedREST(Client{}, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
	rest.New()
}
