// This is a somewhat faithful reproduction of etcdmain/etcd.go
package etcd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/pkg/netutil"
	"github.com/coreos/etcd/pkg/osutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/rafthttp"
)

type config struct {
	// member
	dir            string
	lpurls, lcurls []url.URL
	maxSnapFiles   uint
	maxWalFiles    uint
	name           string
	snapCount      uint64
	// TODO: decouple tickMs and heartbeat tick (current heartbeat tick = 1).
	// make ticks a cluster wide configuration.
	TickMs     uint
	ElectionMs uint

	// clustering
	apurls, acurls      []url.URL
	initialCluster      string
	initialClusterToken string

	// security
	clientTLSInfo, peerTLSInfo transport.TLSInfo
}

const (
	// the owner can make/remove files inside the directory
	defaultName = "openshift.local"
)

// startEtcd launches the etcd server and HTTP handlers for client/server communication.
func startEtcd(cfg *config) (<-chan struct{}, error) {
	initialPeers, token, err := setupCluster(cfg)
	if err != nil {
		return nil, fmt.Errorf("error setting up initial cluster: %v", err)
	}

	pt, err := transport.NewTimeoutTransport(cfg.peerTLSInfo, rafthttp.DialTimeout, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
	if err != nil {
		return nil, err
	}

	if !cfg.peerTLSInfo.Empty() {
		glog.V(2).Infof("etcd: peerTLS: %s", cfg.peerTLSInfo)
	}
	plns := make([]net.Listener, 0)
	for _, u := range cfg.lpurls {
		var l net.Listener
		l, err = transport.NewTimeoutListener(u.Host, u.Scheme, cfg.peerTLSInfo, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
		if err != nil {
			return nil, err
		}

		urlStr := u.String()
		glog.V(2).Info("etcd: listening for peers on ", urlStr)
		defer func() {
			if err != nil {
				l.Close()
				glog.V(2).Info("etcd: stopping listening for peers on ", urlStr)
			}
		}()
		plns = append(plns, l)
	}

	if !cfg.clientTLSInfo.Empty() {
		glog.V(2).Infof("etcd: clientTLS: %s", cfg.clientTLSInfo)
	}
	clns := make([]net.Listener, 0)
	for _, u := range cfg.lcurls {
		var l net.Listener
		l, err = transport.NewKeepAliveListener(u.Host, u.Scheme, cfg.clientTLSInfo)
		if err != nil {
			return nil, err
		}

		urlStr := u.String()
		glog.V(2).Info("etcd: listening for client requests on ", urlStr)
		defer func() {
			if err != nil {
				l.Close()
				glog.V(2).Info("etcd: stopping listening for client requests on ", urlStr)
			}
		}()
		clns = append(clns, l)
	}

	srvcfg := &etcdserver.ServerConfig{
		Name:                cfg.name,
		ClientURLs:          cfg.acurls,
		PeerURLs:            cfg.apurls,
		DataDir:             cfg.dir,
		SnapCount:           cfg.snapCount,
		MaxSnapFiles:        cfg.maxSnapFiles,
		InitialPeerURLsMap:  initialPeers,
		InitialClusterToken: token,
		MaxWALFiles:         cfg.maxWalFiles,
		NewCluster:          true,
		ForceNewCluster:     false,
		Transport:           pt,
		TickMs:              cfg.TickMs,
		ElectionTicks:       cfg.electionTicks(),
	}
	var s *etcdserver.EtcdServer
	s, err = etcdserver.NewServer(srvcfg)
	if err != nil {
		return nil, err
	}
	osutil.HandleInterrupts()
	s.Start()
	osutil.RegisterInterruptHandler(s.Stop)

	ch := etcdhttp.NewClientHandler(s)
	ph := etcdhttp.NewPeerHandler(s.Cluster(), s.RaftHandler())
	// Start the peer server in a goroutine
	for _, l := range plns {
		go func(l net.Listener) {
			glog.Fatal(serveHTTP(l, ph, 5*time.Minute))
		}(l)
	}
	// Start a client server goroutine for each listen address
	for _, l := range clns {
		go func(l net.Listener) {
			// read timeout does not work with http close notify
			// TODO: https://github.com/golang/go/issues/9524
			glog.Fatal(serveHTTP(l, ch, 0))
		}(l)
	}
	return s.StopNotify(), nil
}

// setupCluster sets up an initial cluster definition for bootstrap or discovery.
func setupCluster(cfg *config) (types.URLsMap, string, error) {
	// We're statically configured, and cluster has appropriately been set.
	m, err := types.NewURLsMap(cfg.initialCluster)
	return m, cfg.initialClusterToken, err
}

func genClusterString(name string, urls types.URLs) string {
	addrs := make([]string, 0)
	for _, u := range urls {
		addrs = append(addrs, fmt.Sprintf("%v=%v", name, u.String()))
	}
	return strings.Join(addrs, ",")
}

func initialClusterFromName(name string) string {
	n := name
	if name == "" {
		n = defaultName
	}
	return fmt.Sprintf("%s=http://localhost:7001", n)
}

func urlsFromStrings(input string, tlsInfo transport.TLSInfo) ([]url.URL, error) {
	urls := []url.URL{}
	for _, addr := range strings.Split(input, ",") {
		addrURL := url.URL{Scheme: "http", Host: addr}
		if !tlsInfo.Empty() {
			addrURL.Scheme = "https"
		}
		urls = append(urls, addrURL)
	}
	return urls, nil
}

// serveHTTP accepts incoming HTTP connections on the listener l,
// creating a new service goroutine for each. The service goroutines
// read requests and then call handler to reply to them.
func serveHTTP(l net.Listener, handler http.Handler, readTimeout time.Duration) error {
	logger := log.New(ioutil.Discard, "etcdhttp", 0)
	// TODO: add debug flag; enable logging when debug flag is set
	srv := &http.Server{
		Handler:     handler,
		ReadTimeout: readTimeout,
		ErrorLog:    logger, // do not log user error
	}
	return srv.Serve(l)
}

func (cfg *config) resolveUrls() error {
	out, err := netutil.ResolveTCPAddrs([][]url.URL{cfg.lpurls, cfg.apurls, cfg.lcurls, cfg.acurls})
	if err != nil {
		return err
	}
	cfg.lpurls, cfg.apurls, cfg.lcurls, cfg.acurls = out[0], out[1], out[2], out[3]
	return nil
}

func (cfg config) electionTicks() int { return int(cfg.ElectionMs / cfg.TickMs) }
