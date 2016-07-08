package kubernetes

import (
	"testing"
	"time"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/storage/etcd/etcdtest"
)

func TestNewMasterLeasesHasCorrectTTL(t *testing.T) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	defer server.Terminate(t)

	masterLeases := newMasterLeases(etcdStorage)
	if err := masterLeases.UpdateLease("1.2.3.4"); err != nil {
		t.Fatalf("error updating lease: %v", err)
	}

	etcdClient := server.Client
	keys := client.NewKeysAPI(etcdClient)
	resp, err := keys.Get(context.Background(), etcdtest.PathPrefix()+"/masterleases/1.2.3.4", nil)
	if err != nil {
		t.Fatalf("error getting key: %v", err)
	}
	ttl := resp.Node.TTLDuration()
	if ttl > 15*time.Second || ttl < 10*time.Second {
		t.Errorf("ttl %v should be ~ 15s", ttl)
	}
}
