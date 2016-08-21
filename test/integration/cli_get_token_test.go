package integration

import (
	"bytes"
	"testing"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestCLIGetToken(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	anonymousConfig := clientcmd.AnonymousClientConfig(clusterAdminClientConfig)
	reader := bytes.NewBufferString("user\npass")
	accessToken, err := tokencmd.RequestToken(&anonymousConfig, reader, "", "")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(accessToken) == 0 {
		t.Error("Expected accessToken, but did not get one")
	}

	clientConfig := clientcmd.AnonymousClientConfig(clusterAdminClientConfig)
	clientConfig.BearerToken = accessToken
	osClient, err := client.New(&clientConfig)
	if err != nil {
		t.Fatal(err)
	}

	user, err := osClient.Users().Get("~")
	if err != nil {
		t.Fatal(err)
	}

	if user.Name != "user" {
		t.Errorf("expected %v, got %v", "user", user.Name)
	}
}
