package server

import (
	"context"
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

	"k8s.io/kubernetes/cmd/kube-apiserver/app"

	"github.com/openshift/library-go/pkg/config/helpers"

	etcdclientv3 "github.com/coreos/etcd/clientv3"
	"github.com/openshift/library-go/pkg/assets/create"
	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1client "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	kube_controller_manager "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	projectv1typedclient "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/openshift/library-go/pkg/crypto"

	newproject "github.com/openshift/oc/pkg/cli/admin/project"
	openshiftcontrollermanager "github.com/openshift/openshift-controller-manager/pkg/cmd/openshift-controller-manager"
	"github.com/openshift/origin/pkg/api/legacy"
	openshiftapiserver "github.com/openshift/origin/pkg/cmd/openshift-apiserver"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/test/util"
	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/configconversion"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/deprecatedcerts"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/deprecatedclient"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/etcd"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/etcd/etcdserver"
	openshift_controller_manager "github.com/openshift/origin/test/util/server/deprecated_openshift/openshift-controller-manager"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/start"

	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

var (
	// startLock protects access to the start vars
	startLock sync.Mutex
	// startedMaster is true if the master has already been started in process
	startedMaster bool

	openshiftGVs = []schema.GroupVersion{
		{Group: "apps.openshift.io", Version: "v1"},
		{Group: "authorization.openshift.io", Version: "v1"},
		{Group: "build.openshift.io", Version: "v1"},
		{Group: "image.openshift.io", Version: "v1"},
		{Group: "oauth.openshift.io", Version: "v1"},
		{Group: "project.openshift.io", Version: "v1"},
		{Group: "quota.openshift.io", Version: "v1"},
		{Group: "route.openshift.io", Version: "v1"},
		{Group: "security.openshift.io", Version: "v1"},
		{Group: "template.openshift.io", Version: "v1"},
		{Group: "user.openshift.io", Version: "v1"},
	}
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
	ip, err := cmdutil.DefaultLocalIP4()
	if err != nil {
		return "", err
	}
	for port := lowPort; port <= highPort; port++ {
		tryPort := port
		if tryPort == 0 {
			tryPort = int(rand.Int31n(int32(highPort-1024)) + 1024)
		} else {
			tryPort = int(rand.Int31n(int32(highPort-lowPort))) + lowPort
		}
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip.String(), tryPort))
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

func setupStartOptions() *start.MasterArgs {
	masterArgs := start.NewDefaultMasterArgs()

	basedir := util.GetBaseDir()

	// Allows to override the default etcd directory from the shell script.
	etcdDir := os.Getenv("TEST_ETCD_DIR")
	if len(etcdDir) == 0 {
		etcdDir = path.Join(basedir, "etcd")
	}

	masterArgs.EtcdDir = etcdDir
	masterArgs.ConfigDir.Default(path.Join(basedir, "openshift.local.config", "master"))

	// don't wait for nodes to come up
	masterAddr := os.Getenv("OS_MASTER_ADDR")
	if len(masterAddr) == 0 {
		if addr, err := FindAvailableBindAddress(10000, 29999); err != nil {
			klog.Fatalf("Couldn't find free address for master: %v", err)
		} else {
			masterAddr = addr
		}
	}
	masterArgs.MasterAddr.Set(masterAddr)
	masterArgs.ListenArg.ListenAddr.Set(masterAddr)

	dnsAddr := os.Getenv("OS_DNS_ADDR")
	if len(dnsAddr) == 0 {
		if addr, err := FindAvailableBindAddress(10000, 29999); err != nil {
			klog.Fatalf("Couldn't find free address for DNS: %v", err)
		} else {
			dnsAddr = addr
		}
	}
	masterArgs.DNSBindAddr.Set(dnsAddr)

	return masterArgs
}

func DefaultMasterOptions() (*configapi.MasterConfig, error) {
	return DefaultMasterOptionsWithTweaks()
}

func DefaultMasterOptionsWithTweaks() (*configapi.MasterConfig, error) {
	masterArgs := setupStartOptions()
	if !masterArgs.ConfigDir.Provided() {
		masterArgs.ConfigDir.Default("openshift.local.config/master")
	}

	// reset, since Complete alters the default
	masterArgs.ConfigDir.Default(path.Join(util.GetBaseDir(), "openshift.local.config", "master"))
	if err := CreateMasterCerts(masterArgs); err != nil {
		return nil, err
	}

	masterConfig, err := masterArgs.BuildSerializeableMasterConfig()
	if err != nil {
		return nil, err
	}

	if masterConfig.AdmissionConfig.PluginConfig == nil {
		masterConfig.AdmissionConfig.PluginConfig = make(map[string]*configapi.AdmissionPluginConfig)
	}
	masterConfig.KubernetesMasterConfig.APIServerArguments = map[string][]string{}

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

	// List public registries that make sense to allow importing images from by default.
	// By default all registries have set to be "secure", iow. the port for them is
	// defaulted to "443".
	// If the registry you are adding here is insecure, you can add 'Insecure: true' to
	// make it default to port '80'.
	// If the registry you are adding use custom port, you have to specify the port as
	// part of the domain name.
	recommendedAllowedRegistriesForImport := configapi.AllowedRegistries{
		{DomainName: "docker.io"},
		{DomainName: "*.docker.io"},  // registry-1.docker.io
		{DomainName: "*.redhat.com"}, // registry.connect.redhat.com and registry.access.redhat.com
		{DomainName: "gcr.io"},
		{DomainName: "quay.io"},
		{DomainName: "registry.centos.org"},
		{DomainName: "registry.redhat.io"},
	}

	masterConfig.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds = 1
	allowedRegistries := append(
		recommendedAllowedRegistriesForImport,
		configapi.RegistryLocation{DomainName: "127.0.0.1:*"},
	)
	for r := range util.GetAdditionalAllowedRegistries() {
		allowedRegistries = append(allowedRegistries, configapi.RegistryLocation{DomainName: r})
	}
	masterConfig.ImagePolicyConfig.AllowedRegistriesForImport = &allowedRegistries

	klog.Infof("Starting integration server from master %s", masterArgs.ConfigDir.Value())

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

	createMasterCerts := deprecatedcerts.CreateMasterCertsOptions{
		CertDir:    masterArgs.ConfigDir.Value(),
		SignerName: deprecatedcerts.DefaultSignerName(),
		Hostnames:  hostnames.List(),

		ExpireDays:       crypto.DefaultCertificateLifetimeInDays,
		SignerExpireDays: crypto.DefaultCACertificateLifetimeInDays,

		APIServerURL:       masterURL.String(),
		PublicAPIServerURL: publicMasterURL.String(),

		IOStreams: genericclioptions.IOStreams{Out: os.Stderr},
	}

	if err := createMasterCerts.Validate(nil); err != nil {
		return err
	}
	if err := createMasterCerts.CreateMasterCerts(); err != nil {
		return err
	}

	return nil
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

// The returned channel can be waited on to gracefully shutdown the API server.
func StartConfiguredMasterWithOptions(masterConfig *configapi.MasterConfig, stopCh <-chan struct{}) (string, error) {
	guardMaster()

	// openshift apiserver needs its own scheme, but this installs it for now.  oc needs it off, openshift apiserver needs it on. awesome.
	legacy.InstallInternalLegacyAll(legacyscheme.Scheme)

	// setup clients et all
	adminKubeConfigFile := util.KubeConfigPath()
	clientConfig, err := util.GetClusterAdminClientConfig(adminKubeConfigFile)
	if err != nil {
		return "", err
	}

	if err := startEtcd(masterConfig.EtcdConfig, masterConfig.EtcdClientInfo); err != nil {
		return "", err
	}

	if err := startKubernetesAPIServer(masterConfig, clientConfig, stopCh); err != nil {
		return "", err
	}

	// SCCs are required for OpenShift apiserver bootstrap process
	util.WaitForSecurityContextConstraintsCRDAvailable(clientConfig)

	if err := startOpenShiftAPIServer(masterConfig, clientConfig, stopCh); err != nil {
		return "", err
	}

	if err := startKubernetesControllers(masterConfig, adminKubeConfigFile); err != nil {
		return "", err
	}

	if err := startOpenShiftControllers(masterConfig); err != nil {
		return "", err
	}

	err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
		kubeClient, err := kubernetes.NewForConfig(clientConfig)
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
	if err != nil {
		return "", fmt.Errorf("Waiting for roles failed with: %v", err)
	}

	// the openshift controller manager creates its lease object in this
	// namespace, so we have to ensure it exists.
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	ns := &corev1.Namespace{}
	ns.Name = "openshift-controller-manager"
	kubeClient.CoreV1().Namespaces().Create(ns)

	return adminKubeConfigFile, nil
}

func startEtcd(etcdConfig *configapi.EtcdConfig, etcdClientInfo configapi.EtcdConnectionInfo) error {
	if etcdConfig != nil && len(etcdConfig.StorageDir) > 0 {
		os.RemoveAll(etcdConfig.StorageDir)
	}
	etcdserver.RunEtcd(etcdConfig)
	etcdClient3, err := etcd.MakeEtcdClientV3(etcdClientInfo)
	if err != nil {
		return err
	}
	defer etcdClient3.Close()
	if err := etcd.TestEtcdClientV3(etcdClient3); err != nil {
		return err
	}
	return nil
}

var configGroupVersioner = schema.GroupVersions([]schema.GroupVersion{
	{Group: "autoscaling.openshift.io", Version: "v1"},
	{Group: "image.openshift.io", Version: "v1"},
	{Group: "network.openshift.io", Version: "v1"},
	{Group: "scheduling.openshift.io", Version: "v1"},
	{Group: "project.openshift.io", Version: "v1"},
	legacyconfigv1.LegacySchemeGroupVersion,
})

func startKubernetesAPIServer(masterConfig *configapi.MasterConfig, clientConfig *restclient.Config, stopCh <-chan struct{}) error {
	// TODO: replace this with a default which produces the new configs
	uncastExternalMasterConfig, err := configconversion.Scheme.ConvertToVersion(masterConfig, configGroupVersioner)
	if err != nil {
		return err
	}
	legacyConfigCodec := configconversion.Codecs.LegacyCodec(
		legacyconfigv1.LegacySchemeGroupVersion,
		schema.GroupVersion{Group: "autoscaling.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "scheduling.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "project.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "apiserver.k8s.io", Version: "v1alpha1"},
		schema.GroupVersion{Group: "audit.k8s.io", Version: "v1alpha1"},
		schema.GroupVersion{Group: "admission.config.openshift.io", Version: "v1"},
	)
	externalBytes, err := runtime.Encode(legacyConfigCodec, uncastExternalMasterConfig)
	if err != nil {
		return err
	}
	externalMasterConfig := &legacyconfigv1.MasterConfig{}
	gvk := legacyconfigv1.LegacySchemeGroupVersion.WithKind("MasterConfig")
	_, _, err = legacyConfigCodec.Decode(externalBytes, &gvk, externalMasterConfig)
	if err != nil {
		return err
	}

	kubeAPIServerConfig, err := configconversion.ConvertMasterConfigToKubeAPIServerConfig(externalMasterConfig)
	if err != nil {
		return err
	}
	// we need to set enable-aggregator-routing so that APIServices are resolved from Endpoints
	kubeAPIServerConfig.APIServerArguments["enable-aggregator-routing"] = kubecontrolplanev1.Arguments{"true"}
	kubeAPIServerConfig.APIServerArguments["audit-log-format"] = kubecontrolplanev1.Arguments{"json"}
	json, err := helpers.WriteYAML(kubeAPIServerConfig, kubecontrolplanev1.Install)
	if err != nil {
		return err
	}
	configFile, err := ioutil.TempFile("", "kube-apiserver-config-*.yaml")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(configFile.Name(), json, 0644); err != nil {
		return err
	}

	command := app.NewAPIServerCommand(stopCh)
	if err := command.ParseFlags([]string{"--openshift-config=" + configFile.Name()}); err != nil {
		return err
	}
	go func() {
		command.RunE(command, nil)
	}()

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	crdManifestsDir := "../../hack/local-up-master/kube-apiserver-manifests"
	if _, err := os.Stat(crdManifestsDir); os.IsNotExist(err) {
		return fmt.Errorf("directory with CRD manifests %q not found", crdManifestsDir)
	}
	if err := create.EnsureManifestsCreated(ctx, crdManifestsDir, clientConfig, create.CreateOptions{
		Verbose: true,
	}); err != nil {
		return err
	}

	serverURL, err := url.Parse(fmt.Sprintf("https://%s", masterConfig.ServingInfo.BindAddress))
	if err := waitForServerHealthy(serverURL); err != nil {
		return fmt.Errorf("Waiting for Kubernetes API /healthz failed with: %v", err)
	}

	return nil
}

func startOpenShiftAPIServer(masterConfig *configapi.MasterConfig, clientConfig *restclient.Config, stopCh <-chan struct{}) error {
	// TODO: replace this with a default which produces the new configs
	uncastExternalMasterConfig, err := configconversion.Scheme.ConvertToVersion(masterConfig, configGroupVersioner)
	if err != nil {
		return err
	}
	legacyConfigCodec := configconversion.Codecs.LegacyCodec(
		legacyconfigv1.LegacySchemeGroupVersion,
		schema.GroupVersion{Group: "autoscaling.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "scheduling.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "project.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "apiserver.k8s.io", Version: "v1alpha1"},
		schema.GroupVersion{Group: "audit.k8s.io", Version: "v1alpha1"},
		schema.GroupVersion{Group: "admission.config.openshift.io", Version: "v1"},
	)
	externalBytes, err := runtime.Encode(legacyConfigCodec, uncastExternalMasterConfig)
	if err != nil {
		return err
	}
	externalMasterConfig := &legacyconfigv1.MasterConfig{}
	gvk := legacyconfigv1.LegacySchemeGroupVersion.WithKind("MasterConfig")
	_, _, err = legacyConfigCodec.Decode(externalBytes, &gvk, externalMasterConfig)
	if err != nil {
		return err
	}

	openshiftAPIServerConfig, err := configconversion.ConvertMasterConfigToOpenShiftAPIServerConfig(externalMasterConfig)
	if err != nil {
		return err
	}
	openshiftAddrStr, err := FindAvailableBindAddress(10000, 29999)
	if err != nil {
		return fmt.Errorf("couldn't find free address for OpenShift API: %v", err)
	}
	openshiftAPIServerConfig.ServingInfo.BindAddress = openshiftAddrStr
	go func() {
		if err := openshiftapiserver.RunOpenShiftAPIServer(openshiftAPIServerConfig, stopCh); err != nil {
			klog.Errorf("openshift-apiserver terminated: %v", err)
		} else {
			klog.Info("openshift-apiserver terminated cleanly")
		}
	}()

	openshiftAddr, err := url.Parse(fmt.Sprintf("https://%s", openshiftAddrStr))
	if err != nil {
		return err
	}
	targetPort := intstr.Parse(openshiftAddr.Port())

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	apiregistrationclient, err := apiregistrationv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "openshift",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       443,
					TargetPort: targetPort,
				},
			},
		},
	}
	if _, err := kubeClient.CoreV1().Services("default").Create(service); err != nil {
		return err
	}

	kubeAddr, err := url.Parse(clientConfig.Host)
	if err != nil {
		return err
	}
	endpoint := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "openshift",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{{IP: kubeAddr.Hostname()}},
				Ports: []corev1.EndpointPort{
					{
						Name:     "https",
						Protocol: corev1.ProtocolTCP,
						Port:     int32(targetPort.IntValue()),
					},
				},
			},
		},
	}
	if _, err := kubeClient.CoreV1().Endpoints("default").Create(endpoint); err != nil {
		return err
	}

	for _, apiService := range openshiftGVs {
		obj := &apiregistrationv1.APIService{
			ObjectMeta: metav1.ObjectMeta{
				Name: apiService.Version + "." + apiService.Group,
			},
			Spec: apiregistrationv1.APIServiceSpec{
				Group:   apiService.Group,
				Version: apiService.Version,
				Service: &apiregistrationv1.ServiceReference{
					Namespace: "default",
					Name:      "openshift",
				},
				GroupPriorityMinimum: 20000,
				VersionPriority:      15,
				// FIXME: so that we don't have to skip TLS verification
				InsecureSkipTLSVerify: true,
			},
		}

		if _, err := apiregistrationclient.APIServices().Create(obj); err != nil {
			return err
		}
	}

	if err := waitForServerHealthy(openshiftAddr); err != nil {
		return fmt.Errorf("Waiting for OpenShift API /healthz failed with: %v", err)
	}

	err = wait.Poll(time.Second, 3*time.Minute, func() (bool, error) {
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
		if err != nil {
			return false, err
		}
		// wait for openshift APIs
		for _, gv := range openshiftGVs {
			if _, err := discoveryClient.RESTClient().Get().AbsPath("/apis/" + gv.Group).DoRaw(); err != nil {
				return false, nil
			}
		}
		// and /oauth/token/request
		if masterConfig.OAuthConfig != nil {
			if _, err := discoveryClient.RESTClient().Get().AbsPath("/oauth/token/request").DoRaw(); err != nil {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("Waiting for OpenShift APIs failed with: %v", err)
	}

	return nil
}

func startKubernetesControllers(masterConfig *configapi.MasterConfig, adminKubeConfigFile string) error {
	// copied from pkg/cmd/server/start/start_kube_controller_manager.go
	cmdLineArgs := map[string][]string{}
	cmdLineArgs["controllers"] = []string{
		"*", // start everything but the exceptions
		// not used in openshift
		"-ttl",
		"-bootstrapsigner",
		"-tokencleaner",
	}
	cmdLineArgs["service-account-private-key-file"] = []string{masterConfig.ServiceAccountConfig.PrivateKeyFile}
	cmdLineArgs["root-ca-file"] = []string{masterConfig.ServiceAccountConfig.MasterCA}
	cmdLineArgs["kubeconfig"] = []string{masterConfig.MasterClients.OpenShiftLoopbackKubeConfig}

	cmdLineArgs["use-service-account-credentials"] = []string{"true"}
	cmdLineArgs["cluster-signing-cert-file"] = []string{""}
	cmdLineArgs["cluster-signing-key-file"] = []string{""}
	cmdLineArgs["cert-dir"] = []string{""}
	cmdLineArgs["tls-cert-file"] = []string{masterConfig.ServingInfo.ServerCert.CertFile}
	cmdLineArgs["tls-private-key-file"] = []string{masterConfig.ServingInfo.ServerCert.KeyFile}
	cmdLineArgs["leader-elect"] = []string{"false"}

	kubeAddrStr, err := FindAvailableBindAddress(10000, 29999)
	if err != nil {
		return fmt.Errorf("couldn't find free address for Kubernetes controller-mananger: %v", err)
	}
	kubeAddr, err := url.Parse(fmt.Sprintf("https://%s", kubeAddrStr))
	if err != nil {
		return err
	}
	cmdLineArgs["bind-address"] = []string{kubeAddr.Hostname()}
	cmdLineArgs["secure-port"] = []string{kubeAddr.Port()}

	// we need to explicitly disable insecure port to prevent bind failures due
	// to default port being taken already by other instances of CM running in parallel
	cmdLineArgs["port"] = []string{"0"}

	args := []string{}
	for key, value := range cmdLineArgs {
		for _, token := range value {
			args = append(args, fmt.Sprintf("--%s=%v", key, token))
		}
	}

	go func() {
		cmd := kube_controller_manager.NewControllerManagerCommand(wait.NeverStop)
		if err := cmd.ParseFlags(args); err != nil {
			klog.Errorf("kube-controller-manager failed to parse flags: %v", err)
			return
		}
		cmd.Run(cmd, nil)
		// TODO: print errors here. Now cmd.Run will call os.Exit on error, which stops the test.
	}()

	if err := waitForServerHealthy(kubeAddr); err != nil {
		return fmt.Errorf("Waiting for Kubernetes controller-manager /healthz failed with: %v", err)
	}

	return nil
}

func startOpenShiftControllers(masterConfig *configapi.MasterConfig) error {
	privilegedLoopbackConfig, err := deprecatedclient.GetClientConfig(masterConfig.MasterClients.OpenShiftLoopbackKubeConfig, masterConfig.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return err
	}

	// TODO: replace this with a default which produces the new configs
	uncastExternalMasterConfig, err := configconversion.Scheme.ConvertToVersion(masterConfig, configGroupVersioner)
	if err != nil {
		return err
	}
	legacyConfigCodec := configconversion.Codecs.LegacyCodec(
		legacyconfigv1.LegacySchemeGroupVersion,
		schema.GroupVersion{Group: "autoscaling.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "scheduling.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "project.openshift.io", Version: "v1"},
		schema.GroupVersion{Group: "apiserver.k8s.io", Version: "v1alpha1"},
		schema.GroupVersion{Group: "audit.k8s.io", Version: "v1alpha1"},
		schema.GroupVersion{Group: "admission.config.openshift.io", Version: "v1"},
	)
	externalBytes, err := runtime.Encode(legacyConfigCodec, uncastExternalMasterConfig)
	if err != nil {
		return err
	}
	externalMasterConfig := &legacyconfigv1.MasterConfig{}
	gvk := legacyconfigv1.LegacySchemeGroupVersion.WithKind("MasterConfig")
	_, _, err = legacyConfigCodec.Decode(externalBytes, &gvk, externalMasterConfig)
	if err != nil {
		return err
	}

	openshiftControllerConfig := openshift_controller_manager.ConvertMasterConfigToOpenshiftControllerConfig(externalMasterConfig)
	openshiftAddrStr, err := FindAvailableBindAddress(10000, 29999)
	if err != nil {
		return fmt.Errorf("couldn't find free address for OpenShift controller-manager: %v", err)
	}
	openshiftControllerConfig.ServingInfo.BindAddress = openshiftAddrStr
	go func() {
		if err := openshiftcontrollermanager.RunOpenShiftControllerManager(openshiftControllerConfig, privilegedLoopbackConfig); err != nil {
			klog.Errorf("openshift-controller-manager terminated: %v", err)
		}
		// TODO: stop openshift-controller-manager on exit of the test
	}()

	openshiftAddr, err := url.Parse(fmt.Sprintf("https://%s", openshiftAddrStr))
	if err != nil {
		return err
	}
	if err := waitForServerHealthy(openshiftAddr); err != nil {
		return fmt.Errorf("Waiting for OpenShift controller-manager /healthz failed with: %v", err)
	}

	return nil
}

func waitForServerHealthy(url *url.URL) error {
	if err := waitForSuccessfulDial(url.Scheme == "https", "tcp", url.Host, 100*time.Millisecond, 1*time.Second, 60); err != nil {
		return err
	}
	var healthzResponse string
	err := wait.Poll(time.Second, time.Minute, func() (bool, error) {
		var (
			healthy bool
			err     error
		)
		healthy, healthzResponse, err = isServerPathHealthy(*url, "/healthz", http.StatusOK)
		return healthy, err
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("server did not become healthy: %v", healthzResponse)
	}
	return err
}

// waitForSuccessfulDial attempts to connect to the given address, closing and returning nil on the first successful connection.
func waitForSuccessfulDial(https bool, network, address string, timeout, interval time.Duration, retries int) error {
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i <= retries; i++ {
		dialer := net.Dialer{Timeout: timeout}
		if https {
			conn, err = tls.DialWithDialer(&dialer, network, address, &tls.Config{InsecureSkipVerify: true})
		} else {
			conn, err = dialer.Dial(network, address)
		}
		if err != nil {
			klog.V(5).Infof("Got error %#v, trying again: %#v\n", err, address)
			time.Sleep(interval)
			continue
		}
		conn.Close()
		return nil
	}
	return err
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

// serviceAccountSecretsExist checks whether the given service account has at least a token and a dockercfg
// secret associated with it.
func serviceAccountSecretsExist(clientset kubernetes.Interface, namespace string, sa *corev1.ServiceAccount) bool {
	foundTokenSecret := false
	foundDockercfgSecret := false
	for _, secret := range sa.Secrets {
		ns := namespace
		if len(secret.Namespace) > 0 {
			ns = secret.Namespace
		}
		secret, err := clientset.CoreV1().Secrets(ns).Get(secret.Name, metav1.GetOptions{})
		if err == nil {
			switch secret.Type {
			case corev1.SecretTypeServiceAccountToken:
				foundTokenSecret = true
			case corev1.SecretTypeDockercfg:
				foundDockercfgSecret = true
			}
		}
	}
	return foundTokenSecret && foundDockercfgSecret
}

// WaitForPodCreationServiceAccounts ensures that the service account needed for pod creation exists
// and that the cache for the admission control that checks for pod tokens has caught up to allow
// pod creation.
func WaitForPodCreationServiceAccounts(clientset kubernetes.Interface, namespace string) error {
	if err := WaitForServiceAccounts(clientset, namespace, []string{"default"}); err != nil {
		return err
	}

	testPod := &corev1.Pod{}
	testPod.GenerateName = "test"
	testPod.Spec.Containers = []corev1.Container{
		{
			Name:  "container",
			Image: "openshift/origin-pod:latest",
		},
	}

	return wait.PollImmediate(time.Second, PodCreationWaitTimeout, func() (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Create(testPod)
		if err != nil {
			klog.Warningf("Error attempting to create test pod: %v", err)
			return false, nil
		}
		err = clientset.CoreV1().Pods(namespace).Delete(pod.Name, metav1.NewDeleteOptions(0))
		if err != nil {
			return false, err
		}
		return true, nil
	})
}

// WaitForServiceAccounts ensures the service accounts needed by build pods exist in the namespace
// The extra controllers tend to starve the service account controller
func WaitForServiceAccounts(clientset kubernetes.Interface, namespace string, accounts []string) error {
	serviceAccounts := clientset.CoreV1().ServiceAccounts(namespace)
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
func CreateNewProject(clientConfig *restclient.Config, projectName, adminUser string) (kubernetes.Interface, *restclient.Config, error) {
	projectClient, err := projectv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}
	kubeExternalClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}
	authorizationClient, err := authorizationv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}

	newProjectOptions := &newproject.NewProjectOptions{
		ProjectClient:   projectClient,
		RbacClient:      kubeExternalClient.RbacV1(),
		SARClient:       authorizationClient.SubjectAccessReviews(),
		ProjectName:     projectName,
		AdminRole:       "admin",
		AdminUser:       adminUser,
		UseNodeSelector: false,
		IOStreams:       genericclioptions.NewTestIOStreamsDiscard(),
	}

	if err := newProjectOptions.Run(); err != nil {
		return nil, nil, err
	}

	kubeClient, config, err := util.GetClientForUser(clientConfig, adminUser)
	return kubeClient, config, err
}
