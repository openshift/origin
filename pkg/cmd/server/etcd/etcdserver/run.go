package etcdserver

import (
	"net/url"
	"strings"
	"time"

	"github.com/coreos/etcd/embed"
	"github.com/coreos/etcd/pkg/osutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/golang/glog"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

const defaultName = "openshift.local"

// RunEtcd starts an etcd server and runs it forever
func RunEtcd(etcdServerConfig *configapi.EtcdConfig) {
	cfg := embed.NewConfig()
	cfg.Debug = true
	cfg.Name = defaultName
	cfg.Dir = etcdServerConfig.StorageDir

	clientTLS := configapi.UseTLS(etcdServerConfig.ServingInfo)
	if clientTLS {
		cfg.ClientTLSInfo.CAFile = etcdServerConfig.ServingInfo.ClientCA
		cfg.ClientTLSInfo.CertFile = etcdServerConfig.ServingInfo.ServerCert.CertFile
		cfg.ClientTLSInfo.KeyFile = etcdServerConfig.ServingInfo.ServerCert.KeyFile
		cfg.ClientTLSInfo.ClientCertAuth = len(cfg.ClientTLSInfo.CAFile) > 0
	}
	u, err := types.NewURLs(addressToURLs(etcdServerConfig.ServingInfo.BindAddress, clientTLS))
	if err != nil {
		glog.Fatalf("Unable to build etcd peer URLs: %v", err)
	}
	cfg.LCUrls = []url.URL(u)

	peerTLS := configapi.UseTLS(etcdServerConfig.PeerServingInfo)
	if peerTLS {
		cfg.PeerTLSInfo.CAFile = etcdServerConfig.PeerServingInfo.ClientCA
		cfg.PeerTLSInfo.CertFile = etcdServerConfig.PeerServingInfo.ServerCert.CertFile
		cfg.PeerTLSInfo.KeyFile = etcdServerConfig.PeerServingInfo.ServerCert.KeyFile
		cfg.PeerTLSInfo.ClientCertAuth = len(cfg.PeerTLSInfo.CAFile) > 0
	}
	u, err = types.NewURLs(addressToURLs(etcdServerConfig.PeerServingInfo.BindAddress, peerTLS))
	if err != nil {
		glog.Fatalf("Unable to build etcd peer URLs: %v", err)
	}
	cfg.LPUrls = []url.URL(u)

	u, err = types.NewURLs(addressToURLs(etcdServerConfig.Address, clientTLS))
	if err != nil {
		glog.Fatalf("Unable to build etcd announce client URLs: %v", err)
	}
	cfg.ACUrls = []url.URL(u)

	u, err = types.NewURLs(addressToURLs(etcdServerConfig.PeerAddress, peerTLS))
	if err != nil {
		glog.Fatalf("Unable to build etcd announce peer URLs: %v", err)
	}
	cfg.APUrls = []url.URL(u)

	cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)

	osutil.HandleInterrupts()

	e, err := embed.StartEtcd(cfg)
	if err != nil {
		glog.Fatalf("Unable to start etcd: %v", err)
	}

	go func() {
		defer e.Close()

		select {
		case <-e.Server.ReadyNotify():
			glog.Infof("Started etcd at %s", etcdServerConfig.Address)
		case <-time.After(60 * time.Second):
			glog.Warning("etcd took too long to start, stopped")
			e.Server.Stop() // trigger a shutdown
		}
		glog.Fatalf("etcd has returned an error: %v", <-e.Err())
	}()
}

// addressToURLs turns a host:port comma delimited list into an array valid
// URL strings with the appropriate prefix for the TLS mode.
func addressToURLs(addr string, isTLS bool) []string {
	addrs := strings.Split(addr, ",")
	for i := range addrs {
		if isTLS {
			addrs[i] = "https://" + addrs[i]
		} else {
			addrs[i] = "http://" + addrs[i]
		}
	}
	return addrs
}
