package rollback

import (
	"errors"
	"strings"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserverrest "k8s.io/apiserver/pkg/registry/rest"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	appsv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	appsfake "github.com/openshift/origin/pkg/apps/generated/internalclientset/fake"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

var codec = legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion)

type terribleGenerator struct{}

func (tg *terribleGenerator) GenerateRollback(from, to *appsapi.DeploymentConfig, spec *appsapi.DeploymentConfigRollbackSpec) (*appsapi.DeploymentConfig, error) {
	return nil, kerrors.NewInternalError(errors.New("something terrible happened"))
}

var _ RollbackGenerator = &terribleGenerator{}

func TestCreateError(t *testing.T) {
	rest := REST{}
	obj, err := rest.Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfig{}, apiserverrest.ValidateAllObjectFunc, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Errorf("Unexpected non-nil object: %#v", obj)
	}
}

func TestCreateInvalid(t *testing.T) {
	rest := REST{}
	obj, err := rest.Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{}, apiserverrest.ValidateAllObjectFunc, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Errorf("Unexpected non-nil object: %#v", obj)
	}
}

func TestCreateOk(t *testing.T) {
	oc := &appsfake.Clientset{}
	oc.AddReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, appstest.OkDeploymentConfig(2), nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1), codec)
		return true, deployment, nil
	})

	obj, err := NewREST(oc, kc, codec).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
		Name: "config",
		Spec: appsapi.DeploymentConfigRollbackSpec{
			Revision: 1,
		},
	}, apiserverrest.ValidateAllObjectFunc, false)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if obj == nil {
		t.Errorf("Expected a result obj")
	}

	if _, ok := obj.(*appsapi.DeploymentConfig); !ok {
		t.Errorf("expected a deployment config, got a %#v", obj)
	}
}

func TestCreateRollbackToLatest(t *testing.T) {
	oc := &appsfake.Clientset{}
	oc.AddReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, appstest.OkDeploymentConfig(2), nil
	})

	_, err := NewREST(oc, &fake.Clientset{}, codec).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
		Name: "config",
		Spec: appsapi.DeploymentConfigRollbackSpec{
			Revision: 2,
		},
	}, apiserverrest.ValidateAllObjectFunc, false)

	if err == nil {
		t.Errorf("expected an error when rolling back to the existing deployed revision")
	}
	if err != nil && !strings.Contains(err.Error(), "version 2 is already the latest") {
		t.Errorf("unexpected error received: %v", err)
	}
}

func TestCreateGeneratorError(t *testing.T) {
	oc := &appsfake.Clientset{}
	oc.AddReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, appstest.OkDeploymentConfig(2), nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1), codec)
		return true, deployment, nil
	})

	rest := REST{
		generator: &terribleGenerator{},
		dn:        oc.Apps(),
		rn:        kc.Core(),
		codec:     legacyscheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion),
	}

	_, err := rest.Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
		Name: "config",
		Spec: appsapi.DeploymentConfigRollbackSpec{
			Revision: 1,
		},
	}, apiserverrest.ValidateAllObjectFunc, false)

	if err == nil || !strings.Contains(err.Error(), "something terrible happened") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCreateMissingDeployment(t *testing.T) {
	oc := &appsfake.Clientset{}
	oc.AddReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, appstest.OkDeploymentConfig(2), nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1), codec)
		return true, nil, kerrors.NewNotFound(kapi.Resource("replicationController"), deployment.Name)
	})

	obj, err := NewREST(oc, kc, codec).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
		Name: "config",
		Spec: appsapi.DeploymentConfigRollbackSpec{
			Revision: 1,
		},
	}, apiserverrest.ValidateAllObjectFunc, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}

func TestCreateInvalidDeployment(t *testing.T) {
	oc := &appsfake.Clientset{}
	oc.AddReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, appstest.OkDeploymentConfig(2), nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		// invalidate the encoded config
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1), codec)
		deployment.Annotations[appsapi.DeploymentEncodedConfigAnnotation] = ""
		return true, deployment, nil
	})

	obj, err := NewREST(oc, kc, codec).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
		Name: "config",
		Spec: appsapi.DeploymentConfigRollbackSpec{
			Revision: 1,
		},
	}, apiserverrest.ValidateAllObjectFunc, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}

func TestCreateMissingDeploymentConfig(t *testing.T) {
	oc := &appsfake.Clientset{}
	oc.AddReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		dc := appstest.OkDeploymentConfig(2)
		return true, nil, kerrors.NewNotFound(appsapi.Resource("deploymentConfig"), dc.Name)
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1), codec)
		return true, deployment, nil
	})

	obj, err := NewREST(oc, kc, codec).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
		Name: "config",
		Spec: appsapi.DeploymentConfigRollbackSpec{
			Revision: 1,
		},
	}, apiserverrest.ValidateAllObjectFunc, false)

	if err == nil {
		t.Errorf("Expected an error")
	}

	if obj != nil {
		t.Error("Unexpected result obj")
	}
}
