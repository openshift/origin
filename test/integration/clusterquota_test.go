// +build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const (
	numProjects             = 100
	podsPerProjectPerSecond = 20.0
	testLength              = 30 * time.Second
)

func TestClusterQuota(t *testing.T) {
	testutil.RequireEtcd(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cq := &authorizationapi.ClusterResourceQuota{
		ObjectMeta: kapi.ObjectMeta{Name: "overall"},
		Spec: authorizationapi.ClusterResourceQuotaSpec{
			Quota: kapi.ResourceQuotaSpec{
				Hard: kapi.ResourceList{
					kapi.ResourceConfigMaps: resource.MustParse("1000000"),
				},
			},
		},
	}
	if _, err := clusterAdminClient.ClusterResourceQuotas().Create(cq); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	startCh := make(chan struct{})
	stopCh := make(chan struct{})

	for i := 0; i < numProjects; i++ {
		projectName := fmt.Sprintf("project-%v", i)
		if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, "harold"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := testserver.WaitForPodCreationServiceAccounts(clusterAdminKubeClient, projectName); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		go makeConfigMaps(clusterAdminKubeClient.ConfigMaps(projectName), podsPerProjectPerSecond, startCh, stopCh)
		// go makeConfigMaps(clusterAdminKubeClient.ConfigMaps(projectName), podsPerProjectPerSecond, startCh, stopCh)
		// go makeConfigMaps(clusterAdminKubeClient.ConfigMaps(projectName), podsPerProjectPerSecond, startCh, stopCh)
		// go makeConfigMaps(clusterAdminKubeClient.ConfigMaps(projectName), podsPerProjectPerSecond, startCh, stopCh)
	}

	// this should make everyone start trying to create pods
	close(startCh)
	time.Sleep(testLength)
	// this should make everyone stop trying to create pods
	close(stopCh)

	// let's see how many we got
	podList, err := clusterAdminKubeClient.ConfigMaps(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Errorf("Got %v pods in %v", len(podList.Items), testLength)

	endingCq, _ := clusterAdminClient.ClusterResourceQuotas().Get(cq.Name)
	t.Errorf("Ended with %#v", endingCq)
}

func makePods(podClient kclient.PodInterface, podsPerSecond float32, startCh, stopCh chan struct{}) {
	<-startCh

	testPod := &kapi.Pod{}
	testPod.GenerateName = "test"
	testPod.Spec.Containers = []kapi.Container{
		{
			Name:  "container",
			Image: "openshift/origin-pod:latest",
		},
	}

	startTime := time.Now()
	numPods := 0
	waitDuration := time.Duration(int64((1.2*1000)/podsPerSecond)) * time.Millisecond

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		duration := time.Now().Sub(startTime)
		numberOfExpectedPods := float32(duration.Seconds()) * podsPerSecond

		if numberOfExpectedPods <= float32(numPods) {
			select {
			case <-stopCh:
				return
			case <-time.After(waitDuration):
				continue
			}
		}

		if _, err := podClient.Create(testPod); err == nil {
			numPods = numPods + 1
		}
	}
}

func makeConfigMaps(client kclient.ConfigMapsInterface, podsPerSecond float32, startCh, stopCh chan struct{}) {
	<-startCh

	configmap := &kapi.ConfigMap{}
	configmap.GenerateName = "test"

	startTime := time.Now()
	numPods := 0
	waitDuration := time.Duration(int64((1.2*1000)/podsPerSecond)) * time.Millisecond

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		duration := time.Now().Sub(startTime)
		numberOfExpectedPods := float32(duration.Seconds()) * podsPerSecond

		if numberOfExpectedPods <= float32(numPods) {
			select {
			case <-stopCh:
				return
			case <-time.After(waitDuration):
				continue
			}
		}

		if _, err := client.Create(configmap); err == nil {
			numPods = numPods + 1
		}
	}
}
