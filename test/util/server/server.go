package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"

	etcdclientv3 "github.com/coreos/etcd/clientv3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/start"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	utilflags "github.com/openshift/origin/pkg/cmd/util/flags"
	newproject "github.com/openshift/origin/pkg/oc/admin/project"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	"github.com/openshift/origin/test/util"

	// install all APIs

	_ "github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

var (
	// startLock protects access to the start vars
	startLock sync.Mutex
	// startedMaster is true if the master has already been started in process
	startedMaster bool
	// startedNode is true if the node has already been started in process
	startedNode bool
)

// guardMaster prevents multiple master processes from being started at once
func guardMaster() {
	startLock.Lock()
	defer startLock.Unlock()
	if startedMaster {
		panic("the master has already been started once in this process - run only a single test, or use the sub-shell")
	}
	startedMaster = true
}

// guardMaster prevents multiple master processes from being started at once
func guardNode() {
	startLock.Lock()
	defer startLock.Unlock()
	if startedNode {
		panic("the node has already been started once in this process - run only a single test, or use the sub-shell")
	}
	startedNode = true
}

// ServiceAccountWaitTimeout is used to determine how long to wait for the service account
// controllers to start up, and populate the service accounts in the test namespace
const ServiceAccountWaitTimeout = 30 * time.Second

// PodCreationWaitTimeout is used to determine how long to wait after the service account token
// is available for the admission control cache to catch up and allow pod creation
const PodCreationWaitTimeout = 10 * time.Second

// FindAvailableBindAddress returns a bind address on 127.0.0.1 with a free port in the low-high range.
// If lowPort is 0, an ephemeral port is allocated.
func FindAvailableBindAddress(lowPort, highPort int) (string, error) {
	if highPort < lowPort {
		return "", errors.New("lowPort must be <= highPort")
	}
	for port := lowPort; port <= highPort; port++ {
		tryPort := port
		if tryPort == 0 {
			tryPort = int(rand.Int31n(int32(highPort-1024)) + 1024)
		} else {
			tryPort = int(rand.Int31n(int32(highPort-lowPort))) + lowPort
		}
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", tryPort))
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

func setupStartOptions(useDefaultPort bool) (*start.MasterArgs, *start.NodeArgs, *start.ListenArg, *start.ImageFormatArgs, *start.KubeConnectionArgs) {
	masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs := start.GetAllInOneArgs()

	basedir := util.GetBaseDir()

	nodeArgs.NodeName = "127.0.0.1"
	nodeArgs.VolumeDir = path.Join(basedir, "volume")

	// Allows to override the default etcd directory from the shell script.
	etcdDir := os.Getenv("TEST_ETCD_DIR")
	if len(etcdDir) == 0 {
		etcdDir = path.Join(basedir, "etcd")
	}

	masterArgs.EtcdDir = etcdDir
	masterArgs.ConfigDir.Default(path.Join(basedir, "openshift.local.config", "master"))
	nodeArgs.ConfigDir.Default(path.Join(basedir, "openshift.local.config", nodeArgs.NodeName))
	nodeArgs.MasterCertDir = masterArgs.ConfigDir.Value()

	// give the nodeArgs a separate listen argument
	nodeArgs.ListenArg = start.NewDefaultListenArg()

	if !useDefaultPort {
		// don't wait for nodes to come up
		masterAddr := os.Getenv("OS_MASTER_ADDR")
		if len(masterAddr) == 0 {
			if addr, err := FindAvailableBindAddress(10000, 29999); err != nil {
				glog.Fatalf("Couldn't find free address for master: %v", err)
			} else {
				masterAddr = addr
			}
		}
		masterArgs.MasterAddr.Set(masterAddr)
		listenArg.ListenAddr.Set(masterAddr)

		nodeAddr, err := FindAvailableBindAddress(10000, 29999)
		if err != nil {
			glog.Fatalf("couldn't find free port for node: %v", err)
		}
		nodeArgs.ListenArg.ListenAddr.Set(nodeAddr)
	}

	dnsAddr := os.Getenv("OS_DNS_ADDR")
	if len(dnsAddr) == 0 {
		if addr, err := FindAvailableBindAddress(10000, 29999); err != nil {
			glog.Fatalf("Couldn't find free address for DNS: %v", err)
		} else {
			dnsAddr = addr
		}
	}
	masterArgs.DNSBindAddr.Set(dnsAddr)

	return masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs
}

func DefaultMasterOptions() (*configapi.MasterConfig, error) {
	return DefaultMasterOptionsWithTweaks(false)
}

func DefaultMasterOptionsWithTweaks(useDefaultPort bool) (*configapi.MasterConfig, error) {
	startOptions := start.MasterOptions{}
	startOptions.MasterArgs, _, _, _, _ = setupStartOptions(useDefaultPort)
	startOptions.Complete()
	// reset, since Complete alters the default
	startOptions.MasterArgs.ConfigDir.Default(path.Join(util.GetBaseDir(), "openshift.local.config", "master"))

	if err := CreateMasterCerts(startOptions.MasterArgs); err != nil {
		return nil, err
	}

	masterConfig, err := startOptions.MasterArgs.BuildSerializeableMasterConfig()
	if err != nil {
		return nil, err
	}

	if masterConfig.AdmissionConfig.PluginConfig == nil {
		masterConfig.AdmissionConfig.PluginConfig = make(map[string]*configapi.AdmissionPluginConfig)
	}

	if masterConfig.EtcdConfig != nil {
		addr, err := FindAvailableBindAddress(10000, 29999)
		if err != nil {
			return nil, fmt.Errorf("can't setup etcd address: %v", err)
		}
		peerAddr, err := FindAvailableBindAddress(10000, 29999)
		if err != nil {
			return nil, fmt.Errorf("can't setup etcd address: %v", err)
		}
		masterConfig.EtcdConfig.Address = addr
		masterConfig.EtcdConfig.ServingInfo.BindAddress = masterConfig.EtcdConfig.Address
		masterConfig.EtcdConfig.PeerAddress = peerAddr
		masterConfig.EtcdConfig.PeerServingInfo.BindAddress = masterConfig.EtcdConfig.PeerAddress
		masterConfig.EtcdClientInfo.URLs = []string{"https://" + masterConfig.EtcdConfig.Address}
	}

	masterConfig.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds = 1
	allowedRegistries := append(
		*configapi.DefaultAllowedRegistriesForImport,
		configapi.RegistryLocation{DomainName: "127.0.0.1:*"},
	)
	for r := range util.GetAdditionalAllowedRegistries() {
		allowedRegistries = append(allowedRegistries, configapi.RegistryLocation{DomainName: r})
	}
	masterConfig.ImagePolicyConfig.AllowedRegistriesForImport = &allowedRegistries

	// force strict handling of service account secret references by default, so that all our examples and controllers will handle it.
	masterConfig.ServiceAccountConfig.LimitSecretReferences = true

	glog.Infof("Starting integration server from master %s", startOptions.MasterArgs.ConfigDir.Value())

	return masterConfig, nil
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

		ExpireDays:       crypto.DefaultCertificateLifetimeInDays,
		SignerExpireDays: crypto.DefaultCACertificateLifetimeInDays,

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

func CreateNodeCerts(nodeArgs *start.NodeArgs, masterURL string) error {
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
	createNodeConfig.APIServerURL = masterURL
	createNodeConfig.APIServerCAFiles = []string{admin.DefaultCertFilename(nodeArgs.MasterCertDir, "ca")}
	createNodeConfig.NodeClientCAFile = admin.DefaultCertFilename(nodeArgs.MasterCertDir, "ca")

	if err := createNodeConfig.Validate(nil); err != nil {
		return err
	}
	if _, err := createNodeConfig.CreateNodeFolder(); err != nil {
		return err
	}

	return nil
}

func DefaultAllInOneOptions() (*configapi.MasterConfig, *configapi.NodeConfig, *utilflags.ComponentFlag, error) {
	startOptions := start.AllInOneOptions{MasterOptions: &start.MasterOptions{}, NodeArgs: &start.NodeArgs{}}
	startOptions.MasterOptions.MasterArgs, startOptions.NodeArgs, _, _, _ = setupStartOptions(false)
	startOptions.NodeArgs.AllowDisabledDocker = true
	startOptions.NodeArgs.Components.Disable("plugins", "proxy", "dns")
	startOptions.ServiceNetworkCIDR = start.NewDefaultNetworkArgs().ServiceNetworkCIDR
	if err := startOptions.Complete(); err != nil {
		return nil, nil, nil, err
	}
	startOptions.MasterOptions.MasterArgs.ConfigDir.Default(path.Join(util.GetBaseDir(), "openshift.local.config", "master"))
	startOptions.NodeArgs.ConfigDir.Default(path.Join(util.GetBaseDir(), "openshift.local.config", admin.DefaultNodeDir(startOptions.NodeArgs.NodeName)))
	startOptions.NodeArgs.MasterCertDir = startOptions.MasterOptions.MasterArgs.ConfigDir.Value()

	if err := CreateMasterCerts(startOptions.MasterOptions.MasterArgs); err != nil {
		return nil, nil, nil, err
	}
	if err := CreateNodeCerts(startOptions.NodeArgs, startOptions.MasterOptions.MasterArgs.MasterAddr.String()); err != nil {
		return nil, nil, nil, err
	}

	masterConfig, err := startOptions.MasterOptions.MasterArgs.BuildSerializeableMasterConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	if masterConfig.EtcdConfig != nil {
		addr, err := FindAvailableBindAddress(10000, 29999)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("can't setup etcd address: %v", err)
		}
		peerAddr, err := FindAvailableBindAddress(10000, 29999)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("can't setup etcd address: %v", err)
		}
		masterConfig.EtcdConfig.Address = addr
		masterConfig.EtcdConfig.ServingInfo.BindAddress = masterConfig.EtcdConfig.Address
		masterConfig.EtcdConfig.PeerAddress = peerAddr
		masterConfig.EtcdConfig.PeerServingInfo.BindAddress = masterConfig.EtcdConfig.PeerAddress
		masterConfig.EtcdClientInfo.URLs = []string{"https://" + masterConfig.EtcdConfig.Address}
	}

	if fn := startOptions.MasterOptions.MasterArgs.OverrideConfig; fn != nil {
		if err := fn(masterConfig); err != nil {
			return nil, nil, nil, err
		}
	}

	nodeConfig, err := startOptions.NodeArgs.BuildSerializeableNodeConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	nodeConfig.DockerConfig.DockerShimSocket = path.Join(util.GetBaseDir(), "dockershim.sock")
	nodeConfig.DockerConfig.DockershimRootDirectory = path.Join(util.GetBaseDir(), "dockershim")

	return masterConfig, nodeConfig, startOptions.NodeArgs.Components, nil
}

func StartConfiguredAllInOne(masterConfig *configapi.MasterConfig, nodeConfig *configapi.NodeConfig, components *utilflags.ComponentFlag) (string, error) {
	adminKubeConfigFile, err := StartConfiguredMaster(masterConfig)
	if err != nil {
		return "", err
	}

	if err := StartConfiguredNode(nodeConfig, components); err != nil {
		return "", err
	}

	return adminKubeConfigFile, nil
}

func StartTestAllInOne() (*configapi.MasterConfig, *configapi.NodeConfig, string, error) {
	master, node, components, err := DefaultAllInOneOptions()
	if err != nil {
		return nil, nil, "", err
	}

	adminKubeConfigFile, err := StartConfiguredAllInOne(master, node, components)
	return master, node, adminKubeConfigFile, err
}

func MasterEtcdClients(config *configapi.MasterConfig) (*etcdclientv3.Client, error) {
	etcd3, err := etcd.MakeEtcdClientV3(config.EtcdClientInfo)
	if err != nil {
		return nil, err
	}
	return etcd3, nil
}

func CleanupMasterEtcd(t *testing.T, config *configapi.MasterConfig) {
	etcd3, err := MasterEtcdClients(config)
	if err != nil {
		t.Logf("Unable to get etcd client available for master: %v", err)
	}
	dumpEtcdOnFailure(t, etcd3)
	if config.EtcdConfig != nil {
		if len(config.EtcdConfig.StorageDir) > 0 {
			if err := os.RemoveAll(config.EtcdConfig.StorageDir); err != nil {
				t.Logf("Unable to clean up the config storage directory %s: %v", config.EtcdConfig.StorageDir, err)
			}
		}
	}
}

func StartConfiguredNode(nodeConfig *configapi.NodeConfig, components *utilflags.ComponentFlag) error {
	guardNode()

	_, nodePort, err := net.SplitHostPort(nodeConfig.ServingInfo.BindAddress)
	if err != nil {
		return err
	}

	if err := start.StartNode(*nodeConfig, components); err != nil {
		return err
	}

	// wait for the server to come up for 30 seconds (average time on desktop is 2 seconds, but Jenkins timed out at 10 seconds)
	if err := cmdutil.WaitForSuccessfulDial(true, "tcp", net.JoinHostPort(nodeConfig.NodeName, nodePort), 100*time.Millisecond, 1*time.Second, 30); err != nil {
		return err
	}

	return nil
}

func StartConfiguredMaster(masterConfig *configapi.MasterConfig) (string, error) {
	return StartConfiguredMasterWithOptions(masterConfig)
}

func StartConfiguredMasterAPI(masterConfig *configapi.MasterConfig) (string, error) {
	// we need to unconditionally start this controller for rbac permissions to work
	if masterConfig.KubernetesMasterConfig.ControllerArguments == nil {
		masterConfig.KubernetesMasterConfig.ControllerArguments = map[string][]string{}
	}
	masterConfig.KubernetesMasterConfig.ControllerArguments["controllers"] = append(masterConfig.KubernetesMasterConfig.ControllerArguments["controllers"], "serviceaccount-token", "clusterrole-aggregation")

	return StartConfiguredMasterWithOptions(masterConfig)
}

func StartConfiguredMasterWithOptions(masterConfig *configapi.MasterConfig) (string, error) {
	guardMaster()
	if masterConfig.EtcdConfig != nil && len(masterConfig.EtcdConfig.StorageDir) > 0 {
		os.RemoveAll(masterConfig.EtcdConfig.StorageDir)
	}
	if err := start.NewMaster(masterConfig, true /* always needed for cluster role aggregation */, true).Start(); err != nil {
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

	var healthzResponse string
	err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
		var healthy bool
		healthy, healthzResponse, err = IsServerHealthy(*masterURL, masterConfig.OAuthConfig != nil)
		if err != nil {
			return false, err
		}
		return healthy, nil
	})
	if err == wait.ErrWaitTimeout {
		return "", fmt.Errorf("server did not become healthy: %v", healthzResponse)
	}
	if err != nil {
		return "", err
	}

	// wait until the cluster roles have been aggregated
	clusterAdminClientConfig, err := util.GetClusterAdminClientConfig(adminKubeConfigFile)
	if err != nil {
		return "", err
	}
	err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
		kubeClient, err := kubeclient.NewForConfig(clusterAdminClientConfig)
		if err != nil {
			return false, err
		}
		admin, err := kubeClient.RbacV1().ClusterRoles().Get("admin", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(admin.Rules) == 0 {
			return false, nil
		}
		edit, err := kubeClient.RbacV1().ClusterRoles().Get("edit", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(edit.Rules) == 0 {
			return false, nil
		}
		view, err := kubeClient.RbacV1().ClusterRoles().Get("view", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(view.Rules) == 0 {
			return false, nil
		}

		return true, nil
	})
	if err == wait.ErrWaitTimeout {
		return "", fmt.Errorf("server did not become healthy: %v", healthzResponse)
	}
	if err != nil {
		return "", err
	}

	return adminKubeConfigFile, nil
}

func IsServerHealthy(url url.URL, checkOAuth bool) (bool, string, error) {
	healthy, healthzResponse, err := isServerPathHealthy(url, "/healthz", http.StatusOK)
	if err != nil || !healthy || !checkOAuth {
		return healthy, healthzResponse, err
	}
	// As a special case, check this endpoint as well since the OAuth server is not part of the /healthz check
	// Whenever the OAuth server gets split out, it would have its own /healthz and post start hooks to handle this
	return isServerPathHealthy(url, "/oauth/token/request", http.StatusFound)
}

func isServerPathHealthy(url url.URL, path string, code int) (bool, string, error) {
	transport := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	url.Path = path
	req, err := http.NewRequest("GET", url.String(), nil)
	req.Header.Set("Accept", "text/html")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	content, _ := ioutil.ReadAll(resp.Body)

	return resp.StatusCode == code, string(content), nil
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

func StartTestMasterAPI() (*configapi.MasterConfig, string, error) {
	master, err := DefaultMasterOptions()
	if err != nil {
		return nil, "", err
	}

	adminKubeConfigFile, err := StartConfiguredMasterAPI(master)
	return master, adminKubeConfigFile, err
}

// serviceAccountSecretsExist checks whether the given service account has at least a token and a dockercfg
// secret associated with it.
func serviceAccountSecretsExist(clientset kclientset.Interface, namespace string, sa *kapi.ServiceAccount) bool {
	foundTokenSecret := false
	foundDockercfgSecret := false
	for _, secret := range sa.Secrets {
		ns := namespace
		if len(secret.Namespace) > 0 {
			ns = secret.Namespace
		}
		secret, err := clientset.Core().Secrets(ns).Get(secret.Name, metav1.GetOptions{})
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

// WaitForPodCreationServiceAccounts ensures that the service account needed for pod creation exists
// and that the cache for the admission control that checks for pod tokens has caught up to allow
// pod creation.
func WaitForPodCreationServiceAccounts(clientset kclientset.Interface, namespace string) error {
	if err := WaitForServiceAccounts(clientset, namespace, []string{bootstrappolicy.DefaultServiceAccountName}); err != nil {
		return err
	}

	testPod := &kapi.Pod{}
	testPod.GenerateName = "test"
	testPod.Spec.Containers = []kapi.Container{
		{
			Name:  "container",
			Image: "openshift/origin-pod:latest",
		},
	}

	return wait.PollImmediate(time.Second, PodCreationWaitTimeout, func() (bool, error) {
		pod, err := clientset.Core().Pods(namespace).Create(testPod)
		if err != nil {
			glog.Warningf("Error attempting to create test pod: %v", err)
			return false, nil
		}
		err = clientset.Core().Pods(namespace).Delete(pod.Name, metav1.NewDeleteOptions(0))
		if err != nil {
			return false, err
		}
		return true, nil
	})
}

// WaitForServiceAccounts ensures the service accounts needed by build pods exist in the namespace
// The extra controllers tend to starve the service account controller
func WaitForServiceAccounts(clientset kclientset.Interface, namespace string, accounts []string) error {
	serviceAccounts := clientset.Core().ServiceAccounts(namespace)
	return wait.Poll(time.Second, ServiceAccountWaitTimeout, func() (bool, error) {
		for _, account := range accounts {
			sa, err := serviceAccounts.Get(account, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			if !serviceAccountSecretsExist(clientset, namespace, sa) {
				return false, nil
			}
		}
		return true, nil
	})
}

// CreateNewProject creates a new project using the clusterAdminClient, then gets a token for the adminUser and returns
// back a client for the admin user
func CreateNewProject(clientConfig *restclient.Config, projectName, adminUser string) (kclientset.Interface, *restclient.Config, error) {
	projectClient, err := projectclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}
	authorizationClient, err := authorizationclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}
	authorizationInterface := authorizationClient.Authorization()

	newProjectOptions := &newproject.NewProjectOptions{
		ProjectClient:     projectClient,
		RoleBindingClient: authorizationInterface,
		SARClient:         authorizationInterface.SubjectAccessReviews(),
		ProjectName:       projectName,
		AdminRole:         bootstrappolicy.AdminRoleName,
		AdminUser:         adminUser,
	}

	if err := newProjectOptions.Run(false); err != nil {
		return nil, nil, err
	}

	kubeClient, config, err := util.GetClientForUser(clientConfig, adminUser)
	return kubeClient, config, err
}
