package etcd

import (
	"fmt"
	"net"
	"net/http"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"k8s.io/kubernetes/pkg/client/restclient"
	etcdutil "k8s.io/kubernetes/pkg/storage/etcd/util"
	knet "k8s.io/kubernetes/pkg/util/net"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// GetAndTestEtcdClient creates an etcd client based on the provided config. It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func GetAndTestEtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (etcdclient.Client, error) {
	etcdClient, err := MakeEtcdClient(etcdClientInfo)
	if err != nil {
		return nil, err
	}
	if err := TestEtcdClient(etcdClient); err != nil {
		return nil, err
	}
	return etcdClient, nil
}

// MakeEtcdClient creates an etcd client based on the provided config.
func MakeEtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (etcdclient.Client, error) {
	tlsConfig, err := restclient.TLSConfigFor(&restclient.Config{
		TLSClientConfig: restclient.TLSClientConfig{
			CertFile: etcdClientInfo.ClientCert.CertFile,
			KeyFile:  etcdClientInfo.ClientCert.KeyFile,
			CAFile:   etcdClientInfo.CA,
		},
	})
	if err != nil {
		return nil, err
	}

	transport := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
		Dial: (&net.Dialer{
			// default from http.DefaultTransport
			Timeout: 30 * time.Second,
			// Lower the keep alive for connections.
			KeepAlive: 1 * time.Second,
		}).Dial,
		// Because watches are very bursty, defends against long delays in watch reconnections.
		MaxIdleConnsPerHost: 500,
	})

	cfg := etcdclient.Config{
		Endpoints: etcdClientInfo.URLs,
		// TODO: Determine if transport needs optimization
		Transport: transport,
	}
	return etcdclient.New(cfg)
}

// TestEtcdClient verifies a client is functional.  It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func TestEtcdClient(etcdClient etcdclient.Client) error {
	for i := 0; ; i++ {
		_, err := etcdclient.NewKeysAPI(etcdClient).Get(context.Background(), "/", nil)
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
