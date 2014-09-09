// +build integration

package integration

import (
	"fmt"
	"math/rand"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
)

func newEtcdClient() *etcd.Client {
	return etcd.NewClient([]string{})
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
