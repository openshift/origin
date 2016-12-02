package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	expapi "k8s.io/kubernetes/pkg/apis/extensions"
	extensions_v1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kclientset15 "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"

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
	projectAdminClient, projectAdminKubeClient, projectAdminKubeConfig, err := testutil.GetClientForUser(*clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}
	projectAdminKubeClient15 := kclientset15.NewForConfigOrDie(projectAdminKubeConfig)
	if err := testutil.WaitForPolicyUpdate(projectAdminClient, projName, "get", expapi.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	validHPA := &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: kapi.ObjectMeta{Name: "myjob"},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{Name: "foo", Kind: "ReplicationController"},
			MaxReplicas:    1,
		},
	}
	validJob := &batch.Job{
		ObjectMeta: kapi.ObjectMeta{Name: "myjob"},
		Spec: batch.JobSpec{
			Template: kapi.PodTemplateSpec{
				Spec: kapi.PodSpec{
					Containers:    []kapi.Container{{Name: "mycontainer", Image: "myimage"}},
					RestartPolicy: kapi.RestartPolicyNever,
				},
			},
		},
	}

	legacyAutoscalers := legacyExtensionsAutoscaling{
		projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName),
		projectAdminKubeClient.Extensions().RESTClient(),
		projName,
	}

	// make sure extensions API objects cannot be listed or created
	if _, err := legacyAutoscalers.List(kapi.ListOptions{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error listing HPA, got %v", err)
	}
	if _, err := legacyAutoscalers.Create(validHPA); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error creating HPA, got %v", err)
	}
	if _, err := projectAdminKubeClient15.Extensions().Jobs(projName).List(kapiv1.ListOptions{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error listing jobs, got %v", err)
	}
	if _, err := projectAdminKubeClient15.Extensions().Jobs(projName).Create(&extensions_v1beta1.Job{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error creating job, got %v", err)
	}

	// make sure autoscaling and batch API objects can be listed and created
	if _, err := projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName).List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}
	if _, err := projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName).Create(validHPA); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}
	if _, err := projectAdminKubeClient.Batch().Jobs(projName).List(kapi.ListOptions{}); err != nil {
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
	if hpas, err := projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName).List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	} else if len(hpas.Items) > 0 {
		t.Fatalf("expected 0 HPA objects, got %#v", hpas.Items)
	}
	if jobs, err := projectAdminKubeClient.Batch().Jobs(projName).List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	} else if len(jobs.Items) > 0 {
		t.Fatalf("expected 0 Job objects, got %#v", jobs.Items)
	}
}

func TestExtensionsAPIDisabled(t *testing.T) {
	const projName = "ext-disabled-proj"

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Disable all extensions API versions
	masterConfig.KubernetesMasterConfig.DisabledAPIGroupVersions = map[string][]string{"extensions": {"*"}, "autoscaling": {"*"}, "batch": {"*"}}

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
	projectAdminClient, projectAdminKubeClient, projectAdminKubeConfig, err := testutil.GetClientForUser(*clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}
	projectAdminKubeClient15 := kclientset15.NewForConfigOrDie(projectAdminKubeConfig)
	if err := testutil.WaitForPolicyUpdate(projectAdminClient, projName, "get", expapi.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	legacyAutoscalers := legacyExtensionsAutoscaling{
		projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(projName),
		projectAdminKubeClient.Autoscaling().RESTClient(),
		projName,
	}

	// make sure extensions API objects cannot be listed or created
	if _, err := legacyAutoscalers.List(kapi.ListOptions{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error listing HPA, got %v", err)
	}
	if _, err := legacyAutoscalers.Create(&autoscaling.HorizontalPodAutoscaler{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error creating HPA, got %v", err)
	}
	if _, err := projectAdminKubeClient15.Extensions().Jobs(projName).List(kapiv1.ListOptions{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error listing jobs, got %v", err)
	}
	if _, err := projectAdminKubeClient15.Extensions().Jobs(projName).Create(&extensions_v1beta1.Job{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error creating job, got %v", err)
	}

	// Delete the containing project
	if err := testutil.DeleteAndWaitForNamespaceTermination(clusterAdminKubeClient, projName); err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}
}
