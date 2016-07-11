package testclient

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
)

func TestNewClient(t *testing.T) {
	o := testclient.NewObjects(kapi.Scheme, kapi.Codecs.UniversalDecoder())
	if err := testclient.AddObjectsFromPath("../../../test/integration/testdata/test-deployment-config.yaml", o, kapi.Codecs.UniversalDecoder()); err != nil {
		t.Fatal(err)
	}
	oc, _ := NewFixtureClients(o)
	list, err := oc.DeploymentConfigs("test").List(kapi.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("unexpected list %#v", list)
	}

	// same result
	list, err = oc.DeploymentConfigs("test").List(kapi.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("unexpected list %#v", list)
	}
	t.Logf("list: %#v", list)
}

func TestErrors(t *testing.T) {
	o := testclient.NewObjects(kapi.Scheme, kapi.Codecs.UniversalDecoder())
	o.Add(&kapi.List{
		Items: []runtime.Object{
			&(errors.NewNotFound(deployapi.Resource("DeploymentConfigList"), "").ErrStatus),
			&(errors.NewForbidden(deployapi.Resource("DeploymentConfigList"), "", nil).ErrStatus),
		},
	})
	oc, _ := NewFixtureClients(o)
	_, err := oc.DeploymentConfigs("test").List(kapi.ListOptions{})
	if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("error: %#v", err.(*errors.StatusError).Status())
	_, err = oc.DeploymentConfigs("test").List(kapi.ListOptions{})
	if !errors.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}
}
