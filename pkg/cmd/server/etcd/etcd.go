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
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// RunEtcd starts an etcd server and runs it forever
func RunEtcd(etcdServerConfig *configapi.EtcdConfig) {

	config := etcdconfig.New()

	config.Addr = etcdServerConfig.Address
	config.BindAddr = etcdServerConfig.ServingInfo.BindAddress

	if configapi.UseTLS(etcdServerConfig.ServingInfo) {
		config.CAFile = etcdServerConfig.ServingInfo.ClientCA
		config.CertFile = etcdServerConfig.ServingInfo.ServerCert.CertFile
		config.KeyFile = etcdServerConfig.ServingInfo.ServerCert.KeyFile
	}

	config.Peer.Addr = etcdServerConfig.PeerAddress
	config.Peer.BindAddr = etcdServerConfig.PeerServingInfo.BindAddress

	if configapi.UseTLS(etcdServerConfig.PeerServingInfo) {
		config.Peer.CAFile = etcdServerConfig.PeerServingInfo.ClientCA
		config.Peer.CertFile = etcdServerConfig.PeerServingInfo.ServerCert.CertFile
		config.Peer.KeyFile = etcdServerConfig.PeerServingInfo.ServerCert.KeyFile
	}

	config.DataDir = etcdServerConfig.StorageDir
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
func GetAndTestEtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (*etcdclient.Client, error) {
	var etcdClient *etcdclient.Client

	if len(etcdClientInfo.ClientCert.CertFile) > 0 {
		tlsClient, err := etcdclient.NewTLSClient(
			etcdClientInfo.URLs,
			etcdClientInfo.ClientCert.CertFile,
			etcdClientInfo.ClientCert.KeyFile,
			etcdClientInfo.CA,
		)
		if err != nil {
			return nil, err
		}
		etcdClient = tlsClient
	} else if len(etcdClientInfo.CA) > 0 {
		etcdClient = etcdclient.NewClient(etcdClientInfo.URLs)
		err := etcdClient.AddRootCA(etcdClientInfo.CA)
		if err != nil {
			return nil, err
		}
	} else {
		etcdClient = etcdclient.NewClient(etcdClientInfo.URLs)
	}

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
func NewOpenShiftEtcdHelper(etcdClientInfo configapi.EtcdConnectionInfo) (helper tools.EtcdHelper, err error) {
	// Connect and setup etcd interfaces
	client, err := GetAndTestEtcdClient(etcdClientInfo)
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
