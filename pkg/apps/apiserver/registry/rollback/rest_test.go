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

	"github.com/openshift/api/apps"
	appsv1 "github.com/openshift/api/apps/v1"
	appsfake "github.com/openshift/client-go/apps/clientset/versioned/fake"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	appstest "github.com/openshift/origin/pkg/apps/util/test"
)

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
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1))
		deploymentInternal := &kapi.ReplicationController{}
		legacyscheme.Scheme.Convert(deployment, deploymentInternal, nil)
		return true, deploymentInternal, nil
	})

	obj, err := NewREST(oc, kc).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
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
		config := appstest.OkDeploymentConfig(2)
		return true, config, nil
	})

	_, err := NewREST(oc, &fake.Clientset{}).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
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
		config := appstest.OkDeploymentConfig(2)
		return true, config, nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1))
		internalDeployment := &kapi.ReplicationController{}
		legacyscheme.Scheme.Convert(deployment, internalDeployment, nil)
		return true, internalDeployment, nil
	})

	rest := REST{
		generator: &terribleGenerator{},
		dn:        oc.Apps(),
		rn:        kc.Core(),
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
		config := appstest.OkDeploymentConfig(2)
		return true, config, nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1))
		return true, nil, kerrors.NewNotFound(kapi.Resource("replicationController"), deployment.Name)
	})

	obj, err := NewREST(oc, kc).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
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
		config := appstest.OkDeploymentConfig(2)
		return true, config, nil
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		// invalidate the encoded config
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1))
		internalDeployment := &kapi.ReplicationController{}
		legacyscheme.Scheme.Convert(deployment, internalDeployment, nil)
		internalDeployment.Annotations[appsv1.DeploymentEncodedConfigAnnotation] = ""
		return true, internalDeployment, nil
	})

	obj, err := NewREST(oc, kc).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
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
		return true, nil, kerrors.NewNotFound(apps.Resource("deploymentConfig"), dc.Name)
	})
	kc := &fake.Clientset{}
	kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(1))
		internalDeployment := &kapi.ReplicationController{}
		legacyscheme.Scheme.Convert(deployment, internalDeployment, nil)
		return true, internalDeployment, nil
	})

	obj, err := NewREST(oc, kc).Create(apirequest.NewDefaultContext(), &appsapi.DeploymentConfigRollback{
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
