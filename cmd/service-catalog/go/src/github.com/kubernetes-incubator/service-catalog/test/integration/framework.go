/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"testing"
	"time"

	restfullog "github.com/emicklei/go-restful/log"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	restclient "k8s.io/client-go/rest"

	genericserveroptions "k8s.io/apiserver/pkg/server/options"

	"github.com/kubernetes-incubator/service-catalog/cmd/apiserver/app/server"
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/settings/install"
	servicecatalogclient "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	serverstorage "github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	// silence the go-restful webservices swagger logger
	restfullog.SetLogger(log.New(ioutil.Discard, "[restful]", log.LstdFlags|log.Lshortfile))
}

type TestServerConfig struct {
	etcdServerList []string
	storageType    serverstorage.StorageType
	emptyObjFunc   func() runtime.Object
}

// NewTestServerConfig is a default constructor for the standard test-apiserver setup
func NewTestServerConfig() *TestServerConfig {
	return &TestServerConfig{
		etcdServerList: []string{"http://localhost:2379"},
	}
}

func withConfigGetFreshApiserverAndClient(
	t *testing.T,
	serverConfig *TestServerConfig,
) (servicecatalogclient.Interface,
	*restclient.Config,
	func(),
) {
	securePort := rand.Intn(31743) + 1024
	secureAddr := fmt.Sprintf("https://localhost:%d", securePort)
	stopCh := make(chan struct{})
	serverFailed := make(chan struct{})
	shutdownServer := func() {
		t.Logf("Shutting down server on port: %d", securePort)
		close(stopCh)
	}

	t.Logf("Starting server on port: %d", securePort)
	certDir, _ := ioutil.TempDir("", "service-catalog-integration")
	secureServingOptions := genericserveroptions.NewSecureServingOptions()
	// start the server in the background
	go func() {
		var etcdOptions *server.EtcdOptions
		if serverstorage.StorageTypeEtcd == serverConfig.storageType {
			etcdOptions = server.NewEtcdOptions()
			etcdOptions.StorageConfig.ServerList = serverConfig.etcdServerList
			etcdOptions.EtcdOptions.StorageConfig.Prefix = fmt.Sprintf("%s-%08X", server.DefaultEtcdPathPrefix, rand.Int31())
		} else {
			t.Fatal("no storage type specified")
		}

		options := &server.ServiceCatalogServerOptions{
			StorageTypeString:       serverConfig.storageType.String(),
			GenericServerRunOptions: genericserveroptions.NewServerRunOptions(),
			AdmissionOptions:        genericserveroptions.NewAdmissionOptions(),
			SecureServingOptions:    secureServingOptions,
			EtcdOptions:             etcdOptions,
			AuthenticationOptions:   genericserveroptions.NewDelegatingAuthenticationOptions(),
			AuthorizationOptions:    genericserveroptions.NewDelegatingAuthorizationOptions(),
			AuditOptions:            genericserveroptions.NewAuditOptions(),
			DisableAuth:             true,
			StandaloneMode:          true, // this must be true because we have no kube server for integration.
			ServeOpenAPISpec:        true,
		}
		options.SecureServingOptions.BindPort = securePort
		options.SecureServingOptions.ServerCert.CertDirectory = certDir

		if err := server.RunServer(options, stopCh); err != nil {
			close(serverFailed)
			t.Fatalf("Error in bringing up the server: %v", err)
		}
	}()

	if err := waitForApiserverUp(secureAddr, serverFailed); err != nil {
		t.Fatalf("%v", err)
	}

	config := &restclient.Config{}
	config.Host = secureAddr
	config.Insecure = true
	config.CertFile = secureServingOptions.ServerCert.CertKey.CertFile
	config.KeyFile = secureServingOptions.ServerCert.CertKey.KeyFile
	clientset, err := servicecatalogclient.NewForConfig(config)
	if nil != err {
		t.Fatal("can't make the client from the config", err)
	}
	return clientset, config, shutdownServer
}

func getFreshApiserverAndClient(
	t *testing.T,
	storageTypeStr string,
	newEmptyObj func() runtime.Object,
) (servicecatalogclient.Interface, *restclient.Config, func()) {
	var serverStorageType serverstorage.StorageType
	serverStorageType, err := serverstorage.StorageTypeFromString(storageTypeStr)
	if nil != err {
		t.Fatal("non supported storage type")
	}

	serverConfig := &TestServerConfig{
		etcdServerList: []string{"http://localhost:2379"},
		storageType:    serverStorageType,
		emptyObjFunc:   newEmptyObj,
	}
	client, clientConfig, shutdownFunc := withConfigGetFreshApiserverAndClient(t, serverConfig)
	return client, clientConfig, shutdownFunc
}

func waitForApiserverUp(serverURL string, stopCh <-chan struct{}) error {
	interval := 1 * time.Second
	timeout := 30 * time.Second
	startWaiting := time.Now()
	tries := 0
	return wait.PollImmediate(interval, timeout,
		func() (bool, error) {
			select {
			// we've been told to stop, so no reason to keep going
			case <-stopCh:
				return true, fmt.Errorf("apiserver failed")
			default:
				glog.Infof("Waiting for : %#v", serverURL)
				tr := &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}
				c := &http.Client{Transport: tr}
				_, err := c.Get(serverURL)
				if err == nil {
					glog.Infof("Found server after %v tries and duration %v",
						tries, time.Since(startWaiting))
					return true, nil
				}
				tries++
				return false, nil
			}
		},
	)
}
