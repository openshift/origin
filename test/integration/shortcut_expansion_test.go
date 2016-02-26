// +build integration

package integration

import (
	"testing"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestFullExpansion(t *testing.T) {
	testutil.RequireEtcd(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapper := clientcmd.NewShortcutExpander(client.NewDiscoveryClient(clusterAdminClient.RESTClient), nil)

	if !sets.NewString(mapper.All...).Has("buildconfigs") {
		t.Errorf("expected buildconfigs, got: %v", mapper.All)
	}
}

func TestExpansionWithoutBuilds(t *testing.T) {
	testutil.RequireEtcd(t)

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	masterConfig.DisabledFeatures = configapi.AtomicDisabledFeatures
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapper := clientcmd.NewShortcutExpander(client.NewDiscoveryClient(clusterAdminClient.RESTClient), nil)

	if sets.NewString(mapper.All...).Has("buildconfigs") {
		t.Errorf("expected no buildconfigs, got: %v", mapper.All)
	}
}
