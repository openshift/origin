// +build integration,!no-etcd

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	osclient "github.com/openshift/origin/pkg/client"
	start "github.com/openshift/origin/pkg/cmd/server"
)

func init() {
	requireEtcd()
}

func StartTestServer(args ...string) (*osclient.Client, error) {
	deleteAllEtcdKeys()

	startConfig := start.NewDefaultConfig()

	basedir := path.Join(os.TempDir(), "openshift-integration-tests")

	startConfig.VolumeDir = path.Join(basedir, "volume")
	startConfig.EtcdDir = path.Join(basedir, "etcd")
	startConfig.CertDir = path.Join(basedir, "cert")

	masterAddr := httptest.NewUnstartedServer(nil).Listener.Addr().String()
	fmt.Printf("masterAddr: %#v\n", masterAddr)

	startConfig.MasterAddr.Set(masterAddr)
	startConfig.BindAddr.Set(masterAddr)
	startConfig.EtcdAddr.Set(getEtcdURL())

	startConfig.Complete(args)

	go func() {
		err := startConfig.Start(args)
		if err != nil {
			fmt.Printf("ERROR STARTING SERVER! %v", err)
		}
	}()

	// if we request an OpenshiftClient before Start() has minted them, we can end up messing up the
	// increasing count of signed certs.  So instead we wait until we get back something other than
	// connection refused and then check to see if we can make a resource query
	// TODO clean this up actually use a proper cert since we know the location
	clientCertCreated := false
	stopChannel := make(chan struct{})
	util.Until(
		func() {
			if !clientCertCreated {
				url := path.Join(masterAddr, "osapi")
				url = "https://" + url

				_, err := http.Get(url)
				if (err != nil) && !strings.Contains(err.Error(), "connection refused") {
					clientCertCreated = true
				}

			} else {
				client, _, err := startConfig.GetOpenshiftClient()
				if err != nil {
					return
				}
				if _, err := client.Policies("master").List(labels.Everything(), labels.Everything()); err == nil {
					close(stopChannel)
				}
			}
		}, 100*time.Millisecond, stopChannel)

	client, _, err := startConfig.GetOpenshiftClient()
	if err != nil {
		return nil, err
	}

	return client, nil
}
func StartTestMaster() (*osclient.Client, error) {
	return StartTestServer("master")
}
func StartTestNode() (*osclient.Client, error) {
	return StartTestServer("node")
}
func StartTestAllInOne() (*osclient.Client, error) {
	return StartTestServer()
}
