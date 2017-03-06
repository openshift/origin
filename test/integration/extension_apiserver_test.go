package integration

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestExtensionAPIServerConfigMap(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var configmap *kapi.ConfigMap
	err = utilwait.PollImmediate(50*time.Millisecond, 10*time.Second, func() (bool, error) {
		configmap, err = clusterAdminKubeClient.Core().ConfigMaps(kapi.NamespaceSystem).Get("extension-apiserver-authentication")
		if err == nil {
			return true, nil
		}
		if kapierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := configmap.Data["client-ca-file"]; !ok {
		t.Fatal("missing client-ca-file")
	}
}
