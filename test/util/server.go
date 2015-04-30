package util

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"time"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	newproject "github.com/openshift/origin/pkg/cmd/experimental/project"
	"github.com/openshift/origin/pkg/cmd/server/admin"
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

func FindAvailableBindAddress() string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func setupStartOptions() (*start.MasterArgs, *start.NodeArgs, *start.ListenArg, *start.ImageFormatArgs, *start.KubeConnectionArgs) {
	masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs := start.GetAllInOneArgs()

	basedir := GetBaseDir()

	nodeArgs.VolumeDir = path.Join(basedir, "volume")
	masterArgs.EtcdDir = path.Join(basedir, "etcd")
	masterArgs.ConfigDir.Default(path.Join(basedir, "openshift.local.config", "master"))
	nodeArgs.ConfigDir.Default(path.Join(basedir, "openshift.local.config", nodeArgs.NodeName))
	nodeArgs.MasterCertDir = masterArgs.ConfigDir.Value()

	// don't wait for nodes to come up
	masterAddr := FindAvailableBindAddress()
	if len(os.Getenv("OS_MASTER_ADDR")) > 0 {
		masterAddr = os.Getenv("OS_MASTER_ADDR")
	}
	fmt.Printf("masterAddr: %#v\n", masterAddr)

	masterArgs.MasterAddr.Set(masterAddr)
	listenArg.ListenAddr.Set(masterAddr)
	masterArgs.EtcdAddr.Set(GetEtcdURL())

	dnsAddr := FindAvailableBindAddress()
	if len(os.Getenv("OS_DNS_ADDR")) > 0 {
		dnsAddr = os.Getenv("OS_DNS_ADDR")
	}
	fmt.Printf("dnsAddr: %#v\n", dnsAddr)
	masterArgs.DNSBindAddr.Set(dnsAddr)

	return masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs
}

func DefaultMasterOptions() (*configapi.MasterConfig, error) {
	startOptions := start.MasterOptions{}
	startOptions.MasterArgs, _, _, _, _ = setupStartOptions()
	startOptions.Complete()
	startOptions.MasterArgs.ConfigDir.Default(path.Join(GetBaseDir(), "openshift.local.config", "master"))

	if err := CreateMasterCerts(startOptions.MasterArgs); err != nil {
		return nil, err
	}
	if err := CreateBootstrapPolicy(startOptions.MasterArgs); err != nil {
		return nil, err
	}

	return startOptions.MasterArgs.BuildSerializeableMasterConfig()
}

func CreateBootstrapPolicy(masterArgs *start.MasterArgs) error {
	createBootstrapPolicy := &admin.CreateBootstrapPolicyFileOptions{
		File: path.Join(masterArgs.ConfigDir.Value(), "policy.json"),
		MasterAuthorizationNamespace:      "master",
		OpenShiftSharedResourcesNamespace: "openshift",
	}

	if err := createBootstrapPolicy.Validate(nil); err != nil {
		return err
	}
	if err := createBootstrapPolicy.CreateBootstrapPolicyFile(); err != nil {
		return err
	}

	return nil
}

func CreateMasterCerts(masterArgs *start.MasterArgs) error {
	hostnames, err := masterArgs.GetServerCertHostnames()
	if err != nil {
		return err
	}
	masterURL, err := masterArgs.GetMasterAddress()
	if err != nil {
		return err
	}
	publicMasterURL, err := masterArgs.GetMasterPublicAddress()
	if err != nil {
		return err
	}

	createMasterCerts := admin.CreateMasterCertsOptions{
		CertDir:    masterArgs.ConfigDir.Value(),
		SignerName: admin.DefaultSignerName(),
		Hostnames:  hostnames.List(),

		APIServerURL:       masterURL.String(),
		PublicAPIServerURL: publicMasterURL.String(),
	}

	if err := createMasterCerts.Validate(nil); err != nil {
		return err
	}
	if err := createMasterCerts.CreateMasterCerts(); err != nil {
		return err
	}

	return nil
}

func CreateNodeCerts(nodeArgs *start.NodeArgs) error {
	getSignerOptions := &admin.GetSignerCertOptions{
		CertFile:   admin.DefaultCertFilename(nodeArgs.MasterCertDir, "ca"),
		KeyFile:    admin.DefaultKeyFilename(nodeArgs.MasterCertDir, "ca"),
		SerialFile: admin.DefaultSerialFilename(nodeArgs.MasterCertDir, "ca"),
	}

	createNodeConfig := admin.NewDefaultCreateNodeConfigOptions()
	createNodeConfig.GetSignerCertOptions = getSignerOptions
	createNodeConfig.NodeConfigDir = nodeArgs.ConfigDir.Value()
	createNodeConfig.NodeName = nodeArgs.NodeName
	createNodeConfig.Hostnames = []string{nodeArgs.NodeName}
	createNodeConfig.ListenAddr = nodeArgs.ListenArg.ListenAddr
	createNodeConfig.APIServerCAFile = admin.DefaultCertFilename(nodeArgs.MasterCertDir, "ca")
	createNodeConfig.NodeClientCAFile = admin.DefaultCertFilename(nodeArgs.MasterCertDir, "ca")

	if err := createNodeConfig.Validate(nil); err != nil {
		return err
	}
	if err := createNodeConfig.CreateNodeFolder(); err != nil {
		return err
	}

	return nil
}

func DefaultAllInOneOptions() (*configapi.MasterConfig, *configapi.NodeConfig, error) {
	startOptions := start.AllInOneOptions{}
	startOptions.MasterArgs, startOptions.NodeArgs, _, _, _ = setupStartOptions()
	startOptions.MasterArgs.NodeList = nil
	startOptions.NodeArgs.AllowDisabledDocker = true
	startOptions.Complete()
	startOptions.MasterArgs.ConfigDir.Default(path.Join(GetBaseDir(), "openshift.local.config", "master"))
	startOptions.NodeArgs.ConfigDir.Default(path.Join(GetBaseDir(), "openshift.local.config", admin.DefaultNodeDir(startOptions.NodeArgs.NodeName)))
	startOptions.NodeArgs.MasterCertDir = startOptions.MasterArgs.ConfigDir.Value()

	if err := CreateMasterCerts(startOptions.MasterArgs); err != nil {
		return nil, nil, err
	}
	if err := CreateBootstrapPolicy(startOptions.MasterArgs); err != nil {
		return nil, nil, err
	}

	if err := CreateNodeCerts(startOptions.NodeArgs); err != nil {
		return nil, nil, err
	}

	masterOptions, err := startOptions.MasterArgs.BuildSerializeableMasterConfig()
	if err != nil {
		return nil, nil, err
	}

	nodeOptions, err := startOptions.NodeArgs.BuildSerializeableNodeConfig()
	if err != nil {
		return nil, nil, err
	}

	return masterOptions, nodeOptions, nil
}

func StartConfiguredAllInOne(masterConfig *configapi.MasterConfig, nodeConfig *configapi.NodeConfig) (string, error) {
	adminKubeConfigFile, err := StartConfiguredMaster(masterConfig)
	if err != nil {
		return "", err
	}

	if err := start.StartNode(*nodeConfig); err != nil {
		return "", err
	}

	return adminKubeConfigFile, nil
}

func StartTestAllInOne() (*configapi.MasterConfig, string, error) {
	master, node, err := DefaultAllInOneOptions()
	if err != nil {
		return nil, "", err
	}

	adminKubeConfigFile, err := StartConfiguredAllInOne(master, node)
	return master, adminKubeConfigFile, err
}

func StartConfiguredMaster(masterConfig *configapi.MasterConfig) (string, error) {
	DeleteAllEtcdKeys()

	if err := start.StartMaster(masterConfig); err != nil {
		return "", err
	}
	adminKubeConfigFile := KubeConfigPath()
	clientConfig, err := GetClusterAdminClientConfig(adminKubeConfigFile)
	if err != nil {
		return "", err
	}
	masterURL, err := url.Parse(clientConfig.Host)
	if err != nil {
		return "", err
	}

	// wait for the server to come up: 35 seconds
	if err := cmdutil.WaitForSuccessfulDial(true, "tcp", masterURL.Host, 100*time.Millisecond, 1*time.Second, 35); err != nil {
		return "", err
	}

	for {
		// confirm that we can actually query from the api server
		if client, err := GetClusterAdminClient(adminKubeConfigFile); err == nil {
			if _, err := client.Policies(bootstrappolicy.DefaultMasterAuthorizationNamespace).List(labels.Everything(), fields.Everything()); err == nil {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return adminKubeConfigFile, nil
}

// StartTestMaster starts up a test master and returns back the startOptions so you can get clients and certs
func StartTestMaster() (*configapi.MasterConfig, string, error) {
	master, err := DefaultMasterOptions()
	if err != nil {
		return nil, "", err
	}

	adminKubeConfigFile, err := StartConfiguredMaster(master)
	return master, adminKubeConfigFile, err
}

// CreateNewProject creates a new project using the clusterAdminClient, then gets a token for the adminUser and returns
// back a client for the admin user
func CreateNewProject(clusterAdminClient *client.Client, clientConfig kclient.Config, projectName, adminUser string) (*client.Client, error) {
	newProjectOptions := &newproject.NewProjectOptions{
		Client:                clusterAdminClient,
		ProjectName:           projectName,
		AdminRole:             bootstrappolicy.AdminRoleName,
		MasterPolicyNamespace: bootstrappolicy.DefaultMasterAuthorizationNamespace,
		AdminUser:             adminUser,
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
