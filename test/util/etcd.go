package util

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

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
	logEtcd()
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
			glog.Fatalf("Unable to delete key: %v", err)
		}
	}
}

func NewEtcdClient() *etcd.Client {
	etcdServers := []string{GetEtcdURL()}

	return etcd.NewClient(etcdServers)
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
	logDir := os.Getenv("LOG_DIR")
	if len(logDir) == 0 {
		logDir = "/tmp/origin/e2e/"
	}
	os.MkdirAll(logDir, os.FileMode(0700))

	logFile := fmt.Sprintf("integration-etcd-%d", time.Now().Unix())
	if testName := os.Getenv("TEST_NAME"); len(testName) > 0 {
		logFile += "-" + testName
	}
	logFile += ".log"

	fileWriter, err := os.Create(filepath.Join(logDir, logFile))
	if err != nil {
		panic(err)
	}

	etcd.SetLogger(log.New(fileWriter, "go-etcd", log.LstdFlags))
}

func withEtcdKey(f func(string)) {
	prefix := fmt.Sprintf("/test-%d", rand.Int63())
	defer NewEtcdClient().Delete(prefix, true)
	f(prefix)
}
