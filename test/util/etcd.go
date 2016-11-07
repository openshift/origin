package util

import (
	"encoding/json"
	"flag"
	"os"
	goruntime "runtime"
	"strings"
	"testing"

	"golang.org/x/net/context"

	"github.com/coreos/pkg/capnslog"

	etcdclient "github.com/coreos/etcd/client"

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

func NewEtcdClient() etcdclient.Client {
	client, _ := MakeNewEtcdClient()
	return client
}

func MakeNewEtcdClient() (etcdclient.Client, error) {
	etcdServers := []string{GetEtcdURL()}

	cfg := etcdclient.Config{
		Endpoints: etcdServers,
	}
	client, err := etcdclient.New(cfg)
	if err != nil {
		return nil, err
	}
	return client, serveretcd.TestEtcdClient(client)
}

func GetEtcdURL() string {
	if len(url) == 0 {
		panic("can't invoke GetEtcdURL prior to calling RequireEtcd")
	}
	return url
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
	keyClient := etcdclient.NewKeysAPI(client)

	response, err := keyClient.Get(context.Background(), "/", &etcdclient.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		t.Logf("error dumping etcd: %v", err)
		return
	}
	jsonResponse, err := json.Marshal(response.Node)
	if err != nil {
		t.Logf("error encoding etcd dump: %v", err)
		return
	}

	t.Logf("dumping etcd to %q", GetBaseDir()+"/etcd-dump-"+name+".json")
	dumpFile, err := os.OpenFile(GetBaseDir()+"/etcd-dump-"+name+".json", os.O_WRONLY|os.O_CREATE, 0444)
	if err != nil {
		t.Logf("error writing etcd dump: %v", err)
		return
	}
	defer dumpFile.Close()
	_, err = dumpFile.Write(jsonResponse)
	if err != nil {
		t.Logf("error writing etcd dump: %v", err)
		return
	}

}
