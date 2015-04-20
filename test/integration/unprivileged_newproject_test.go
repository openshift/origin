// +build integration,!no-etcd

package integration

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/client"

	osc "github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
)

func TestUnprivilegedNewProject(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	valerieClientConfig := *clusterAdminClientConfig
	valerieClientConfig.Username = ""
	valerieClientConfig.Password = ""
	valerieClientConfig.BearerToken = ""
	valerieClientConfig.CertFile = ""
	valerieClientConfig.KeyFile = ""
	valerieClientConfig.CertData = nil
	valerieClientConfig.KeyData = nil

	accessToken, err := tokencmd.RequestToken(&valerieClientConfig, nil, "valerie", "security!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig.BearerToken = accessToken
	valerieOpenshiftClient, err := client.New(&valerieClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requestProject := osc.NewProjectOptions{
		ProjectName: "new-project",
		DisplayName: "display name here",
		Description: "the special description",

		Client: valerieOpenshiftClient,
	}

	if err := requestProject.Run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	waitForProject(t, valerieOpenshiftClient, "new-project", 5*time.Second, 10)
}
