package etcdserver

import (
	"fmt"

	"github.com/golang/glog"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// RunEtcd starts an etcd server and runs it forever
func RunEtcd(etcdServerConfig *configapi.EtcdConfig) {
	cfg := &config{
		name: defaultName,
		dir:  etcdServerConfig.StorageDir,

		TickMs:       100,
		ElectionMs:   1000,
		maxSnapFiles: 5,
		maxWalFiles:  5,

		initialClusterToken: "etcd-cluster",
	}
	var err error
	if configapi.UseTLS(etcdServerConfig.ServingInfo) {
		cfg.clientTLSInfo.CAFile = etcdServerConfig.ServingInfo.ClientCA
		cfg.clientTLSInfo.CertFile = etcdServerConfig.ServingInfo.ServerCert.CertFile
		cfg.clientTLSInfo.KeyFile = etcdServerConfig.ServingInfo.ServerCert.KeyFile
	}
	if cfg.lcurls, err = urlsFromStrings(etcdServerConfig.ServingInfo.BindAddress, cfg.clientTLSInfo); err != nil {
		glog.Fatalf("Unable to build etcd client URLs: %v", err)
	}

	if configapi.UseTLS(etcdServerConfig.PeerServingInfo) {
		cfg.peerTLSInfo.CAFile = etcdServerConfig.PeerServingInfo.ClientCA
		cfg.peerTLSInfo.CertFile = etcdServerConfig.PeerServingInfo.ServerCert.CertFile
		cfg.peerTLSInfo.KeyFile = etcdServerConfig.PeerServingInfo.ServerCert.KeyFile
	}
	if cfg.lpurls, err = urlsFromStrings(etcdServerConfig.PeerServingInfo.BindAddress, cfg.peerTLSInfo); err != nil {
		glog.Fatalf("Unable to build etcd peer URLs: %v", err)
	}

	if cfg.acurls, err = urlsFromStrings(etcdServerConfig.Address, cfg.clientTLSInfo); err != nil {
		glog.Fatalf("Unable to build etcd announce client URLs: %v", err)
	}
	if cfg.apurls, err = urlsFromStrings(etcdServerConfig.PeerAddress, cfg.peerTLSInfo); err != nil {
		glog.Fatalf("Unable to build etcd announce peer URLs: %v", err)
	}

	if err := cfg.resolveUrls(); err != nil {
		glog.Fatalf("Unable to resolve etcd URLs: %v", err)
	}

	cfg.initialCluster = fmt.Sprintf("%s=%s", cfg.name, cfg.apurls[0].String())

	stopped, err := startEtcd(cfg)
	if err != nil {
		glog.Fatalf("Unable to start etcd: %v", err)
	}
	go func() {
		glog.Infof("Started etcd at %s", etcdServerConfig.Address)
		<-stopped
	}()
}
