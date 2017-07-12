package integration

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	expapi "k8s.io/kubernetes/pkg/apis/extensions"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestExtensionsAPIDisabledAutoscaleBatchEnabled(t *testing.T) {
	const projName = "ext-disabled-batch-enabled-proj"

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Disable all extensions API versions
	// Leave autoscaling/batch APIs enabled
	masterConfig.KubernetesMasterConfig.DisabledAPIGroupVersions = map[string][]string{"extensions": {"*"}}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
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

	// create the containing project
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projName, "admin"); err != nil {
		t.Fatalf("unexpected error creating the project: %v", err)
	}
	projectAdminClient, projectAdminKubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(projectAdminClient, projName, "get", expapi.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	validJob := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "myjob"},
		Spec: batch.JobSpec{
			Template: kapi.PodTemplateSpec{
				Spec: kapi.PodSpec{
					Containers:    []kapi.Container{{Name: "mycontainer", Image: "myimage"}},
					RestartPolicy: kapi.RestartPolicyNever,
				},
			},
		},
	}

	// make sure autoscaling and batch API objects can be listed and created
	if _, err := projectAdminKubeClient.Batch().Jobs(projName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}
	if _, err := projectAdminKubeClient.Batch().Jobs(projName).Create(validJob); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}

	// Delete the containing project
	if err := testutil.DeleteAndWaitForNamespaceTermination(clusterAdminKubeClient, projName); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}

	// recreate the containing project
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projName, "admin"); err != nil {
		t.Fatalf("unexpected error creating the project: %v", err)
	}
	projectAdminClient, projectAdminKubeClient, _, err = testutil.GetClientForUser(*clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(projectAdminClient, projName, "get", expapi.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	// make sure the created objects got cleaned up by namespace deletion
	if jobs, err := projectAdminKubeClient.Batch().Jobs(projName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	} else if len(jobs.Items) > 0 {
		t.Fatalf("expected 0 Job objects, got %#v", jobs.Items)
	}
}
