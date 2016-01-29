package etcd

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	newetcdclient "github.com/coreos/etcd/client"
	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	client "k8s.io/kubernetes/pkg/client/unversioned"
	etcdutil "k8s.io/kubernetes/pkg/storage/etcd/util"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// GetAndTestEtcdClient creates an etcd client based on the provided config. It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func GetAndTestEtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (*etcdclient.Client, error) {
	etcdClient, err := EtcdClient(etcdClientInfo)
	if err != nil {
		return nil, err
	}
	if err := TestEtcdClient(etcdClient); err != nil {
		return nil, err
	}
	return etcdClient, nil
}

// EtcdClient creates an etcd client based on the provided config.
func EtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (*etcdclient.Client, error) {
	tlsConfig, err := client.TLSConfigFor(&client.Config{
		TLSClientConfig: client.TLSClientConfig{
			CertFile: etcdClientInfo.ClientCert.CertFile,
			KeyFile:  etcdClientInfo.ClientCert.KeyFile,
			CAFile:   etcdClientInfo.CA,
		},
	})
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		Dial: (&net.Dialer{
			// default from http.DefaultTransport
			Timeout: 30 * time.Second,
			// Lower the keep alive for connections.
			KeepAlive: 1 * time.Second,
		}).Dial,
		// Because watches are very bursty, defends against long delays in watch reconnections.
		MaxIdleConnsPerHost: 500,
		// defaults from http.DefaultTransport
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	etcdClient := etcdclient.NewClient(etcdClientInfo.URLs)
	etcdClient.SetTransport(transport)
	etcdClient.CheckRetry = NeverRetryOnFailure
	return etcdClient, nil
}

// MakeNewEtcdClient creates an etcd client based on the provided config.
func MakeNewEtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (newetcdclient.Client, error) {
	tlsConfig, err := client.TLSConfigFor(&client.Config{
		TLSClientConfig: client.TLSClientConfig{
			CertFile: etcdClientInfo.ClientCert.CertFile,
			KeyFile:  etcdClientInfo.ClientCert.KeyFile,
			CAFile:   etcdClientInfo.CA,
		},
	})
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		Dial: (&net.Dialer{
			// default from http.DefaultTransport
			Timeout: 30 * time.Second,
			// Lower the keep alive for connections.
			KeepAlive: 1 * time.Second,
		}).Dial,
		// Because watches are very bursty, defends against long delays in watch reconnections.
		MaxIdleConnsPerHost: 500,
		// defaults from http.DefaultTransport
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	cfg := newetcdclient.Config{
		Endpoints: etcdClientInfo.URLs,
		// TODO: Determine if transport needs optimization
		Transport: transport,
	}
	return newetcdclient.New(cfg)
}

// TestEtcdClient verifies a client is functional.  It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func TestEtcdClient(etcdClient *etcdclient.Client) error {
	for i := 0; ; i++ {
		_, err := etcdClient.Get("/", false, false)
		if err == nil || etcdutil.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			return fmt.Errorf("could not reach etcd: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// NeverRetryOnFailure is a retry function for the etcdClient.  If there's only one machine, master election doesn't make much sense,
// so we don't bother to retry, we simply dump the failure and return the error directly.
func NeverRetryOnFailure(cluster *etcdclient.Cluster, numReqs int, lastResp http.Response, err error) error {
	if len(cluster.Machines) > 1 {
		return etcdclient.DefaultCheckRetry(cluster, numReqs, lastResp, err)
	}

	content, err := httputil.DumpResponse(&lastResp, true)
	if err != nil {
		glog.Errorf("failure dumping response: %v", err)
	} else {
		glog.Errorf("etcd failure response: %s", string(content))
	}
	return err
}
