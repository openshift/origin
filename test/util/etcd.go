package util

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"

	newetcdclient "github.com/coreos/etcd/client"
	"github.com/coreos/go-etcd/etcd"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/capabilities"
)

func init() {
	capabilities.SetForTests(capabilities.Capabilities{
		AllowPrivileged: true,
	})
	flag.Set("v", "5")
}

// RequireEtcd verifies if the etcd is running and accessible for testing
func RequireEtcd() {
	if _, err := NewEtcdClient().Get("/", false, false); err != nil {
		glog.Fatalf("unable to connect to etcd for testing: %v", err)
	}
}

// DeleteAllEtcdKeys removes all etcd keys
func DeleteAllEtcdKeys() {
	client := NewEtcdClient()
	keys, err := client.Get("/", false, false)
	if err != nil {
		glog.Fatalf("Unable to list root etcd keys: %v", err)
	}
	for _, node := range keys.Node.Nodes {
		glog.Infof("Deleting %#v (child of %#v)", node, keys.Node)
		if _, err := client.Delete(node.Key, true); err != nil {
			// This should't happen, but something is confusing etcd: https://github.com/openshift/origin/issues/6065
			// This is strictly test setup.  The test will still be valid if there are no root keys when this method is over.
			glog.Errorf("Unable to delete key: %v", err)
		}
	}

	keys, err = client.Get("/", false, false)
	if err != nil {
		glog.Fatalf("Unable to list root etcd keys: %v", err)
	}
	if len(keys.Node.Nodes) > 0 {
		glog.Fatalf("Unable to deletes key: %v", keys)
	}
}

func NewEtcdClient() *etcd.Client {
	etcdServers := []string{GetEtcdURL()}

	return etcd.NewClient(etcdServers)
}

func MakeNewEtcdClient() (newetcdclient.Client, error) {
	etcdServers := []string{GetEtcdURL()}

	cfg := newetcdclient.Config{
		Endpoints: etcdServers,
	}
	return newetcdclient.New(cfg)
}

func GetEtcdURL() string {
	etcdFromEnv := os.Getenv("ETCD_SERVER")
	if len(etcdFromEnv) > 0 {
		return etcdFromEnv
	}

	etcdPort := "4001"
	if len(os.Getenv("ETCD_PORT")) > 0 {
		etcdPort = os.Getenv("ETCD_PORT")
	}

	return fmt.Sprintf("http://127.0.0.1:%s", etcdPort)
}

func logEtcd() {
	etcd.SetLogger(log.New(os.Stderr, "go-etcd", log.LstdFlags))
}

func withEtcdKey(f func(string)) {
	prefix := fmt.Sprintf("/test-%d", rand.Int63())
	defer NewEtcdClient().Delete(prefix, true)
	f(prefix)
}
