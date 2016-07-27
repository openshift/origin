package util

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/coreos/pkg/capnslog"

	newetcdclient "github.com/coreos/etcd/client"
	"github.com/coreos/go-etcd/etcd"

	"k8s.io/kubernetes/pkg/capabilities"
	etcdtest "k8s.io/kubernetes/pkg/storage/etcd/testing"

	serveretcd "github.com/openshift/origin/pkg/cmd/server/etcd"
)

func init() {
	capabilities.SetForTests(capabilities.Capabilities{
		AllowPrivileged: true,
	})
	flag.Set("v", "5")
	if len(os.Getenv("OS_TEST_VERBOSE_ETCD")) > 0 {
		capnslog.SetGlobalLogLevel(capnslog.DEBUG)
		capnslog.SetFormatter(capnslog.NewGlogFormatter(os.Stderr))
	} else {
		capnslog.SetGlobalLogLevel(capnslog.INFO)
		capnslog.SetFormatter(capnslog.NewGlogFormatter(os.Stderr))
	}
}

// url is the url for the launched etcd server
var url string

// RequireEtcd verifies if the etcd is running and accessible for testing
func RequireEtcd(t *testing.T) *etcdtest.EtcdTestServer {
	s := etcdtest.NewUnsecuredEtcdTestClientServer(t)
	url = s.Client.Endpoints()[0]
	return s
}

func NewEtcdClient() *etcd.Client {
	etcdServers := []string{GetEtcdURL()}

	client := etcd.NewClient(etcdServers)
	if err := serveretcd.TestEtcdClient(client); err != nil {
		panic(err)
	}
	return client
}

func MakeNewEtcdClient() (newetcdclient.Client, error) {
	etcdServers := []string{GetEtcdURL()}

	cfg := newetcdclient.Config{
		Endpoints: etcdServers,
	}
	client, err := newetcdclient.New(cfg)
	if err != nil {
		return nil, err
	}
	return client, serveretcd.TestNewEtcdClient(client)
}

func GetEtcdURL() string {
	if len(url) == 0 {
		panic("can't invoke GetEtcdURL prior to calling RequireEtcd")
	}
	return url
}

func logEtcd() {
	etcd.SetLogger(log.New(os.Stderr, "go-etcd", log.LstdFlags))
}

func withEtcdKey(f func(string)) {
	prefix := fmt.Sprintf("/test-%d", rand.Int63())
	defer NewEtcdClient().Delete(prefix, true)
	f(prefix)
}

func DumpEtcdOnFailure(t *testing.T) {
	if !t.Failed() {
		return
	}

	pc := make([]uintptr, 10)
	goruntime.Callers(2, pc)
	f := goruntime.FuncForPC(pc[0])
	last := strings.LastIndex(f.Name(), "Test")
	if last == -1 {
		last = 0
	}
	name := f.Name()[last:]

	client := NewEtcdClient()
	etcdResponse, err := client.RawGet("/", false, true)
	if err != nil {
		t.Logf("error dumping etcd: %v", err)
		return
	}

	if err := ioutil.WriteFile(GetBaseDir()+"/etcd-dump-"+name+".json", etcdResponse.Body, os.FileMode(0444)); err != nil {
		t.Logf("error dumping etcd: %v", err)
		return
	}
}
