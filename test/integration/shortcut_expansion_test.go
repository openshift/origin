package integration

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestCachingDiscoveryClient(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, originKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	originClient, err := testutil.GetClusterAdminClient(originKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resourceType := "buildconfigs"

	originDiscoveryClient := client.NewDiscoveryClient(originClient.RESTClient)
	originUncachedMapper := clientcmd.NewShortcutExpander(originDiscoveryClient, nil)
	if !sets.NewString(originUncachedMapper.All...).Has(resourceType) {
		t.Errorf("expected %v, got: %v", resourceType, originUncachedMapper.All)
	}

	cacheDir, err := ioutil.TempDir("", "TestCachingDiscoveryClient")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if !t.Failed() {
			os.RemoveAll(cacheDir)
		}
	}()
	// this client should prime the cache
	originCachedDiscoveryClient := clientcmd.NewCachedDiscoveryClient(originDiscoveryClient, cacheDir, time.Duration(10*time.Minute))
	originCachedMapper := clientcmd.NewShortcutExpander(originCachedDiscoveryClient, nil)
	if !sets.NewString(originCachedMapper.All...).Has(resourceType) {
		t.Errorf("expected %v, got: %v", resourceType, originCachedMapper.All)
	}

	// this client will fail if the cache fails
	unbackedDiscoveryClient := clientcmd.NewCachedDiscoveryClient(nil, cacheDir, time.Duration(10*time.Minute))
	unbackedOriginCachedMapper := clientcmd.NewShortcutExpander(unbackedDiscoveryClient, nil)
	if !sets.NewString(unbackedOriginCachedMapper.All...).Has(resourceType) {
		t.Errorf("expected %v, got: %v", resourceType, unbackedOriginCachedMapper.All)
	}

	atomicConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	atomicConfig.DisabledFeatures = configapi.AtomicDisabledFeatures
	atomicConfig.DNSConfig = nil
	atomicKubeConfig, err := testserver.StartConfiguredMasterAPI(atomicConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	atomicClient, err := testutil.GetClusterAdminClient(atomicKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	atomicDiscoveryClient := client.NewDiscoveryClient(atomicClient.RESTClient)
	atomicUncachedMapper := clientcmd.NewShortcutExpander(atomicDiscoveryClient, nil)
	if sets.NewString(atomicUncachedMapper.All...).Has(resourceType) {
		t.Errorf("expected no %v, got: %v", resourceType, atomicUncachedMapper.All)
	}

	// this client will give different results if the cache fails
	conflictingDiscoveryClient := clientcmd.NewCachedDiscoveryClient(atomicDiscoveryClient, cacheDir, time.Duration(10*time.Minute))
	conflictingCachedMapper := clientcmd.NewShortcutExpander(conflictingDiscoveryClient, nil)
	if !sets.NewString(conflictingCachedMapper.All...).Has(resourceType) {
		t.Errorf("expected %v, got: %v", resourceType, conflictingCachedMapper.All)
	}

	// this client should give different results as result of a live lookup
	expiredDiscoveryClient := clientcmd.NewCachedDiscoveryClient(atomicDiscoveryClient, cacheDir, time.Duration(-1*time.Second))
	expiredAtomicCachedMapper := clientcmd.NewShortcutExpander(expiredDiscoveryClient, nil)
	if sets.NewString(expiredAtomicCachedMapper.All...).Has(resourceType) {
		t.Errorf("expected no %v, got: %v", resourceType, expiredAtomicCachedMapper.All)
	}

}
