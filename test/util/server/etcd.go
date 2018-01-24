package server

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	etcdclientv3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/pkg/capnslog"
	"golang.org/x/net/context"

	"k8s.io/kubernetes/pkg/capabilities"

	"github.com/openshift/origin/test/util"
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

func dumpEtcd3(name string, etcd3 *etcdclientv3.Client) error {
	r, err := etcd3.KV.Get(context.Background(), "\x00", etcdclientv3.WithFromKey())
	if err != nil {
		return err
	}
	dumpFile, err := os.OpenFile(util.GetBaseDir()+"/etcd-dump-"+name+"-v3.json", os.O_WRONLY|os.O_CREATE, 0444)
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

func dumpEtcdOnFailure(t *testing.T, etcd3 *etcdclientv3.Client) {
	if !t.Failed() {
		return
	}
	name := t.Name()
	name = strings.Replace(name, "/", "_", -1)
	if etcd3 != nil {
		if err := dumpEtcd3(name, etcd3); err != nil {
			t.Logf("Unable to dump etcd3: %v", err)
		}
	}
}
