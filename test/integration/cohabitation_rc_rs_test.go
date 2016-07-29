package integration

import (
	"encoding/json"
	"testing"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	kapi "k8s.io/kubernetes/pkg/api"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"
)

type fakeObject struct {
	Kind       string       `json:"kind"`
	APIVersion string       `json:"apiVersion"`
	Metadata   fakeMetadata `json:"metadata"`
}

type fakeMetadata struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
}

func TestRCRSCohabitation(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err := testutil.GetFixture("../../vendor/k8s.io/kubernetes/docs/user-guide/replication.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := obj.(*kapi.ReplicationController)
	if _, err := clusterAdminKubeClient.ReplicationControllers("default").Create(rc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err = testutil.GetFixture("../../vendor/k8s.io/kubernetes/docs/user-guide/replicaset/frontend.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rs := obj.(*extensionsapi.ReplicaSet)
	if _, err := clusterAdminKubeClient.Extensions().ReplicaSets("default").Create(rs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// make sure we get back both from each endpoint
	rcList, err := clusterAdminKubeClient.ReplicationControllers("default").List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundFrontend := false
	foundNginx := false
	for _, rc := range rcList.Items {
		if rc.Name == "nginx" {
			foundNginx = true
		}
		if rc.Name == "frontend" {
			foundFrontend = true
		}
	}
	if !foundFrontend || !foundNginx {
		t.Errorf("missing rc: %#v", rcList)
	}

	rsList, err := clusterAdminKubeClient.Extensions().ReplicaSets("default").List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundFrontend = false
	foundNginx = false
	for _, rs := range rsList.Items {
		if rs.Name == "nginx" {
			foundNginx = true
		}
		if rs.Name == "frontend" {
			foundFrontend = true
		}
	}
	if !foundFrontend || !foundNginx {
		t.Errorf("missing rs: %#v", rsList)
	}

	etcdClient := testutil.NewEtcdClient()
	etcdControllers, err := etcdClient.Get("/kubernetes.io/controllers/default", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, node := range etcdControllers.Node.Nodes {
		obj := fakeObject{}
		if err := json.Unmarshal([]byte(node.Value), &obj); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if obj.Kind != "ReplicationController" || obj.APIVersion != "v1" {
			t.Errorf("wrong serialization: %v", node.Value)
		}
	}

	rcWatch, err := clusterAdminKubeClient.ReplicationControllers("default").Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundFrontend = false
	foundNginx = false
	for i := 0; i < 2; i++ {
		obj := <-rcWatch.ResultChan()
		rc := obj.Object.(*kapi.ReplicationController)
		if rc.Name == "nginx" {
			foundNginx = true
		}
		if rc.Name == "frontend" {
			foundFrontend = true
		}
	}
	if !foundFrontend || !foundNginx {
		t.Errorf("missing rc: %v %v", foundFrontend, foundNginx)
	}

	rsWatch, err := clusterAdminKubeClient.Extensions().ReplicaSets("default").Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundFrontend = false
	foundNginx = false
	for i := 0; i < 2; i++ {
		obj := <-rsWatch.ResultChan()
		rs := obj.Object.(*extensionsapi.ReplicaSet)
		if rs.Name == "nginx" {
			foundNginx = true
		}
		if rs.Name == "frontend" {
			foundFrontend = true
		}
	}
	if !foundFrontend || !foundNginx {
		t.Errorf("missing rs: %v %v", foundFrontend, foundNginx)
	}

}
