package etcd

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	etcdconfig "github.com/coreos/etcd/config"
	"github.com/coreos/etcd/etcd"
	"github.com/golang/glog"
)

// Config is an object that can run an etcd server
type Config struct {
	BindAddr     string
	PeerBindAddr string
	MasterAddr   string
	EtcdDir      string
}

// Run starts an etcd server and runs it forever
func (c *Config) Run() {
	config := etcdconfig.New()
	config.Addr = c.MasterAddr
	config.BindAddr = c.BindAddr
	config.Peer.BindAddr = c.PeerBindAddr
	config.DataDir = c.EtcdDir
	config.Name = "openshift.local"

	server := etcd.New(config)
	go util.Forever(func() {
		glog.Infof("Started etcd at %s", config.Addr)
		server.Run()
		glog.Fatalf("etcd died, exiting.")
	}, 0)
	<-server.ReadyNotify()
}
