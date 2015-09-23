package server

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	newproject "github.com/openshift/origin/pkg/cmd/admin/project"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/start"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/test/util"
)

// ServiceAccountWaitTimeout is used to determine how long to wait for the service account
// controllers to start up, and populate the service accounts in the test namespace
const ServiceAccountWaitTimeout = 30 * time.Second

// RequireServer verifies if the etcd, docker and the OpenShift server are
// available and you can successfully connected to them.
func RequireServer() {
	util.RequireEtcd()
	util.RequireDocker()
	if _, err := util.GetClusterAdminClient(util.KubeConfigPath()); err != nil {
		os.Exit(1)
	}
}

// FindAvailableBindAddress returns a bind address on 127.0.0.1 with a free port in the low-high range.
// If lowPort is 0, an ephemeral port is allocated.
func FindAvailableBindAddress(lowPort, highPort int) (string, error) {
	if highPort < lowPort {
		return "", errors.New("lowPort must be <= highPort")
	}
	for port := lowPort; port <= highPort; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			if port == 0 {
				// Only get one shot to get an ephemeral port
				return "", err
			}
			continue
		}
		defer l.Close()
		return l.Addr().String(), nil
	}

	return "", fmt.Errorf("Could not find available port in the range %d-%d", lowPort, highPort)
}

func setupStartOptions() (*start.MasterArgs, *start.NodeArgs, *start.ListenArg, *start.ImageFormatArgs, *start.KubeConnectionArgs) {
	masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs := start.GetAllInOneArgs()

	basedir := util.GetBaseDir()

	nodeArgs.VolumeDir = path.Join(basedir, "volume")
	masterArgs.EtcdDir = path.Join(basedir, "etcd")
	masterArgs.ConfigDir.Default(path.Join(basedir, "openshift.local.config", "master"))
	nodeArgs.ConfigDir.Default(path.Join(basedir, "openshift.local.config", nodeArgs.NodeName))
	nodeArgs.MasterCertDir = masterArgs.ConfigDir.Value()

	// don't wait for nodes to come up
	masterAddr := os.Getenv("OS_MASTER_ADDR")
	if len(masterAddr) == 0 {
		if addr, err := FindAvailableBindAddress(8443, 8999); err != nil {
			glog.Fatalf("Couldn't find free address for master: %v", err)
		} else {
			masterAddr = addr
		}
	}
	fmt.Printf("masterAddr: %#v\n", masterAddr)

	masterArgs.MasterAddr.Set(masterAddr)
	listenArg.ListenAddr.Set(masterAddr)
	masterArgs.EtcdAddr.Set(util.GetEtcdURL())

	dnsAddr := os.Getenv("OS_DNS_ADDR")
	if len(dnsAddr) == 0 {
		if addr, err := FindAvailableBindAddress(8053, 8100); err != nil {
			glog.Fatalf("Couldn't find free address for DNS: %v", err)
		} else {
			dnsAddr = addr
		}
	}
	fmt.Printf("dnsAddr: %#v\n", dnsAddr)
	masterArgs.DNSBindAddr.Set(dnsAddr)

	return masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs
}

func DefaultMasterOptions() (*configapi.MasterConfig, error) {
	startOptions := start.MasterOptions{}
	startOptions.MasterArgs, _, _, _, _ = setupStartOptions()
	startOptions.Complete()
	startOptions.MasterArgs.ConfigDir.Default(path.Join(util.GetBaseDir(), "openshift.local.config", "master"))

	if err := CreateMasterCerts(startOptions.MasterArgs); err != nil {
		return nil, err
	}
	if err := CreateBootstrapPolicy(startOptions.MasterArgs); err != nil {
		return nil, err
	}

	masterConfig, err := startOptions.MasterArgs.BuildSerializeableMasterConfig()
	if err != nil {
		return nil, err
	}

	// force strict handling of service account secret references by default, so that all our examples and controllers will handle it.
	masterConfig.ServiceAccountConfig.LimitSecretReferences = true
	return masterConfig, nil
}

func CreateBootstrapPolicy(masterArgs *start.MasterArgs) error {
	createBootstrapPolicy := &admin.CreateBootstrapPolicyFileOptions{
		File: path.Join(masterArgs.ConfigDir.Value(), "policy.json"),
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

		Output: os.Stderr,
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
	getSignerOptions := &admin.SignerCertOptions{
		CertFile:   admin.DefaultCertFilename(nodeArgs.MasterCertDir, "ca"),
		KeyFile:    admin.DefaultKeyFilename(nodeArgs.MasterCertDir, "ca"),
		SerialFile: admin.DefaultSerialFilename(nodeArgs.MasterCertDir, "ca"),
	}

	createNodeConfig := admin.NewDefaultCreateNodeConfigOptions()
	createNodeConfig.Output = os.Stdout
	createNodeConfig.SignerCertOptions = getSignerOptions
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
	startOptions := start.AllInOneOptions{MasterOptions: &start.MasterOptions{}, NodeArgs: &start.NodeArgs{}}
	startOptions.MasterOptions.MasterArgs, startOptions.NodeArgs, _, _, _ = setupStartOptions()
	startOptions.MasterOptions.MasterArgs.NodeList = nil
	startOptions.NodeArgs.AllowDisabledDocker = true
	startOptions.ServiceNetworkCIDR = start.NewDefaultNetworkArgs().ServiceNetworkCIDR
	startOptions.Complete()
	startOptions.MasterOptions.MasterArgs.ConfigDir.Default(path.Join(util.GetBaseDir(), "openshift.local.config", "master"))
	startOptions.NodeArgs.ConfigDir.Default(path.Join(util.GetBaseDir(), "openshift.local.config", admin.DefaultNodeDir(startOptions.NodeArgs.NodeName)))
	startOptions.NodeArgs.MasterCertDir = startOptions.MasterOptions.MasterArgs.ConfigDir.Value()

	if err := CreateMasterCerts(startOptions.MasterOptions.MasterArgs); err != nil {
		return nil, nil, err
	}
	if err := CreateBootstrapPolicy(startOptions.MasterOptions.MasterArgs); err != nil {
		return nil, nil, err
	}

	if err := CreateNodeCerts(startOptions.NodeArgs); err != nil {
		return nil, nil, err
	}

	masterOptions, err := startOptions.MasterOptions.MasterArgs.BuildSerializeableMasterConfig()
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

func StartTestAllInOne() (*configapi.MasterConfig, *configapi.NodeConfig, string, error) {
	master, node, err := DefaultAllInOneOptions()
	if err != nil {
		return nil, nil, "", err
	}

	adminKubeConfigFile, err := StartConfiguredAllInOne(master, node)
	return master, node, adminKubeConfigFile, err
}

type TestOptions struct {
	DeleteAllEtcdKeys bool
}

func DefaultTestOptions() TestOptions {
	return TestOptions{true}
}

func StartConfiguredMaster(masterConfig *configapi.MasterConfig) (string, error) {
	return StartConfiguredMasterWithOptions(masterConfig, DefaultTestOptions())
}

func StartConfiguredMasterWithOptions(masterConfig *configapi.MasterConfig, testOptions TestOptions) (string, error) {
	if testOptions.DeleteAllEtcdKeys {
		util.DeleteAllEtcdKeys()
	}

	if err := start.NewMaster(masterConfig, true, true).Start(); err != nil {
		return "", err
	}
	adminKubeConfigFile := util.KubeConfigPath()
	clientConfig, err := util.GetClusterAdminClientConfig(adminKubeConfigFile)
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
		if client, err := util.GetClusterAdminClient(adminKubeConfigFile); err == nil {
			if _, err := client.ClusterPolicies().List(labels.Everything(), fields.Everything()); err == nil {
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

// serviceAccountSecretsExist checks whether the given service account has at least a token and a dockercfg
// secret associated with it.
func serviceAccountSecretsExist(client *kclient.Client, namespace string, sa *kapi.ServiceAccount) bool {
	foundTokenSecret := false
	foundDockercfgSecret := false
	for _, secret := range sa.Secrets {
		ns := namespace
		if len(secret.Namespace) > 0 {
			ns = secret.Namespace
		}
		secret, err := client.Secrets(ns).Get(secret.Name)
		if err == nil {
			switch secret.Type {
			case kapi.SecretTypeServiceAccountToken:
				foundTokenSecret = true
			case kapi.SecretTypeDockercfg:
				foundDockercfgSecret = true
			}
		}
	}
	return foundTokenSecret && foundDockercfgSecret
}

// WaitForServiceAccounts ensures the service accounts needed by build pods exist in the namespace
// The extra controllers tend to starve the service account controller
func WaitForServiceAccounts(client *kclient.Client, namespace string, accounts []string) error {
	serviceAccounts := client.ServiceAccounts(namespace)
	return wait.Poll(time.Second, ServiceAccountWaitTimeout, func() (bool, error) {
		for _, account := range accounts {
			if sa, err := serviceAccounts.Get(account); err != nil {
				if !serviceAccountSecretsExist(client, namespace, sa) {
					continue
				}
				return false, nil
			}
		}
		return true, nil
	})
}

// CreateNewProject creates a new project using the clusterAdminClient, then gets a token for the adminUser and returns
// back a client for the admin user
func CreateNewProject(clusterAdminClient *client.Client, clientConfig kclient.Config, projectName, adminUser string) (*client.Client, error) {
	newProjectOptions := &newproject.NewProjectOptions{
		Client:      clusterAdminClient,
		ProjectName: projectName,
		AdminRole:   bootstrappolicy.AdminRoleName,
		AdminUser:   adminUser,
	}

	if err := newProjectOptions.Run(false); err != nil {
		return nil, err
	}

	client, _, _, err := util.GetClientForUser(clientConfig, adminUser)
	return client, err
}
