package integration

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	kapi "k8s.io/kubernetes/pkg/api"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func TestDCDCohabitation(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	projectName := "dc-d-cohabitation"
	username := "some-user"
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, username); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oclient, kclient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, username)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err := testutil.GetFixture("../../test/extended/testdata/deployment-simple.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dc := obj.(*deployapi.DeploymentConfig)
	if _, err := oclient.DeploymentConfigs(projectName).Create(dc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err = testutil.GetFixture("../../vendor/k8s.io/kubernetes/docs/user-guide/deployment.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := obj.(*extensionsapi.Deployment)
	if _, err := kclient.Extensions().Deployments(projectName).Create(d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// wait until we have two RCs.  this ensures that controllers have run and tried to create things.
	// if our types were ever going to be stored wrong, this is where it would happen
	rcWatch, err := kclient.ReplicationControllers(projectName).Watch(kapi.ListOptions{ResourceVersion: "0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	numAdds := 0
	watch.Until(30*time.Second, rcWatch, func(event watch.Event) (bool, error) {
		if event.Type == watch.Added {
			numAdds = numAdds + 1
		}
		if numAdds >= 2 {
			return true, nil
		}
		return false, nil
	})

	// make sure we get back both from each list endpoint
	/////////////////////////////////////////////////////////////////////////////////
	dcList, err := oclient.DeploymentConfigs(projectName).List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundFrontend := false
	foundNginx := false
	for _, dc := range dcList.Items {
		if dc.Name == "nginx-deployment" {
			foundNginx = true
		}
		if dc.Name == "deployment-simple" {
			foundFrontend = true
		}
	}
	if !foundFrontend || !foundNginx {
		t.Errorf("missing dc: %#v", dcList)
	}

	dList, err := kclient.Extensions().Deployments(projectName).List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundFrontend = false
	foundNginx = false
	for _, d := range dList.Items {
		if d.Name == "nginx-deployment" {
			foundNginx = true
		}
		if d.Name == "deployment-simple" {
			foundFrontend = true
		}
	}
	if !foundFrontend || !foundNginx {
		t.Errorf("missing d: %#v", dList)
	}
	/////////////////////////////////////////////////////////////////////////////////

	// make sure we serialize in the correct format
	/////////////////////////////////////////////////////////////////////////////////
	etcdClient := testutil.NewEtcdClient()
	etcdControlled, err := etcdClient.Get("/openshift.io/deploymentconfigs/"+projectName, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, node := range etcdControlled.Node.Nodes {
		obj := fakeObject{}
		if err := json.Unmarshal([]byte(node.Value), &obj); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		switch obj.Metadata.Name {
		case "deployment-simple":
			if obj.Kind != "DeploymentConfig" || obj.APIVersion != "v1" {
				t.Errorf("wrong serialization: %v", node.Value)
			}

		case "nginx-deployment":
			if obj.Kind != "Deployment" || obj.APIVersion != "extensions/v1beta1" {
				t.Errorf("wrong serialization: %v", node.Value)
			}

		default:
			t.Errorf("unexpected deployment: %v", node.Value)
		}

		for key := range obj.Metadata.Annotations {
			if key == kapi.OriginalKindAnnotationName {
				t.Errorf("wrong serialization: %v", node.Value)
			}
			if strings.HasPrefix(key, kapi.NonConvertibleAnnotationPrefix) {
				t.Errorf("wrong serialization: %v", node.Value)
			}
		}
	}
	/////////////////////////////////////////////////////////////////////////////////

	// make the lists return each object in the correct format
	/////////////////////////////////////////////////////////////////////////////////
	dcWatch, err := oclient.DeploymentConfigs(projectName).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundFrontend = false
	foundNginx = false
	for i := 0; i < 2; i++ {
		obj := <-dcWatch.ResultChan()
		dc := obj.Object.(*deployapi.DeploymentConfig)
		if dc.Name == "nginx-deployment" {
			foundNginx = true
		}
		if dc.Name == "deployment-simple" {
			foundFrontend = true
		}
	}
	if !foundFrontend || !foundNginx {
		t.Errorf("missing dc: %v %v", foundFrontend, foundNginx)
	}

	dWatch, err := kclient.Extensions().Deployments(projectName).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundFrontend = false
	foundNginx = false
	for i := 0; i < 2; i++ {
		obj := <-dWatch.ResultChan()
		d := obj.Object.(*extensionsapi.Deployment)
		if d.Name == "nginx-deployment" {
			foundNginx = true
		}
		if d.Name == "deployment-simple" {
			foundFrontend = true
		}
	}
	if !foundFrontend || !foundNginx {
		t.Errorf("missing d: %v %v", foundFrontend, foundNginx)
	}
	/////////////////////////////////////////////////////////////////////////////////

	// make the gets have the correct annotations and that updates across types are rejected
	/////////////////////////////////////////////////////////////////////////////////
	dAsDC, err := oclient.DeploymentConfigs(projectName).Get("nginx-deployment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := oclient.DeploymentConfigs(projectName).Update(dAsDC); err == nil || !strings.Contains(err.Error(), "wrong native type, no cross type updates allowed") {
		t.Errorf("wrong error: %v", err)
	}
	if _, err := clusterAdminClient.DeploymentConfigs(projectName).UpdateStatus(dAsDC); err == nil || !strings.Contains(err.Error(), "wrong native type, no cross type updates allowed") {
		t.Errorf("wrong error: %v", err)
	}
	if dAsDC.Annotations[kapi.OriginalKindAnnotationName] != "Deployment.extensions" {
		t.Errorf("missing annotation: %v", dAsDC.Annotations)
	}

	dcAsD, err := kclient.Extensions().Deployments(projectName).Get("deployment-simple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := kclient.Extensions().Deployments(projectName).Update(dcAsD); err == nil || !strings.Contains(err.Error(), "wrong native type, no cross type updates allowed") {
		t.Errorf("wrong error: %v", err)
	}
	if _, err := clusterAdminKubeClient.Extensions().Deployments(projectName).UpdateStatus(dcAsD); err == nil || !strings.Contains(err.Error(), "wrong native type, no cross type updates allowed") {
		t.Errorf("wrong error: %v", err)
	}
	if dcAsD.Annotations[kapi.OriginalKindAnnotationName] != "DeploymentConfig." {
		t.Errorf("missing annotation: %v", dcAsD.Annotations)
	}
	/////////////////////////////////////////////////////////////////////////////////

}
