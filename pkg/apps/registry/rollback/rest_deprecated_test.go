package rollback

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	deploytest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	deployv1 "github.com/openshift/origin/pkg/apps/apis/apps/v1"
	deployutil "github.com/openshift/origin/pkg/apps/util"
)

func TestCreateErrorDepr(t *testing.T) {
	rest := DeprecatedREST{}
	obj, err := rest.Create(apirequest.NewDefaultContext(), &deployapi.DeploymentConfig{}, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Errorf("Unexpected non-nil object: %#v", obj)
	}
}

func TestCreateInvalidDepr(t *testing.T) {
	rest := DeprecatedREST{}
	obj, err := rest.Create(apirequest.NewDefaultContext(), &deployapi.DeploymentConfigRollback{}, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Errorf("Unexpected non-nil object: %#v", obj)
	}
}

func TestCreateOkDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: TestClient{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				return &deployapi.DeploymentConfig{}, nil
			},
			RCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
				return deployment, nil
			},
			DCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
	}

	obj, err := rest.Create(apirequest.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: metav1.NamespaceDefault,
			},
		},
	}, false)

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
		generator: TestClient{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("something terrible happened"))
			},
			RCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
				return deployment, nil
			},
			DCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
				return deploytest.OkDeploymentConfig(1), nil
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
	}

	_, err := rest.Create(apirequest.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: metav1.NamespaceDefault,
			},
		},
	}, false)

	if err == nil || !strings.Contains(err.Error(), "something terrible happened") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCreateMissingDeploymentDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: TestClient{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewNotFound(kapi.Resource("replicationController"), name)
			},
			DCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
				namespace, _ := apirequest.NamespaceFrom(ctx)
				t.Fatalf("unexpected call to GetDeploymentConfig(%s/%s)", namespace, name)
				return nil, kerrors.NewNotFound(deployapi.Resource("deploymentConfig"), name)
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
	}

	obj, err := rest.Create(apirequest.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: metav1.NamespaceDefault,
			},
		},
	}, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}

func TestCreateInvalidDeploymentDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: TestClient{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error) {
				// invalidate the encoded config
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
				deployment.Annotations[deployapi.DeploymentEncodedConfigAnnotation] = ""
				return deployment, nil
			},
			DCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
				namespace, _ := apirequest.NamespaceFrom(ctx)
				t.Fatalf("unexpected call to GetDeploymentConfig(%s/%s)", namespace, name)
				return nil, kerrors.NewNotFound(deployapi.Resource("deploymentConfig"), name)
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
	}

	obj, err := rest.Create(apirequest.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: metav1.NamespaceDefault,
			},
		},
	}, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}

func TestCreateMissingDeploymentConfigDepr(t *testing.T) {
	rest := DeprecatedREST{
		generator: TestClient{
			GRFn: func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
				t.Fatal("unexpected call to generator")
				return nil, errors.New("something terrible happened")
			},
			RCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error) {
				deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
				return deployment, nil
			},
			DCFn: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
				return nil, kerrors.NewNotFound(deployapi.Resource("deploymentConfig"), name)
			},
		},
		codec: kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
	}

	obj, err := rest.Create(apirequest.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: metav1.NamespaceDefault,
			},
		},
	}, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}

func TestNewDepr(t *testing.T) {
	// :)
	rest := NewDeprecatedREST(TestClient{}, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
	rest.New()
}

// Client provides an implementation of Generator client
type TestClient struct {
	GRFn func(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error)
	RCFn func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error)
	DCFn func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error)
}

// GetDeployment returns the deploymentConfig with the provided context and name
func (c TestClient) GetDeploymentConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
	return c.DCFn(ctx, name, options)
}

// GetDeployment returns the deployment with the provided context and name
func (c TestClient) GetDeployment(ctx apirequest.Context, name string, options *metav1.GetOptions) (*kapi.ReplicationController, error) {
	return c.RCFn(ctx, name, options)
}

// GenerateRollback generates a new deploymentConfig by merging a pair of deploymentConfigs
func (c TestClient) GenerateRollback(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
	return c.GRFn(from, to, spec)
}
