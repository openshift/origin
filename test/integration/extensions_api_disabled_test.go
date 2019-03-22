package integration

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscalingapi "k8s.io/kubernetes/pkg/apis/autoscaling"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestExtensionsAPIDisabledAutoscaleBatchEnabled(t *testing.T) {
	const projName = "ext-disabled-batch-enabled-proj"

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	// Disable all extensions API versions
	// Leave autoscaling/batch APIs enabled
	masterConfig.KubernetesMasterConfig.DisabledAPIGroupVersions = map[string][]string{"extensions": {"*"}}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
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
	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, projName, "admin"); err != nil {
		t.Fatalf("unexpected error creating the project: %v", err)
	}
	projectAdminKubeClient, _, err := testutil.GetClientForUser(clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(projectAdminKubeClient.AuthorizationV1(), projName, "get", autoscalingapi.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	validJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "myjob"},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "mycontainer", Image: "myimage"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	// make sure autoscaling and batch API objects can be listed and created
	if _, err := projectAdminKubeClient.BatchV1().Jobs(projName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}
	if _, err := projectAdminKubeClient.BatchV1().Jobs(projName).Create(validJob); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}

	// Delete the containing project
	if err := testutil.DeleteAndWaitForNamespaceTermination(clusterAdminKubeClient, projName); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}

	// recreate the containing project
	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, projName, "admin"); err != nil {
		t.Fatalf("unexpected error creating the project: %v", err)
	}
	projectAdminKubeClient, _, err = testutil.GetClientForUser(clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(projectAdminKubeClient.AuthorizationV1(), projName, "get", autoscalingapi.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	// make sure the created objects got cleaned up by namespace deletion
	if jobs, err := projectAdminKubeClient.BatchV1().Jobs(projName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	} else if len(jobs.Items) > 0 {
		t.Fatalf("expected 0 Job objects, got %#v", jobs.Items)
	}
}
