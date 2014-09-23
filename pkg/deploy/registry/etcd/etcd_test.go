package etcd

import (
	"fmt"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/coreos/go-etcd/etcd"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/deploy/api"
)

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.EtcdHelper{client, latest.Codec, latest.ResourceVersioner})
}

func TestEtcdListEmptyDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/deployments"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deployments, err := registry.ListDeployments(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deployments.Items) != 0 {
		t.Errorf("Unexpected deployments list: %#v", deployments)
	}
}

func TestEtcdListErrorDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/deployments"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	deployments, err := registry.ListDeployments(labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if deployments != nil {
		t.Errorf("Unexpected non-nil deployments: %#v", deployments)
	}
}

func TestEtcdListEverythingDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/deployments"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{JSONBase: kubeapi.JSONBase{ID: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deployments, err := registry.ListDeployments(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deployments.Items) != 2 || deployments.Items[0].ID != "foo" || deployments.Items[1].ID != "bar" {
		t.Errorf("Unexpected deployments list: %#v", deployments)
	}
}

func TestEtcdListFilteredDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/deployments"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{
							JSONBase: kubeapi.JSONBase{ID: "foo"},
							Labels:   map[string]string{"env": "prod"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{
							JSONBase: kubeapi.JSONBase{ID: "bar"},
							Labels:   map[string]string{"env": "dev"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deployments, err := registry.ListDeployments(labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deployments.Items) != 1 || deployments.Items[0].ID != "bar" {
		t.Errorf("Unexpected deployments list: %#v", deployments)
	}
}

func TestEtcdGetDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set("/deployments/foo", runtime.EncodeOrDie(latest.Codec, &api.Deployment{JSONBase: kubeapi.JSONBase{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	deployment, err := registry.GetDeployment("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if deployment.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestEtcdGetNotFoundDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/deployments/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	deployment, err := registry.GetDeployment("foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if deployment != nil {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestEtcdCreateDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data["/deployments/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeployment(&api.Deployment{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get("/deployments/foo", false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var deployment api.Deployment
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &deployment)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if deployment.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v %s", deployment, resp.Node.Value)
	}
}

func TestEtcdCreateAlreadyExistsDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/deployments/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Deployment{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeployment(&api.Deployment{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected 'already exists' error, got %#v", err)
	}
}

func TestEtcdUpdateOkDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateDeployment(&api.Deployment{})
	if err != nil {
		t.Error("Unexpected error")
	}
}

func TestEtcdDeleteNotFoundDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeployment("foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteErrorDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeployment("foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteOkDeployments(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := "/deployments/foo"
	err := registry.DeleteDeployment("foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdListEmptyDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/deploymentConfigs"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deploymentConfigs, err := registry.ListDeploymentConfigs(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deploymentConfigs.Items) != 0 {
		t.Errorf("Unexpected deploymentConfigs list: %#v", deploymentConfigs)
	}
}

func TestEtcdListErrorDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/deploymentConfigs"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	deploymentConfigs, err := registry.ListDeploymentConfigs(labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if deploymentConfigs != nil {
		t.Errorf("Unexpected non-nil deploymentConfigs: %#v", deploymentConfigs)
	}
}

func TestEtcdListEverythingDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/deploymentConfigs"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{JSONBase: kubeapi.JSONBase{ID: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deploymentConfigs, err := registry.ListDeploymentConfigs(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deploymentConfigs.Items) != 2 || deploymentConfigs.Items[0].ID != "foo" || deploymentConfigs.Items[1].ID != "bar" {
		t.Errorf("Unexpected deploymentConfigs list: %#v", deploymentConfigs)
	}
}

func TestEtcdListFilteredDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/deploymentConfigs"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{
							JSONBase: kubeapi.JSONBase{ID: "foo"},
							Labels:   map[string]string{"env": "prod"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{
							JSONBase: kubeapi.JSONBase{ID: "bar"},
							Labels:   map[string]string{"env": "dev"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	deploymentConfigs, err := registry.ListDeploymentConfigs(labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(deploymentConfigs.Items) != 1 || deploymentConfigs.Items[0].ID != "bar" {
		t.Errorf("Unexpected deploymentConfigs list: %#v", deploymentConfigs)
	}
}

func TestEtcdGetDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set("/deploymentConfigs/foo", runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{JSONBase: kubeapi.JSONBase{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	deployment, err := registry.GetDeploymentConfig("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if deployment.ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestEtcdGetNotFoundDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/deploymentConfigs/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	deployment, err := registry.GetDeploymentConfig("foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if deployment != nil {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestEtcdCreateDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data["/deploymentConfigs/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeploymentConfig(&api.DeploymentConfig{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get("/deploymentConfigs/foo", false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var d api.DeploymentConfig
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &d)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if d.ID != "foo" {
		t.Errorf("Unexpected deploymentConfig: %#v %s", d, resp.Node.Value)
	}
}

func TestEtcdCreateAlreadyExistsDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/deploymentConfigs/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.DeploymentConfig{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateDeploymentConfig(&api.DeploymentConfig{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected 'already exists' error, got %#v", err)
	}
}

func TestEtcdUpdateOkDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateDeploymentConfig(&api.DeploymentConfig{})
	if err != nil {
		t.Error("Unexpected error")
	}
}

func TestEtcdDeleteNotFoundDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeploymentConfig("foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteErrorDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteDeploymentConfig("foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteOkDeploymentConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := "/deploymentConfigs/foo"
	err := registry.DeleteDeploymentConfig("foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}
