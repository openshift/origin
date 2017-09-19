package integration

import (
	"testing"

	dockerregistryintegration "github.com/openshift/docker-registry/test/integration"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestV2RegistryGetTags(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("error starting master: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client config: %v", err)
	}
	user := "admin"
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client: %v", err)
	}
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, testutil.Namespace(), user); err != nil {
		t.Fatalf("error creating project: %v", err)
	}

	token, err := tokencmd.RequestToken(clusterAdminClientConfig, nil, user, "password")
	if err != nil {
		t.Fatalf("error requesting token: %v", err)
	}

	dockerregistryintegration.TestV2RegistryGetTags(t, clusterAdminKubeConfig, testutil.Namespace(), user, token)
}
