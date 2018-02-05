package integration

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func testWatchCacheWithConfig(t *testing.T, master *configapi.MasterConfig, expectedCacheSize, counterExampleCacheSize int) {
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(master)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// create client with high burst value, otherwise we can only do 5 changes per second
	config, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	config.Burst = expectedCacheSize
	patchClient, err := coreclient.NewForConfig(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// modify labels of default namespace expectedCacheSize + 1 times
	defaultNS, err := client.Core().Namespaces().Get("default", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	startVersion, err := strconv.Atoi(defaultNS.ResourceVersion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < expectedCacheSize+1; i++ {
		for r := 0; r < 10; r++ {
			defaultNS, err = patchClient.Namespaces().Patch("default", types.StrategicMergePatchType,
				[]byte(fmt.Sprintf(`{"metadata":{"labels":{"test":"%d"}}}`, i)))
			if err != nil && !kerrors.IsConflict(err) {
				t.Fatalf("unexpected patch error: %v", err)
			}
			if err == nil {
				break
			}
		}
		if err != nil {
			t.Fatalf("too many retries: %v", err)
		}
	}

	// do a versioned GET because it force the cache to sync
	_, err = client.Core().Namespaces().Get("default", metav1.GetOptions{ResourceVersion: defaultNS.ResourceVersion})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// try watch with very old resource version, not really expectedCacheSize versions back (there
	// might be other namespace changes which push the default namespace versions out of the cache.
	// Also note that the resource versions are global in etcd. So other resources will also lead
	// to resource version jumps.
	lastVersion, err := strconv.Atoi(defaultNS.ResourceVersion)
	if err != nil {
		t.Fatalf("unexpected error converting the resource version: %v", err)
	}
	w, err := client.Core().Namespaces().Watch(metav1.ListOptions{ResourceVersion: strconv.Itoa(lastVersion - (expectedCacheSize-counterExampleCacheSize)/2)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Stop()
	ev := <-w.ResultChan()
	if ev.Type == watch.Error {
		t.Fatalf("unexpected event of error type: %v", ev)
	}

	// try watch with an version that is too old
	w, err = client.Core().Namespaces().Watch(metav1.ListOptions{ResourceVersion: strconv.Itoa(startVersion - 1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer w.Stop()

	// the first event will be of error type
	goneErrMsg := "too old resource version"
	ev = <-w.ResultChan()
	if ev.Type != watch.Error {
		t.Fatalf("expected an %q error as first event, got: %v", goneErrMsg, ev)
	}
	status, ok := ev.Object.(*metav1.Status)
	if !ok {
		t.Fatalf("expected a metav1.Status object in first event, got: %v", ev.Object)
	}
	if !strings.Contains(status.Message, goneErrMsg) {
		t.Fatalf("expected an %q error, got: %v", goneErrMsg, err)
	}
}

func TestDefaultWatchCacheSize(t *testing.T) {
	master, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, master)

	// test that the origin default really applies and that we don't fall back to kube's default
	etcdOptions := apiserveroptions.NewEtcdOptions(&storagebackend.Config{})
	kubeDefaultCacheSize := etcdOptions.DefaultWatchCacheSize
	if kubeDefaultCacheSize != 100 {
		t.Fatalf("upstream DefaultWatchCacheSize changed to %d", kubeDefaultCacheSize)
	}
	if master.KubernetesMasterConfig.APIServerArguments == nil {
		master.KubernetesMasterConfig.APIServerArguments = configapi.ExtendedArguments{}
	}
	master.KubernetesMasterConfig.APIServerArguments["watch-cache-sizes"] = []string{"namespaces#100"}
	testWatchCacheWithConfig(t, master, 100, 0)
}

func TestWatchCacheSizeWithFlag(t *testing.T) {
	master, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, master)
	if master.KubernetesMasterConfig.APIServerArguments == nil {
		master.KubernetesMasterConfig.APIServerArguments = configapi.ExtendedArguments{}
	}
	master.KubernetesMasterConfig.APIServerArguments["watch-cache-sizes"] = []string{"namespaces#2000"}

	testWatchCacheWithConfig(t, master, 2000, 0)
}
