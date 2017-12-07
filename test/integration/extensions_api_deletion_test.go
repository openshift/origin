package integration

import (
	"testing"
	"time"

	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

func TestExtensionsAPIDeletion(t *testing.T) {
	const projName = "ext-deletion-proj"

	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

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
	if err := testutil.WaitForPolicyUpdate(projectAdminKubeClient.Authorization(), projName, "get", autoscaling.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	// create the extensions resources as the project admin
	percent := int32(10)
	hpa := autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "test-hpa"},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{Kind: "DeploymentConfig", Name: "frontend", APIVersion: "v1"},
			MaxReplicas:    10,
			Metrics: []autoscaling.MetricSpec{
				{
					Type: autoscaling.ResourceMetricSourceType,
					Resource: &autoscaling.ResourceMetricSource{
						Name: kapi.ResourceCPU,
						TargetAverageUtilization: &percent,
					},
				},
			},
		},
	}
	if _, err := projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName).Create(&hpa); err != nil {
		t.Fatalf("unexpected error creating the HPA object: %v", err)
	}

	job := batch.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job"},
		Spec: batch.JobSpec{
			Template: kapi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
				Spec: kapi.PodSpec{
					Containers:    []kapi.Container{{Name: "baz", Image: "run"}},
					RestartPolicy: kapi.RestartPolicyOnFailure,
				},
			},
		},
	}
	if _, err := projectAdminKubeClient.Batch().Jobs(projName).Create(&job); err != nil {
		t.Fatalf("unexpected error creating the job object: %v", err)
	}

	if err := projectclient.NewForConfigOrDie(clusterAdminClientConfig).Project().Projects().Delete(projName, nil); err != nil {
		t.Fatalf("unexpected error deleting the project: %v", err)
	}
	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		_, err := clusterAdminKubeClient.Core().Namespaces().Get(projName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("unexpected error while waiting for project to delete: %v", err)
	}

	if _, err := clusterAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName).Get(hpa.Name, metav1.GetOptions{}); err == nil {
		t.Fatalf("HPA object was still present after project was deleted!")
	} else if !errors.IsNotFound(err) {
		t.Fatalf("Error trying to get deleted HPA object (not a not-found error): %v", err)
	}
	if _, err := clusterAdminKubeClient.Batch().Jobs(projName).Get(job.Name, metav1.GetOptions{}); err == nil {
		t.Fatalf("Job object was still present after project was deleted!")
	} else if !errors.IsNotFound(err) {
		t.Fatalf("Error trying to get deleted Job object (not a not-found error): %v", err)
	}
}
