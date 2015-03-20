package util

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path"
	"time"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/client"
	newproject "github.com/openshift/origin/pkg/cmd/experimental/project"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/start"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func init() {
	RequireEtcd()
}

// RequireServer verifies if the etcd, docker and the OpenShift server are
// available and you can successfully connected to them.
func RequireServer() {
	RequireEtcd()
	RequireDocker()
	if _, err := GetClusterAdminClient(KubeConfigPath()); err != nil {
		os.Exit(1)
	}
}

// GetBaseDir returns the base directory used for test.
func GetBaseDir() string {
	return cmdutil.Env("BASETMPDIR", path.Join(os.TempDir(), "openshift-"+Namespace()))
}

func setupStartOptions() (*start.MasterArgs, *start.NodeArgs, *start.ListenArg, *start.ImageFormatArgs, *start.KubeConnectionArgs, *start.CertArgs) {
	masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs, certArgs := start.GetAllInOneArgs()

	basedir := GetBaseDir()

	nodeArgs.VolumeDir = path.Join(basedir, "volume")
	masterArgs.EtcdDir = path.Join(basedir, "etcd")
	masterArgs.PolicyArgs.PolicyFile = path.Join(basedir, "policy", "policy.json")
	certArgs.CertDir = path.Join(basedir, "cert")

	// don't wait for nodes to come up

	masterAddr := httptest.NewUnstartedServer(nil).Listener.Addr().String()
	if len(os.Getenv("OS_MASTER_ADDR")) > 0 {
		masterAddr = os.Getenv("OS_MASTER_ADDR")
	}
	fmt.Printf("masterAddr: %#v\n", masterAddr)

	masterArgs.MasterAddr.Set(masterAddr)
	listenArg.ListenAddr.Set(masterAddr)
	masterArgs.EtcdAddr.Set(GetEtcdURL())

	assetAddr := httptest.NewUnstartedServer(nil).Listener.Addr().String()
	if len(os.Getenv("OS_ASSETS_ADDR")) > 0 {
		assetAddr = os.Getenv("OS_ASSETS_ADDR")
	}

	fmt.Printf("assetAddr: %#v\n", assetAddr)
	masterArgs.AssetBindAddr.Set(assetAddr)
	masterArgs.AssetPublicAddr.Set(assetAddr)

	dnsAddr := httptest.NewUnstartedServer(nil).Listener.Addr().String()
	if len(os.Getenv("OS_DNS_ADDR")) > 0 {
		dnsAddr = os.Getenv("OS_DNS_ADDR")
	}
	fmt.Printf("dnsAddr: %#v\n", dnsAddr)
	masterArgs.DNSBindAddr.Set(dnsAddr)

	return masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs, certArgs
}

func StartTestAllInOne() (*configapi.MasterConfig, string, error) {
	DeleteAllEtcdKeys()

	masterArgs, nodeArgs, _, _, _, _ := setupStartOptions()
	masterArgs.NodeList = nil

	startOptions := start.AllInOneOptions{}
	startOptions.MasterArgs, startOptions.NodeArgs = masterArgs, nodeArgs
	startOptions.NodeArgs.AllowDisabledDocker = true
	startOptions.Complete()

	errCh := make(chan error)
	go func() {
		errCh <- startOptions.StartAllInOne()
		close(errCh)
	}()

	adminKubeConfigFile := getAdminKubeConfigFile(*masterArgs.CertArgs)

	openshiftMasterConfig, err := startOptions.MasterArgs.BuildSerializeableMasterConfig()
	if err != nil {
		return nil, "", err
	}

	// wait for the server to come up: 35 seconds
	if err := cmdutil.WaitForSuccessfulDial(true, "tcp", masterArgs.MasterAddr.URL.Host, 100*time.Millisecond, 1*time.Second, 35); err != nil {
		select {
		case err := <-errCh:
			if err != nil {
				return nil, "", err
			}
		default:
		}
		return nil, "", err
	}

	// try to get a client
	for {
		select {
		case err := <-errCh:
			if err != nil {
				return nil, "", err
			}
		default:
		}
		// confirm that we can actually query from the api server

		if client, err := GetClusterAdminClient(adminKubeConfigFile); err == nil {
			if _, err := client.Policies(bootstrappolicy.DefaultMasterAuthorizationNamespace).List(labels.Everything(), fields.Everything()); err == nil {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return openshiftMasterConfig, adminKubeConfigFile, nil
}

// TODO Unify with StartAllInOne.

// StartTestMaster starts up a test master and returns back the startOptions so you can get clients and certs
func StartTestMaster() (*configapi.MasterConfig, string, error) {
	DeleteAllEtcdKeys()

	masterArgs, _, _, _, _, _ := setupStartOptions()

	startOptions := start.MasterOptions{}
	startOptions.MasterArgs = masterArgs
	startOptions.Complete()

	var startError error
	go func() {
		err := startOptions.StartMaster()
		if err != nil {
			startError = err
			fmt.Printf("ERROR STARTING SERVER! %v", err)
		}
	}()

	adminKubeConfigFile := getAdminKubeConfigFile(*masterArgs.CertArgs)

	openshiftMasterConfig, err := startOptions.MasterArgs.BuildSerializeableMasterConfig()
	if err != nil {
		return nil, "", err
	}

	// wait for the server to come up: 35 seconds
	if err := cmdutil.WaitForSuccessfulDial(true, "tcp", masterArgs.MasterAddr.URL.Host, 100*time.Millisecond, 1*time.Second, 35); err != nil {
		return nil, "", err
	}

	stopChannel := make(chan struct{})
	util.Until(
		func() {
			if startError != nil {
				close(stopChannel)
				return
			}

			// confirm that we can actually query from the api server
			client, err := GetClusterAdminClient(adminKubeConfigFile)
			if err != nil {
				return
			}
			if _, err := client.Policies(bootstrappolicy.DefaultMasterAuthorizationNamespace).List(labels.Everything(), fields.Everything()); err == nil {
				close(stopChannel)
			}
		}, 100*time.Millisecond, stopChannel)

	return openshiftMasterConfig, adminKubeConfigFile, startError
}

// CreateNewProject creates a new project using the clusterAdminClient, then gets a token for the adminUser and returns
// back a client for the admin user
func CreateNewProject(clusterAdminClient *client.Client, clientConfig kclient.Config, projectName, adminUser string) (*client.Client, error) {
	qualifiedUser := "anypassword:" + adminUser
	newProjectOptions := &newproject.NewProjectOptions{
		Client:                clusterAdminClient,
		ProjectName:           projectName,
		AdminRole:             bootstrappolicy.AdminRoleName,
		MasterPolicyNamespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		AdminUser:             qualifiedUser,
	}

	if err := newProjectOptions.Run(); err != nil {
		return nil, err
	}

	token, err := tokencmd.RequestToken(&clientConfig, nil, adminUser, "password")
	if err != nil {
		return nil, err
	}

	adminClientConfig := clientConfig
	adminClientConfig.BearerToken = token
	adminClientConfig.Username = ""
	adminClientConfig.Password = ""
	adminClientConfig.TLSClientConfig.CertFile = ""
	adminClientConfig.TLSClientConfig.KeyFile = ""
	adminClientConfig.TLSClientConfig.CertData = nil
	adminClientConfig.TLSClientConfig.KeyData = nil

	adminClient, err := client.New(&adminClientConfig)
	if err != nil {
		return nil, err
	}

	return adminClient, nil
}
