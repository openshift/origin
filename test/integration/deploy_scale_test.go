// +build integration

package integration

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/util/wait"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestDeployScale(t *testing.T) {
	const namespace = "test-deploy-scale"

	testutil.RequireEtcd(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	checkErr(t, err)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	checkErr(t, err)
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	checkErr(t, err)
	_, err = testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, namespace, "my-test-user")
	checkErr(t, err)
	osClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "my-test-user")
	checkErr(t, err)

	config := deploytest.OkDeploymentConfig(0)
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{}
	config.Spec.Replicas = 1

	dc, err := osClient.DeploymentConfigs(namespace).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v %#v", err, config)
	}

	condition := func() (bool, error) {
		config, err := osClient.DeploymentConfigs(namespace).Get(dc.Name)
		if err != nil {
			return false, nil
		}
		return deployutil.HasSynced(config), nil
	}
	if err := wait.PollImmediate(500*time.Millisecond, 10*time.Second, condition); err != nil {
		t.Fatalf("Deployment config never synced: %v", err)
	}

	scale, err := osClient.DeploymentConfigs(namespace).GetScale(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get DeploymentConfig scale: %v", err)
	}
	if scale.Spec.Replicas != 1 {
		t.Fatalf("Expected scale.spec.replicas=1, got %#v", scale)
	}

	scaleUpdate := deployapi.ScaleFromConfig(dc)
	scaleUpdate.Spec.Replicas = 3

	updatedScale, err := osClient.DeploymentConfigs(namespace).UpdateScale(scaleUpdate)
	if err != nil {
		// If this complains about "Scale" not being registered in "v1", check the kind overrides in the API registration in SubresourceGroupVersionKind
		t.Fatalf("Couldn't update DeploymentConfig scale to %#v: %v", scaleUpdate, err)
	}
	if updatedScale.Spec.Replicas != 3 {
		t.Fatalf("Expected scale.spec.replicas=3, got %#v", scale)
	}

	persistedScale, err := osClient.DeploymentConfigs(namespace).GetScale(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get DeploymentConfig scale: %v", err)
	}
	if persistedScale.Spec.Replicas != 3 {
		t.Fatalf("Expected scale.spec.replicas=3, got %#v", scale)
	}
}
