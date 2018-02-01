package etcdserver

import (
	"net/url"
	"strings"
	"time"

	"github.com/coreos/etcd/embed"
	"github.com/coreos/etcd/pkg/osutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/go-semver/semver"
	"github.com/golang/glog"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

const defaultName = "openshift.local"

// RunEtcd starts an etcd server and runs it forever
func RunEtcd(etcdServerConfig *configapi.EtcdConfig) {
	cfg := embed.NewConfig()
	cfg.Debug = true
	cfg.Name = defaultName
	cfg.Dir = etcdServerConfig.StorageDir

	clientTLS := true
	cfg.ClientTLSInfo.CAFile = etcdServerConfig.ServingInfo.ClientCA
	cfg.ClientTLSInfo.CertFile = etcdServerConfig.ServingInfo.ServerCert.CertFile
	cfg.ClientTLSInfo.KeyFile = etcdServerConfig.ServingInfo.ServerCert.KeyFile
	cfg.ClientTLSInfo.ClientCertAuth = len(cfg.ClientTLSInfo.CAFile) > 0
	u, err := types.NewURLs(addressToURLs(etcdServerConfig.ServingInfo.BindAddress, clientTLS))
	if err != nil {
		glog.Fatalf("Unable to build etcd peer URLs: %v", err)
	}
	cfg.LCUrls = []url.URL(u)

	peerTLS := true
	cfg.PeerTLSInfo.CAFile = etcdServerConfig.PeerServingInfo.ClientCA
	cfg.PeerTLSInfo.CertFile = etcdServerConfig.PeerServingInfo.ServerCert.CertFile
	cfg.PeerTLSInfo.KeyFile = etcdServerConfig.PeerServingInfo.ServerCert.KeyFile
	cfg.PeerTLSInfo.ClientCertAuth = len(cfg.PeerTLSInfo.CAFile) > 0
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

	ready := make(chan struct{})
	go func() {
		defer e.Close()

		select {
		case <-e.Server.ReadyNotify():
			// embedded servers must negotiate to reach v3 mode, this ensures we loop until that happens
			glog.V(4).Infof("Waiting for etcd to reach cluster version 3.0.0")
			for min := semver.Must(semver.NewVersion("3.0.0")); e.Server.ClusterVersion() == nil || e.Server.ClusterVersion().LessThan(*min); {
				time.Sleep(25 * time.Millisecond)
			}
			close(ready)
			glog.Infof("Started etcd at %s", etcdServerConfig.Address)
		case <-time.After(60 * time.Second):
			glog.Warning("etcd took too long to start, stopped")
			e.Server.Stop() // trigger a shutdown
		}
		glog.Fatalf("etcd has returned an error: %v", <-e.Err())
	}()
	<-ready
}

// addressToURLs turns a host:port comma delimited list into an array valid
// URL strings with the appropriate prefix for the TLS mode.
func addressToURLs(addr string, isTLS bool) []string {
	addrs := strings.Split(addr, ",")
	for i := range addrs {
		if strings.HasPrefix(addrs[i], "unix://") || strings.HasPrefix(addrs[i], "unixs://") {
			continue
		}
		if isTLS {
			addrs[i] = "https://" + addrs[i]
		} else {
			addrs[i] = "http://" + addrs[i]
		}
	}
	return addrs
}
