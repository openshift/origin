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
	etcdclientv3 "github.com/coreos/etcd/clientv3"

	etcdtest "k8s.io/apiserver/pkg/storage/etcd/testing"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/capabilities"
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

type EtcdTestServer struct {
	*etcdtest.EtcdTestServer
	t *testing.T
}

// RequireEtcd verifies if the etcd is running and accessible for testing
// TODO: make this use etcd3
func RequireEtcd(t *testing.T) EtcdTestServer {
	return RequireEtcd2(t)
}

// RequireEtcd verifies if the etcd is running and accessible for testing
// TODO: remove use of etcd2 specific apis in 1.6
func RequireEtcd2(t *testing.T) EtcdTestServer {
	s := etcdtest.NewUnsecuredEtcdTestClientServer(t)
	url = s.Client.Endpoints()[0]
	return EtcdTestServer{EtcdTestServer: s, t: t}
}

func RequireEtcd3(t *testing.T) EtcdTestServer {
	s, _ := etcdtest.NewUnsecuredEtcd3TestClientServer(t, kapi.Scheme)
	url = s.V3Client.Endpoints()[0]
	return EtcdTestServer{EtcdTestServer: s, t: t}
}

func GetEtcdURL() string {
	if len(url) == 0 {
		panic("can't invoke GetEtcdURL prior to calling RequireEtcd")
	}
	return url
}

func (s EtcdTestServer) DumpEtcdOnFailure() {
	defer func() {
		s.Terminate(s.t)
		os.RemoveAll(s.DataDir)
	}()
	if !s.t.Failed() {
		return
	}
	v3 := s.V3Client != nil
	if v3 {
		s.dumpEtcd3()
	} else {
		s.dumpEtcd2()
	}
}

func (s EtcdTestServer) dumpEtcd2() {
	response, err := etcdclient.NewKeysAPI(s.Client).Get(context.Background(), "/", &etcdclient.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		s.t.Logf("error dumping etcd: %v", err)
		return
	}

	s.writeEtcdDump(response.Node)
}

func (s EtcdTestServer) dumpEtcd3() {
	response, err := s.V3Client.KV.Get(context.Background(), "/", etcdclientv3.WithPrefix(), etcdclientv3.WithSort(etcdclientv3.SortByKey, etcdclientv3.SortDescend))
	if err != nil {
		s.t.Logf("error dumping etcd: %v", err)
		return
	}

	kvData := []etcd3kv{}
	for _, kvs := range response.Kvs {
		obj, _, err := kapi.Codecs.UniversalDeserializer().Decode(kvs.Value, nil, nil)
		if err != nil {
			s.t.Logf("error decoding value %s: %v", string(kvs.Value), err)
			continue
		}
		objJSON, err := json.Marshal(obj)
		if err != nil {
			s.t.Logf("error encoding object %#v as JSON: %v", obj, err)
			continue
		}
		kvData = append(kvData, etcd3kv{string(kvs.Key), string(objJSON)})
	}

	s.writeEtcdDump(kvData)
}

func (s EtcdTestServer) writeEtcdDump(data interface{}) {
	jsonResponse, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		s.t.Logf("error encoding etcd dump: %v", err)
		return
	}
	name := getCallingTestName()
	s.t.Logf("dumping etcd to %q", GetBaseDir()+"/etcd-dump-"+name+".json")
	dumpFile, err := os.OpenFile(GetBaseDir()+"/etcd-dump-"+name+".json", os.O_WRONLY|os.O_CREATE, 0444)
	if err != nil {
		s.t.Logf("error writing etcd dump: %v", err)
		return
	}
	defer dumpFile.Close()
	_, err = dumpFile.Write(jsonResponse)
	if err != nil {
		s.t.Logf("error writing etcd dump: %v", err)
		return
	}
}

func getCallingTestName() string {
	pc := make([]uintptr, 10)
	goruntime.Callers(5, pc)
	f := goruntime.FuncForPC(pc[0])
	last := strings.LastIndex(f.Name(), "Test")
	if last == -1 {
		last = 0
	}
	return f.Name()[last:]
}

type etcd3kv struct {
	Key, Value string
}
