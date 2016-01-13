package etcd

import (
	"fmt"
	"net"
	"net/http"
	"time"

	etcdclient "github.com/coreos/go-etcd/etcd"

	client "k8s.io/kubernetes/pkg/client/unversioned"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"

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
	return etcdClient, nil
}

// TestEtcdClient verifies a client is functional.  It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func TestEtcdClient(etcdClient *etcdclient.Client) error {
	for i := 0; ; i++ {
		_, err := etcdClient.Get("/", false, false)
		if err == nil || etcdstorage.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			return fmt.Errorf("could not reach etcd: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}
