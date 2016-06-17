// +build integration

package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
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

	scale, err := osClient.DeploymentConfigs(namespace).GetScale(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get DeploymentConfig scale: %v", err)
	}
	if scale.Spec.Replicas != 1 {
		t.Fatalf("Expected scale.spec.replicas=1, got %#v", scale)
	}

	scaleUpdate := &extensions.Scale{
		ObjectMeta: kapi.ObjectMeta{
			Name:      dc.Name,
			Namespace: namespace,
		},
		Spec: extensions.ScaleSpec{Replicas: 3},
	}
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
