package integration

import (
	"testing"
	"time"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	expapi "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/wait"
)

func TestExtensionsAPIDeletion(t *testing.T) {
	const projName = "ext-deletion-proj"

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
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

	// create the extensions resources as the project admin
	percent := int32(10)
	hpa := autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: kapi.ObjectMeta{Name: "test-hpa"},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			ScaleTargetRef:                 autoscaling.CrossVersionObjectReference{Kind: "DeploymentConfig", Name: "frontend", APIVersion: "v1"},
			MaxReplicas:                    10,
			TargetCPUUtilizationPercentage: &percent,
		},
	}
	if _, err := projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName).Create(&hpa); err != nil {
		t.Fatalf("unexpected error creating the HPA object: %v", err)
	}

	job := batch.Job{
		ObjectMeta: kapi.ObjectMeta{Name: "test-job"},
		Spec: batch.JobSpec{
			Template: kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
				Spec: kapi.PodSpec{
					Containers:    []kapi.Container{{Name: "baz", Image: "run"}},
					RestartPolicy: kapi.RestartPolicyOnFailure,
				},
			},
		},
	}
	if _, err := projectAdminKubeClient.Extensions().Jobs(projName).Create(&job); err != nil {
		t.Fatalf("unexpected error creating the job object: %v", err)
	}

	if err := clusterAdminClient.Projects().Delete(projName); err != nil {
		t.Fatalf("unexpected error deleting the project: %v", err)
	}
	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		_, err := clusterAdminKubeClient.Namespaces().Get(projName)
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("unexpected error while waiting for project to delete: %v", err)
	}

	if _, err := clusterAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName).Get(hpa.Name); err == nil {
		t.Fatalf("HPA object was still present after project was deleted!")
	} else if !errors.IsNotFound(err) {
		t.Fatalf("Error trying to get deleted HPA object (not a not-found error): %v", err)
	}
	if _, err := clusterAdminKubeClient.Extensions().Jobs(projName).Get(job.Name); err == nil {
		t.Fatalf("Job object was still present after project was deleted!")
	} else if !errors.IsNotFound(err) {
		t.Fatalf("Error trying to get deleted Job object (not a not-found error): %v", err)
	}
}
