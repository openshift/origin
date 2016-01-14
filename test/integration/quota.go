// +build integration,etcd

package integration

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kresource "k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/wait"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestSimpleQuota(t *testing.T) {
	projectName := "quota"

	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, "david")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quota := &kapi.ResourceQuota{
		ObjectMeta: kapi.ObjectMeta{Name: "my-quota"},
		Spec: kapi.ResourceQuotaSpec{
			Hard: kapi.ResourceList{
				kapi.ResourcePods: kresource.MustParse("100"),
			},
		},
	}
	if _, err := clusterAdminKubeClient.ResourceQuotas(projectName).Create(quota); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rc := &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{Name: "my-rc"},
		Spec: kapi.ReplicationControllerSpec{
			Replicas: 100,
			Selector: map[string]string{"whose": "mine"},
			Template: &kapi.PodTemplateSpec{
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{Image: "scratch"},
					},
				},
			},
		},
	}
	if _, err := clusterAdminKubeClient.ReplicationControllers(projectName).Create(rc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	startTime := time.Now()
	wait.PollImmediate(10*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		results, err := clusterAdminKubeClient.Pods(projectName).List(labels.Everything(), fields.Everything())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return false, nil
		}

		if len(results.Items) != 100 {
			return false, nil
		}

		return true, nil
	})

	duration := time.Since(startTime)
	t.Errorf("It too %v seconds", duration.Seconds())

}
