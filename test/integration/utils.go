// +build integration

package integration

import (
	"fmt"
	"math/rand"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

func newEtcdClient() *etcd.Client {
	return etcd.NewClient([]string{"http://127.0.0.1:4001"})
}

func requireEtcd() {
	if _, err := newEtcdClient().Get("/", false, false); err != nil {
		glog.Fatalf("unable to connect to etcd for integration testing: %v", err)
	}
}

func withEtcdKey(f func(string)) {
	prefix := fmt.Sprintf("/test-%d", rand.Int63())
	defer newEtcdClient().Delete(prefix, true)
	f(prefix)
}

func deleteAllEtcdKeys() {
	client := newEtcdClient()
	keys, err := client.Get("/", false, false)
	if err != nil {
		glog.Fatalf("Unable to list root etcd keys: %v", err)
	}
	for _, node := range keys.Node.Nodes {
		if _, err := client.Delete(node.Key, true); err != nil {
			glog.Fatalf("Unable delete key: %v", err)
		}
	}
}
