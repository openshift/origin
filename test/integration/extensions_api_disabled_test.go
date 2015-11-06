// +build integration,etcd

package integration

import (
	"testing"
	"time"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	"k8s.io/kubernetes/pkg/api/errors"
	expapi "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/wait"
)

func TestExtensionsAPIDisabled(t *testing.T) {
	const projName = "ext-disabled-proj"

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Disable all extensions API versions
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
	// TODO: switch to checking "list","horizontalpodautoscalers" once SAR supports API groups
	if err := testutil.WaitForPolicyUpdate(projectAdminClient, projName, "get", "pods", true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	// make sure extensions API objects cannot be listed or created
	if _, err := projectAdminKubeClient.Extensions().HorizontalPodAutoscalers(projName).List(labels.Everything(), fields.Everything()); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error listing HPA, got %v", err)
	}
	if _, err := projectAdminKubeClient.Extensions().HorizontalPodAutoscalers(projName).Create(&expapi.HorizontalPodAutoscaler{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error creating HPA, got %v", err)
	}
	if _, err := projectAdminKubeClient.Extensions().Jobs(projName).List(labels.Everything(), fields.Everything()); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error listing jobs, got %v", err)
	}
	if _, err := projectAdminKubeClient.Extensions().Jobs(projName).Create(&expapi.Job{}); !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound error creating job, got %v", err)
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
}
