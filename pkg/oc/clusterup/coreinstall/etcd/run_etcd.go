package etcd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"text/template"

	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
)

type EtcdConfig struct {
	Image, ImagePullPolicy string

	// StaticPodDir is the kubelet location for static pod manifests.
	StaticPodDir string

	// TlsDir is the location where the TLS assets are.
	TlsDir string

	// EtcdDataDir is location where etcd will store data.
	EtcdDataDir string
}

// Start creates an etcd pod manifest in StaticPodDir.
func (opt *EtcdConfig) Start() error {
	etcdYaml, err := manifests.Asset("install/etcd/etcd.yaml")
	if err != nil {
		return err
	}
	tpl, err := template.New("etcd.yaml").Parse(string(etcdYaml))
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, opt); err != nil {
		return fmt.Errorf("failed to render static etcd pod manifest: %v", err)
	}

	fname := filepath.Join(opt.StaticPodDir, "etcd.yaml")
	if err := ioutil.WriteFile(fname, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write static etcd pod %q: %v", fname, err)
	}

	return nil
}
