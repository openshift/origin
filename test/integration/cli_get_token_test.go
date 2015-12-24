// +build integration,etcd

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
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	checkErr(t, err)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	checkErr(t, err)

	anonymousConfig := clientcmd.AnonymousClientConfig(*clusterAdminClientConfig)
	reader := bytes.NewBufferString("user\npass")
	accessToken, err := tokencmd.RequestToken(&anonymousConfig, reader, "", "")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(accessToken) == 0 {
		t.Error("Expected accessToken, but did not get one")
	}

	clientConfig := clientcmd.AnonymousClientConfig(*clusterAdminClientConfig)
	clientConfig.BearerToken = accessToken
	osClient, err := client.New(&clientConfig)
	checkErr(t, err)

	user, err := osClient.Users().Get("~")
	checkErr(t, err)

	if user.Name != "user" {
		t.Errorf("expected %v, got %v", "user", user.Name)
	}
}
