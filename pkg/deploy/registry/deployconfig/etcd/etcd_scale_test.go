package etcd

import (
	"fmt"
	"testing"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/tools"
	"k8s.io/kubernetes/pkg/tools/etcdtest"
	"k8s.io/kubernetes/pkg/util"
)

const (
	namespace = "test-ns"
	name      = "test-dc"
)

func newStorage(t *testing.T, rcNamespacer kclient.ReplicationControllersNamespacer) (*DeploymentConfigStorage, *tools.FakeEtcdClient) {
	etcdStorage, fakeEtcdClient := registrytest.NewEtcdStorage(t, "extensions")
	dcStorage := NewStorage(etcdStorage, rcNamespacer)

	return &dcStorage, fakeEtcdClient
}

func makeScale(targetReplicas, actualReplicas int) *extensions.Scale {
	return &extensions.Scale{
		ObjectMeta: kapi.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       extensions.ScaleSpec{Replicas: targetReplicas},
		Status: extensions.ScaleStatus{
			Replicas: actualReplicas,
			Selector: deploytest.OkSelector(),
		},
	}
}

func makeDeploymentConfig(replicas, version int) *deployapi.DeploymentConfig {
	dc := deploytest.OkDeploymentConfig(version)
	dc.Name = name
	dc.Namespace = namespace
	dc.Template.ControllerTemplate.Replicas = replicas

	return dc
}

func makeDeploymentWithReplicas(dc *deployapi.DeploymentConfig, replicas int) (*kapi.ReplicationController, error) {
	rc, err := deployutil.MakeDeployment(dc, api.Codec)
	if err != nil {
		return nil, err
	}
	rc.Spec.Replicas = 1
	return rc, nil
}

func makeFakeClient(replicationControllers ...kapi.ReplicationController) *ktestclient.Fake {
	rcs := map[string]kapi.ReplicationController{}
	for _, rc := range replicationControllers {
		rcs[rc.Name] = rc
	}

	fakeClient := &ktestclient.Fake{}
	reaction := func(action ktestclient.Action) (bool, runtime.Object, error) {
		if action.GetResource() != "replicationcontrollers" {
			return false, nil, fmt.Errorf("Expected an action involving ReplicationControllers")
		}

		switch castAction := action.(type) {
		case ktestclient.ListAction:
			return true, &kapi.ReplicationControllerList{Items: replicationControllers}, nil
		case ktestclient.GetAction:
			if rc, ok := rcs[castAction.GetName()]; ok {
				return true, &rc, nil
			}
			return true, nil, errors.NewNotFound("ReplicationController", castAction.GetName())
		default:
			return false, nil, fmt.Errorf("no reaction implemented for %s", action)
		}
	}
	fakeClient.AddReactor("get", "replicationcontrollers", reaction)
	fakeClient.AddReactor("list", "replicationcontrollers", reaction)

	return fakeClient
}

func TestScaleGet(t *testing.T) {
	dc := makeDeploymentConfig(1, 0)
	rc, err := makeDeploymentWithReplicas(dc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fakeKClient := makeFakeClient(*rc)
	storage, fakeEtcdClient := newStorage(t, fakeKClient)

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	key := etcdtest.AddPrefix("/deploymentconfigs/" + namespace + "/" + name)
	if _, err := fakeEtcdClient.Set(key, runtime.EncodeOrDie(api.Codec, dc), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := makeScale(1, 1)
	obj, err := storage.Scale.Get(ctx, name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scale := obj.(*extensions.Scale)
	if !kapi.Semantic.DeepEqual(expected, scale) {
		t.Errorf("Unexpected scale returned: %s", util.ObjectDiff(expected, scale))
	}
}

func TestScaleUpdateInvalidDeployment(t *testing.T) {
	dc := makeDeploymentConfig(1, 1)
	rc, err := makeDeploymentWithReplicas(dc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc.Annotations[deployapi.DeploymentStatusAnnotation] = "Blech"
	fakeKClient := makeFakeClient(*rc)
	storage, fakeEtcdClient := newStorage(t, fakeKClient)

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	key := etcdtest.AddPrefix("/deploymentconfigs/" + namespace + "/" + name)
	if _, err := fakeEtcdClient.Set(key, runtime.EncodeOrDie(api.Codec, dc), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fakeKClient.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		t.Errorf("Expected deployment not to be updated")
		obj := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		return true, obj, nil
	})

	update := makeScale(2, 1)
	if _, _, err := storage.Scale.Update(ctx, update); err == nil {
		t.Errorf("Expected the update to fail due to a failed deployment")
	}
}

func TestScaleUpdateValidDeployment(t *testing.T) {
	dc := makeDeploymentConfig(1, 1)
	rc, err := makeDeploymentWithReplicas(dc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	fakeKClient := makeFakeClient(*rc)
	storage, fakeEtcdClient := newStorage(t, fakeKClient)

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	key := etcdtest.AddPrefix("/deploymentconfigs/" + namespace + "/" + name)
	if _, err := fakeEtcdClient.Set(key, runtime.EncodeOrDie(api.Codec, dc), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	update := makeScale(2, 1)
	wasScaled := false

	fakeKClient.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		obj := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		replicas := obj.Spec.Replicas
		if obj.Name != name+"-1" {
			t.Errorf("Expected replication controller \"%s-1\" to have been updated, not \"%s\"", name, obj.Name)
		}

		if replicas != update.Spec.Replicas {
			t.Errorf("Expected replication controller to be scaled to %v replicas, not %v", update.Spec.Replicas, replicas)
		}

		wasScaled = true
		return true, obj, nil
	})

	if _, _, err := storage.Scale.Update(ctx, update); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wasScaled {
		t.Errorf("Expected replication controller to be scaled")
	}
}

func TestScaleUpdateMultipleValidDeployments(t *testing.T) {
	dc := makeDeploymentConfig(1, 1)
	rc1, err := makeDeploymentWithReplicas(dc, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dc.LatestVersion = 2
	rc2, err := makeDeploymentWithReplicas(dc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc1.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	rc2.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
	fakeKClient := makeFakeClient(*rc1, *rc2)
	storage, fakeEtcdClient := newStorage(t, fakeKClient)

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	key := etcdtest.AddPrefix("/deploymentconfigs/" + namespace + "/" + name)
	if _, err := fakeEtcdClient.Set(key, runtime.EncodeOrDie(api.Codec, dc), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	update := makeScale(2, 1)
	wasScaled := false

	fakeKClient.AddReactor("update", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		obj := action.(ktestclient.UpdateAction).GetObject().(*kapi.ReplicationController)
		replicas := obj.Spec.Replicas
		if obj.Name != name+"-2" {
			t.Errorf("Expected replication controller \"%s-2\" to have been updated, not \"%s\"", name, obj.Name)
		}

		if replicas != update.Spec.Replicas {
			t.Errorf("Expected replication controller to be scaled to %v replicas, not %v", update.Spec.Replicas, replicas)
		}

		wasScaled = true
		return true, obj, nil
	})

	if _, _, err := storage.Scale.Update(ctx, update); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wasScaled {
		t.Errorf("Expected replication controller to be scaled")
	}

}
