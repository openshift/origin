package rollback

import (
	"errors"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

var codec = kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion)

type terribleGenerator struct{}

func (tg *terribleGenerator) GenerateRollback(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
	return nil, kerrors.NewInternalError(errors.New("something terrible happened"))
}

var _ RollbackGenerator = &terribleGenerator{}

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
	oc := &testclient.Fake{}
	oc.AddReactor("get", "deploymentconfigs", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, deploytest.OkDeploymentConfig(2), nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		return true, deployment, nil
	})

	obj, err := NewREST(oc, kc, codec).Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Name: "config",
		Spec: deployapi.DeploymentConfigRollbackSpec{
			Revision: 1,
		},
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if obj == nil {
		t.Errorf("Expected a result obj")
	}

	if _, ok := obj.(*deployapi.DeploymentConfig); !ok {
		t.Errorf("expected a deployment config, got a %#v", obj)
	}
}

func TestCreateRollbackToLatest(t *testing.T) {
	oc := &testclient.Fake{}
	oc.AddReactor("get", "deploymentconfigs", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, deploytest.OkDeploymentConfig(2), nil
	})

	_, err := NewREST(oc, &fake.Clientset{}, codec).Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Name: "config",
		Spec: deployapi.DeploymentConfigRollbackSpec{
			Revision: 2,
		},
	})

	if err == nil {
		t.Errorf("expected an error when rolling back to the existing deployed revision")
	}
	if err != nil && !strings.Contains(err.Error(), "version 2 is already the latest") {
		t.Errorf("unexpected error received: %v", err)
	}
}

func TestCreateGeneratorError(t *testing.T) {
	oc := &testclient.Fake{}
	oc.AddReactor("get", "deploymentconfigs", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, deploytest.OkDeploymentConfig(2), nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		return true, deployment, nil
	})

	rest := REST{
		generator: &terribleGenerator{},
		dn:        oc,
		rn:        kc.Core(),
		codec:     kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion),
	}

	_, err := rest.Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Name: "config",
		Spec: deployapi.DeploymentConfigRollbackSpec{
			Revision: 1,
		},
	})

	if err == nil || !strings.Contains(err.Error(), "something terrible happened") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCreateMissingDeployment(t *testing.T) {
	oc := &testclient.Fake{}
	oc.AddReactor("get", "deploymentconfigs", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, deploytest.OkDeploymentConfig(2), nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		return true, nil, kerrors.NewNotFound(kapi.Resource("replicationController"), deployment.Name)
	})

	obj, err := NewREST(oc, kc, codec).Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Name: "config",
		Spec: deployapi.DeploymentConfigRollbackSpec{
			Revision: 1,
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
	oc := &testclient.Fake{}
	oc.AddReactor("get", "deploymentconfigs", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, deploytest.OkDeploymentConfig(2), nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		// invalidate the encoded config
		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		deployment.Annotations[deployapi.DeploymentEncodedConfigAnnotation] = ""
		return true, deployment, nil
	})

	obj, err := NewREST(oc, kc, codec).Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Name: "config",
		Spec: deployapi.DeploymentConfigRollbackSpec{
			Revision: 1,
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
	oc := &testclient.Fake{}
	oc.AddReactor("get", "deploymentconfigs", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		dc := deploytest.OkDeploymentConfig(2)
		return true, nil, kerrors.NewNotFound(deployapi.Resource("deploymentConfig"), dc.Name)
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), codec)
		return true, deployment, nil
	})

	obj, err := NewREST(oc, kc, codec).Create(kapi.NewDefaultContext(), &deployapi.DeploymentConfigRollback{
		Name: "config",
		Spec: deployapi.DeploymentConfigRollbackSpec{
			Revision: 1,
		},
	})

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}
