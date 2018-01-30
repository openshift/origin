package master

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage/etcd/etcdtest"
	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
)

func TestNewMasterLeasesHasCorrectTTL(t *testing.T) {
	server, etcdStorage := etcdtesting.NewUnsecuredEtcd3TestClientServer(t)
	etcdStorage.Codec = testapi.Groups[""].StorageCodec()

	restOptions := generic.RESTOptions{StorageConfig: etcdStorage, Decorator: generic.UndecoratedStorage, DeleteCollectionWorkers: 1}
	storageInterface, _ := restOptions.Decorator(restOptions.StorageConfig, nil, "masterleases", nil, nil, nil, nil)
	defer server.Terminate(t)

	masterLeases := newMasterLeases(storageInterface, 15)
	if err := masterLeases.UpdateLease("1.2.3.4"); err != nil {
		t.Fatalf("error updating lease: %v", err)
	}

	etcdClient := server.V3Client
	resp, err := etcdClient.Get(context.Background(), etcdtest.PathPrefix()+"/masterleases/1.2.3.4", nil)
	if err != nil {
		t.Fatalf("error getting key: %v", err)
	}
	ttl := resp.Node.TTLDuration()
	if ttl > 15*time.Second || ttl < 10*time.Second {
		t.Errorf("ttl %v should be ~ 15s", ttl)
	}
}
