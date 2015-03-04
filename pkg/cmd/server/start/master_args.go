package start

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/ghodss/yaml"
	"github.com/spf13/pflag"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	latestconfigapi "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/certs"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// MasterArgs is a struct that the command stores flag values into.  It holds a partially complete set of parameters for starting the master
// This object should hold the common set values, but not attempt to handle all cases.  The expected path is to use this object to create
// a fully specified config later on.  If you need something not set here, then create a fully specified config file and pass that as argument
// to starting the master.
type MasterArgs struct {
	MasterAddr flagtypes.Addr
	EtcdAddr   flagtypes.Addr
	PortalNet  flagtypes.IPNet
	// addresses for external clients
	MasterPublicAddr     flagtypes.Addr
	AssetPublicAddr      flagtypes.Addr
	KubernetesPublicAddr flagtypes.Addr

	// AssetBindAddr exposed for integration tests to set
	AssetBindAddr flagtypes.Addr
	// DNSBindAddr exposed for integration tests to set
	DNSBindAddr flagtypes.Addr

	EtcdDir string

	NodeList util.StringList

	CORSAllowedOrigins util.StringList

	BindAddrArg        *BindAddrArg
	ImageFormatArgs    *ImageFormatArgs
	KubeConnectionArgs *KubeConnectionArgs
	CertArgs           *CertArgs
}

// BindMasterArgs binds the options to the flags with prefix + default flag names
func BindMasterArgs(args *MasterArgs, flags *pflag.FlagSet, prefix string) {
	flags.Var(&args.MasterAddr, prefix+"master", "The master address for use by OpenShift components (host, host:port, or URL). Scheme and port default to the --listen scheme and port.")
	flags.Var(&args.MasterPublicAddr, prefix+"public-master", "The master address for use by public clients, if different (host, host:port, or URL). Defaults to same as --master.")
	flags.Var(&args.EtcdAddr, prefix+"etcd", "The address of the etcd server (host, host:port, or URL). If specified, no built-in etcd will be started.")
	flags.Var(&args.KubernetesPublicAddr, prefix+"public-kubernetes", "The Kubernetes server address for use by public clients, if different. (host, host:port, or URL). Defaults to same as --kubernetes.")
	flags.Var(&args.PortalNet, prefix+"portal-net", "A CIDR notation IP range from which to assign portal IPs. This must not overlap with any IP ranges assigned to nodes for pods.")

	flags.StringVar(&args.EtcdDir, prefix+"etcd-dir", "openshift.local.etcd", "The etcd data directory.")

	flags.Var(&args.NodeList, prefix+"nodes", "The hostnames of each node. This currently must be specified up front. Comma delimited list")
	flags.Var(&args.CORSAllowedOrigins, prefix+"cors-allowed-origins", "List of allowed origins for CORS, comma separated.  An allowed origin can be a regular expression to support subdomain matching.  CORS is enabled for localhost, 127.0.0.1, and the asset server by default.")
}

// NewDefaultMasterArgs creates MasterArgs with sub-objects created and default values set.
func NewDefaultMasterArgs() *MasterArgs {
	config := &MasterArgs{
		MasterAddr:           flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		EtcdAddr:             flagtypes.Addr{Value: "0.0.0.0:4001", DefaultScheme: "http", DefaultPort: 4001}.Default(),
		PortalNet:            flagtypes.DefaultIPNet("172.30.17.0/24"),
		MasterPublicAddr:     flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		KubernetesPublicAddr: flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		AssetPublicAddr:      flagtypes.Addr{Value: "localhost:8444", DefaultScheme: "https", DefaultPort: 8444, AllowPrefix: true}.Default(),
		AssetBindAddr:        flagtypes.Addr{Value: "0.0.0.0:8444", DefaultScheme: "https", DefaultPort: 8444, AllowPrefix: true}.Default(),
		DNSBindAddr:          flagtypes.Addr{Value: "0.0.0.0:53", DefaultScheme: "http", DefaultPort: 53, AllowPrefix: true}.Default(),

		BindAddrArg:        NewDefaultBindAddrArg(),
		ImageFormatArgs:    NewDefaultImageFormatArgs(),
		KubeConnectionArgs: NewDefaultKubeConnectionArgs(),
		CertArgs:           NewDefaultCertArgs(),
	}

	return config
}

// BuildSerializeableMasterConfig takes the MasterArgs (partially complete config) and uses them along with defaulting behavior to create the fully specified
// config object for starting the master
func (args MasterArgs) BuildSerializeableMasterConfig() (*configapi.MasterConfig, error) {
	masterAddr, err := args.GetMasterAddress()
	if err != nil {
		return nil, err
	}
	masterPublicAddr, err := args.GetMasterPublicAddress()
	if err != nil {
		return nil, err
	}
	kubePublicAddr, err := args.GetKubernetesPublicAddress()
	if err != nil {
		return nil, err
	}
	assetPublicAddr, err := args.GetAssetPublicAddress()
	if err != nil {
		return nil, err
	}
	dnsBindAddr, err := args.GetDNSBindAddress()
	if err != nil {
		return nil, err
	}

	corsAllowedOrigins := []string{}
	corsAllowedOrigins = append(corsAllowedOrigins, args.CORSAllowedOrigins...)
	// always include the all-in-one server's web console as an allowed CORS origin
	// always include localhost as an allowed CORS origin
	// always include master public address as an allowed CORS origin
	for _, origin := range []string{assetPublicAddr.Host, masterPublicAddr.Host, "localhost", "127.0.0.1"} {
		corsAllowedOrigins = append(corsAllowedOrigins, origin)
	}

	etcdAddress, err := args.GetEtcdAddress()
	if err != nil {
		return nil, err
	}

	var etcdConfig *configapi.EtcdConfig
	if !args.EtcdAddr.Provided {
		etcdConfig, err = args.BuildSerializeableEtcdConfig()
		if err != nil {
			return nil, err
		}
	}
	var kubernetesMasterConfig *configapi.KubernetesMasterConfig
	if !args.KubeConnectionArgs.KubernetesAddr.Provided && len(args.KubeConnectionArgs.ClientConfigLoadingRules.CommandLinePath) == 0 {
		kubernetesMasterConfig, err = args.BuildSerializeableKubeMasterConfig()
		if err != nil {
			return nil, err
		}
	}

	config := &configapi.MasterConfig{
		ServingInfo: configapi.ServingInfo{
			BindAddress: args.BindAddrArg.BindAddr.URL.Host,
			ServerCert:  certs.DefaultMasterServingCertInfo(args.CertArgs.CertDir),
			ClientCA:    certs.DefaultRootCAFile(args.CertArgs.CertDir),
		},
		CORSAllowedOrigins: corsAllowedOrigins,

		KubernetesMasterConfig: kubernetesMasterConfig,
		EtcdConfig:             etcdConfig,

		OAuthConfig: &configapi.OAuthConfig{
			ProxyCA:         cmdutil.Env("OPENSHIFT_OAUTH_REQUEST_HEADER_CA_FILE", ""),
			MasterURL:       masterAddr.String(),
			MasterPublicURL: masterPublicAddr.String(),
			AssetPublicURL:  assetPublicAddr.String(),
		},

		AssetConfig: &configapi.AssetConfig{
			ServingInfo: configapi.ServingInfo{
				BindAddress: args.GetAssetBindAddress(),
				ServerCert:  certs.DefaultAssetServingCertInfo(args.CertArgs.CertDir),
				ClientCA:    certs.DefaultRootCAFile(args.CertArgs.CertDir),
			},

			LogoutURI:           cmdutil.Env("OPENSHIFT_LOGOUT_URI", ""),
			MasterPublicURL:     masterPublicAddr.String(),
			PublicURL:           assetPublicAddr.String(),
			KubernetesPublicURL: kubePublicAddr.String(),
		},

		DNSConfig: &configapi.DNSConfig{
			BindAddress: dnsBindAddr.URL.Host,
		},

		MasterClients: configapi.MasterClients{
			DeployerKubeConfig:          certs.DefaultKubeConfigFilename(args.CertArgs.CertDir, "openshift-deployer"),
			OpenShiftLoopbackKubeConfig: certs.DefaultKubeConfigFilename(args.CertArgs.CertDir, "openshift-client"),
			KubernetesKubeConfig:        certs.DefaultKubeConfigFilename(args.CertArgs.CertDir, "kube-client"),
		},

		EtcdClientInfo: configapi.RemoteConnectionInfo{
			URL: etcdAddress.String(),
			// TODO allow for https etcd
			CA:         "",
			ClientCert: configapi.CertInfo{},
		},

		MasterAuthorizationNamespace:      "master",
		OpenShiftSharedResourcesNamespace: "openshift",

		ImageConfig: configapi.ImageConfig{
			Format: args.ImageFormatArgs.ImageTemplate.Format,
			Latest: args.ImageFormatArgs.ImageTemplate.Latest,
		},
	}

	return config, nil
}

// BuildSerializeableEtcdConfig creates a fully specified etcd startup configuration based on MasterArgs
func (args MasterArgs) BuildSerializeableEtcdConfig() (*configapi.EtcdConfig, error) {
	etcdAddr, err := args.GetEtcdAddress()
	if err != nil {
		return nil, err
	}

	config := &configapi.EtcdConfig{
		ServingInfo: configapi.ServingInfo{
			BindAddress: args.GetEtcdBindAddress(),
		},
		PeerAddress:   args.GetEtcdPeerBindAddress(),
		MasterAddress: etcdAddr.Host,
		StorageDir:    args.EtcdDir,
	}

	return config, nil
}

// BuildSerializeableKubeMasterConfig creates a fully specified kubernetes master startup configuration based on MasterArgs
func (args MasterArgs) BuildSerializeableKubeMasterConfig() (*configapi.KubernetesMasterConfig, error) {
	servicesSubnet := net.IPNet(args.PortalNet)

	config := &configapi.KubernetesMasterConfig{
		ServicesSubnet:  servicesSubnet.String(),
		StaticNodeNames: args.NodeList,
	}

	return config, nil
}

// GetServerCertHostnames returns the set of hostnames that any serving certificate for master needs to be valid for.
func (args MasterArgs) GetServerCertHostnames() (util.StringSet, error) {
	masterAddr, err := args.GetMasterAddress()
	if err != nil {
		return nil, err
	}
	masterPublicAddr, err := args.GetMasterPublicAddress()
	if err != nil {
		return nil, err
	}
	kubePublicAddr, err := args.GetKubernetesPublicAddress()
	if err != nil {
		return nil, err
	}
	assetPublicAddr, err := args.GetAssetPublicAddress()
	if err != nil {
		return nil, err
	}

	// 172.17.42.1 enables the router to call back out to the master
	// TODO: Remove 172.17.42.1 once we can figure out how to validate the master's cert from inside a pod, or tell pods the real IP for the master
	allHostnames := util.NewStringSet("localhost", "127.0.0.1", "172.17.42.1", masterAddr.Host, masterPublicAddr.Host, kubePublicAddr.Host, assetPublicAddr.Host)
	certHostnames := util.StringSet{}
	for hostname := range allHostnames {
		if host, _, err := net.SplitHostPort(hostname); err == nil {
			// add the hostname without the port
			certHostnames.Insert(host)
		} else {
			// add the originally specified hostname
			certHostnames.Insert(hostname)
		}
	}

	return certHostnames, nil
}

// GetMasterAddress checks for an unset master address and then attempts to use the first
// public IPv4 non-loopback address registered on this host.
// TODO: make me IPv6 safe
func (args MasterArgs) GetMasterAddress() (*url.URL, error) {
	if args.MasterAddr.Provided {
		return args.MasterAddr.URL, nil
	}

	// If the user specifies a bind address, and the master is not provided, use the bind port by default
	port := args.MasterAddr.Port
	if args.BindAddrArg.BindAddr.Provided {
		port = args.BindAddrArg.BindAddr.Port
	}

	// If the user specifies a bind address, and the master is not provided, use the bind scheme by default
	scheme := args.MasterAddr.URL.Scheme
	if args.BindAddrArg.BindAddr.Provided {
		scheme = args.BindAddrArg.BindAddr.URL.Scheme
	}

	addr := ""
	if ip, err := cmdutil.DefaultLocalIP4(); err == nil {
		addr = ip.String()
	} else if err == cmdutil.ErrorNoDefaultIP {
		addr = "127.0.0.1"
	} else if err != nil {
		return nil, fmt.Errorf("Unable to find a public IP address: %v", err)
	}

	masterAddr := scheme + "://" + net.JoinHostPort(addr, strconv.Itoa(port))
	return url.Parse(masterAddr)
}

func (args MasterArgs) GetDNSBindAddress() (flagtypes.Addr, error) {
	if args.DNSBindAddr.Provided {
		return args.DNSBindAddr, nil
	}
	dnsAddr := flagtypes.Addr{Value: args.BindAddrArg.BindAddr.Host, DefaultPort: 53}.Default()
	return dnsAddr, nil
}

func (args MasterArgs) GetMasterPublicAddress() (*url.URL, error) {
	if args.MasterPublicAddr.Provided {
		return args.MasterPublicAddr.URL, nil
	}

	return args.GetMasterAddress()
}

func (args MasterArgs) GetEtcdBindAddress() string {
	// Derive the etcd bind address by using the bind address and the default etcd port
	return net.JoinHostPort(args.BindAddrArg.BindAddr.Host, strconv.Itoa(args.EtcdAddr.DefaultPort))
}

func (args MasterArgs) GetEtcdPeerBindAddress() string {
	// Derive the etcd peer address by using the bind address and the default etcd peering port
	return net.JoinHostPort(args.BindAddrArg.BindAddr.Host, "7001")
}

func (args MasterArgs) GetEtcdAddress() (*url.URL, error) {
	if args.EtcdAddr.Provided {
		return args.EtcdAddr.URL, nil
	}

	// Etcd should be reachable on the same address that the master is (for simplicity)
	masterAddr, err := args.GetMasterAddress()
	if err != nil {
		return nil, err
	}

	etcdAddr := net.JoinHostPort(getHost(*masterAddr), strconv.Itoa(args.EtcdAddr.DefaultPort))
	return url.Parse(args.EtcdAddr.DefaultScheme + "://" + etcdAddr)
}

func (args MasterArgs) GetKubernetesPublicAddress() (*url.URL, error) {
	if args.KubernetesPublicAddr.Provided {
		return args.KubernetesPublicAddr.URL, nil
	}
	if args.KubeConnectionArgs.KubernetesAddr.Provided {
		return args.KubeConnectionArgs.KubernetesAddr.URL, nil
	}
	config, ok, err := args.KubeConnectionArgs.GetExternalKubernetesClientConfig()
	if err != nil {
		return nil, err
	}
	if ok && len(config.Host) > 0 {
		return url.Parse(config.Host)
	}

	return args.GetMasterPublicAddress()
}

func (args MasterArgs) GetAssetPublicAddress() (*url.URL, error) {
	if args.AssetPublicAddr.Provided {
		return args.AssetPublicAddr.URL, nil
	}
	// Derive the asset public address by incrementing the master public address port by 1
	// TODO: derive the scheme/port from the asset bind scheme/port once that is settable via the command line
	t, err := args.GetMasterPublicAddress()
	if err != nil {
		return nil, err
	}
	assetPublicAddr := *t
	assetPublicAddr.Host = net.JoinHostPort(getHost(assetPublicAddr), strconv.Itoa(getPort(assetPublicAddr)+1))

	return &assetPublicAddr, nil
}

func (args MasterArgs) GetAssetBindAddress() string {
	if args.AssetBindAddr.Provided {
		return args.AssetBindAddr.URL.Host
	}
	// Derive the asset bind address by incrementing the master bind address port by 1
	return net.JoinHostPort(args.BindAddrArg.BindAddr.Host, strconv.Itoa(args.BindAddrArg.BindAddr.Port+1))
}

func getHost(theURL url.URL) string {
	host, _, err := net.SplitHostPort(theURL.Host)
	if err != nil {
		return theURL.Host
	}

	return host
}

func getPort(theURL url.URL) int {
	_, port, err := net.SplitHostPort(theURL.Host)
	if err != nil {
		return 0
	}

	intport, _ := strconv.Atoi(port)
	return intport
}

// WriteMaster serializes the config to yaml.
func WriteMaster(config *configapi.MasterConfig) ([]byte, error) {
	json, err := latestconfigapi.Codec.Encode(config)
	if err != nil {
		return nil, err
	}
	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, nil
}
