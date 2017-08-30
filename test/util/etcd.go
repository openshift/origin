package util

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	etcdclient "github.com/coreos/etcd/client"
	etcdclientv3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/pkg/capnslog"
	"golang.org/x/net/context"

	etcdtest "k8s.io/apiserver/pkg/storage/etcd/testing"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/capabilities"

	serveretcd "github.com/openshift/origin/pkg/cmd/server/etcd"
)

func init() {
	capabilities.SetForTests(capabilities.Capabilities{
		AllowPrivileged: true,
	})
	flag.Set("v", "4")
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

type EtcdTestServer struct {
	*etcdtest.EtcdTestServer
}

// RequireEtcd verifies if the etcd is running and accessible for testing
func RequireEtcd(t *testing.T) EtcdTestServer {
	s, _ := etcdtest.NewUnsecuredEtcd3TestClientServer(t, kapi.Scheme)
	t.Logf("endpoints: %v", s.V3Client.Endpoints())
	url = s.V3Client.Endpoints()[0]
	return EtcdTestServer{s}
}

// RequireEtcd verifies if the etcd is running and accessible for testing
// TODO: remove use of etcd2 specific apis in 1.6
func RequireEtcd2(t *testing.T) EtcdTestServer {
	s := etcdtest.NewUnsecuredEtcdTestClientServer(t)
	url = s.Client.Endpoints()[0]
	return EtcdTestServer{s}
}

func RequireEtcd3(t *testing.T) (*etcdtest.EtcdTestServer, *storagebackend.Config) {
	s, c := etcdtest.NewUnsecuredEtcd3TestClientServer(t, kapi.Scheme)
	url = s.V3Client.Endpoints()[0]
	return s, c
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

func NewEtcd3Client() *etcdclientv3.Client {
	client, _ := MakeNewEtcd3Client()
	return client
}

func MakeNewEtcd3Client() (*etcdclientv3.Client, error) {
	etcdServers := []string{GetEtcdURL()}

	cfg := etcdclientv3.Config{
		Endpoints: etcdServers,
	}
	client, err := etcdclientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	return client, serveretcd.TestEtcdClientV3(client)
}

func GetEtcdURL() string {
	if len(url) == 0 {
		panic("can't invoke GetEtcdURL prior to calling RequireEtcd")
	}
	return url
}

func (s EtcdTestServer) DumpEtcdOnFailure(t *testing.T) {
	defer func() {
		s.Terminate(t)
		dir := s.DataDir
		if len(dir) > 0 {
			t.Logf("Removing etcd dir %s", dir)
			if err := os.RemoveAll(dir); err != nil {
				t.Errorf("Unable to remove contents of etcd data directory %s: %v", dir, err)
			}
		} else {
			t.Logf("No data directory, nothing to clean up")
		}
	}()
	if !t.Failed() {
		return
	}

	DumpEtcdOnFailure(t, s.Client, s.V3Client)
}

func dumpEtcd2(t *testing.T, name string, etcd2 etcdclient.Client) error {
	keyClient := etcdclient.NewKeysAPI(etcd2)

	response, err := keyClient.Get(context.Background(), "/", &etcdclient.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		return err
	}
	jsonResponse, err := json.Marshal(response.Node)
	if err != nil {
		return err
	}

	t.Logf("dumping etcd to %q", GetBaseDir()+"/etcd-dump-"+name+"-v2.json")
	dumpFile, err := os.OpenFile(GetBaseDir()+"/etcd-dump-"+name+"-v2.json", os.O_WRONLY|os.O_CREATE, 0444)
	if err != nil {
		return err
	}
	defer dumpFile.Close()
	_, err = dumpFile.Write(jsonResponse)
	if err != nil {
		return err
	}
	return nil
}

func dumpEtcd3(t *testing.T, name string, etcd3 *etcdclientv3.Client) error {
	r, err := etcd3.KV.Get(context.Background(), "\x00", etcdclientv3.WithFromKey())
	if err != nil {
		return err
	}
	dumpFile, err := os.OpenFile(GetBaseDir()+"/etcd-dump-"+name+"-v3.json", os.O_WRONLY|os.O_CREATE, 0444)
	if err != nil {
		return err
	}
	defer dumpFile.Close()
	w := json.NewEncoder(dumpFile)
	result := struct {
		Key            string
		Value          []byte
		CreateRevision int64
		ModRevision    int64
	}{}
	for _, v := range r.Kvs {
		result.Key = string(v.Key)
		result.Value = v.Value
		result.CreateRevision, result.ModRevision = v.CreateRevision, v.ModRevision
		if err := w.Encode(result); err != nil {
			return err
		}
	}
	return nil
}

func DumpEtcdOnFailure(t *testing.T, etcd2 etcdclient.Client, etcd3 *etcdclientv3.Client) {
	if !t.Failed() {
		return
	}
	name := t.Name()
	name = strings.Replace(name, "/", "_", -1)
	if etcd2 != nil {
		if err := dumpEtcd2(t, name, etcd2); err != nil {
			t.Logf("Unable to dump etcd2: %v", err)
		}
	}
	if etcd3 != nil {
		if err := dumpEtcd3(t, name, etcd3); err != nil {
			t.Logf("Unable to dump etcd3: %v", err)
		}
	}
}
