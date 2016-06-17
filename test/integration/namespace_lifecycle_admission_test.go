// +build integration

package integration

import (
	"testing"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestNamespaceLifecycleAdmission(t *testing.T) {
	testutil.RequireEtcd(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	checkErr(t, err)
	clusterAdminClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	checkErr(t, err)

	for _, ns := range []string{"default", "openshift", "openshift-infra"} {
		if err := clusterAdminClient.Namespaces().Delete(ns); err == nil {
			t.Fatalf("expected error deleting %q namespace, got none", ns)
		}
	}
}
