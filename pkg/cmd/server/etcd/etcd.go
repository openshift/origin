package etcd

import (
	"fmt"
	"time"

	etcdconfig "github.com/coreos/etcd/config"
	"github.com/coreos/etcd/etcd"
	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
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
	}, 500*time.Millisecond)
	<-server.ReadyNotify()
}

// getAndTestEtcdClient creates an etcd client based on the provided config and waits
// until etcd server is reachable. It errors out and exits if the server cannot
// be reached for a certain amount of time.
func GetAndTestEtcdClient(etcdURL string) (*etcdclient.Client, error) {
	etcdServers := []string{etcdURL}
	etcdClient := etcdclient.NewClient(etcdServers)

	for i := 0; ; i++ {
		// TODO: make sure this works with etcd2 (root key may not exist)
		_, err := etcdClient.Get("/", false, false)
		if err == nil || tools.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			return nil, fmt.Errorf("Could not reach etcd: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	return etcdClient, nil
}

// newOpenShiftEtcdHelper returns an EtcdHelper for the provided arguments or an error if the version
// is incorrect.
func NewOpenShiftEtcdHelper(etcdURL string) (helper tools.EtcdHelper, err error) {
	// Connect and setup etcd interfaces
	client, err := GetAndTestEtcdClient(etcdURL)
	if err != nil {
		return tools.EtcdHelper{}, err
	}

	version := latest.Version
	interfaces, err := latest.InterfacesFor(version)
	if err != nil {
		return helper, err
	}
	return tools.NewEtcdHelper(client, interfaces.Codec), nil
}
