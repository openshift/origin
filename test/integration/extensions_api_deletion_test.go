// +build integration,etcd

package integration

import (
	"testing"
	"time"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	expapi "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/wait"
)

func TestExtensionsAPIDeletion(t *testing.T) {
	const projName = "ext-deletion-proj"

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
	// TODO: switch to checking "list","horizontalpodautoscalers" once SAR supports API groups
	if err := testutil.WaitForPolicyUpdate(projectAdminClient, projName, "get", "pods", true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	// create the extensions resources as the project admin
	hpa := expapi.HorizontalPodAutoscaler{
		ObjectMeta: kapi.ObjectMeta{Name: "test-hpa"},
		Spec: expapi.HorizontalPodAutoscalerSpec{
			ScaleRef:       expapi.SubresourceReference{Kind: "DeploymentConfig", Name: "frontend", APIVersion: "v1", Subresource: "scale"},
			MaxReplicas:    10,
			CPUUtilization: &expapi.CPUTargetUtilization{TargetPercentage: 10},
		},
	}
	if _, err := projectAdminKubeClient.Extensions().HorizontalPodAutoscalers(projName).Create(&hpa); err != nil {
		t.Fatalf("unexpected error creating the HPA object: %v", err)
	}

	job := expapi.Job{
		ObjectMeta: kapi.ObjectMeta{Name: "test-job"},
		Spec: expapi.JobSpec{
			Selector: &expapi.PodSelector{MatchLabels: map[string]string{"foo": "bar"}},
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

	if _, err := clusterAdminKubeClient.Extensions().HorizontalPodAutoscalers(projName).Get(hpa.Name); err == nil {
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
