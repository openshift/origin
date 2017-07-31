package etcd

import (
	"fmt"
	"net"
	"net/http"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"

	knet "k8s.io/apimachinery/pkg/util/net"
	etcdutil "k8s.io/apiserver/pkg/storage/etcd/util"
	restclient "k8s.io/client-go/rest"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// TestEtcdConnectionInfo creates an etcd client based on the provided config and attempts to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func TestEtcdConnectionInfo(etcdClientInfo configapi.EtcdConnectionInfo) error {
	tlsConfig, err := restclient.TLSConfigFor(&restclient.Config{
		TLSClientConfig: restclient.TLSClientConfig{
			CertFile: etcdClientInfo.ClientCert.CertFile,
			KeyFile:  etcdClientInfo.ClientCert.KeyFile,
			CAFile:   etcdClientInfo.CA,
		},
	})
	if err != nil {
		return err
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints: etcdClientInfo.URLs,
		TLS:       tlsConfig,
	})
	if err != nil {
		return err
	}
	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	return client.Sync(ctx)
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
			return fmt.Errorf("could not reach etcd(v2): %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// MakeEtcdClientV3Config creates client configuration based on the configapi.
func MakeEtcdClientV3Config(etcdClientInfo configapi.EtcdConnectionInfo) (*clientv3.Config, error) {
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

	return &clientv3.Config{
		Endpoints:   etcdClientInfo.URLs,
		DialTimeout: 30 * time.Second,
		TLS:         tlsConfig,
	}, nil
}

// MakeEtcdClientV3 creates an etcd v3 client based on the provided config.
func MakeEtcdClientV3(etcdClientInfo configapi.EtcdConnectionInfo) (*clientv3.Client, error) {
	cfg, err := MakeEtcdClientV3Config(etcdClientInfo)
	if err != nil {
		return nil, err
	}
	return clientv3.New(*cfg)
}

// TestEtcdClientV3 verifies a client is functional.  It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func TestEtcdClientV3(etcdClient *clientv3.Client) error {
	for i := 0; ; i++ {
		_, err := clientv3.NewKV(etcdClient).Get(context.Background(), "/", clientv3.WithLimit(1))
		if err == nil || etcdutil.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			return fmt.Errorf("could not reach etcd(v3): %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}
