package server

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/capabilities"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"
	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/request/bearertoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	authorizationetcd "github.com/openshift/origin/pkg/authorization/registry/etcd"

	"github.com/openshift/origin/pkg/auth/group"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/util"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	pkgutil "github.com/openshift/origin/pkg/util"

	// Admission control plugins from upstream Kubernetes
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/limitranger"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/namespace/exists"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/resourcedefaults"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/resourcequota"
)

const longCommandDesc = `
Start an OpenShift server

This command helps you launch an OpenShift server. The default mode is all-in-one, which allows
you to run all of the components of an OpenShift system on a server with Docker. Running

    $ openshift start

will start OpenShift listening on all interfaces, launch an etcd server to store persistent
data, and launch the Kubernetes system components. The server will run in the foreground until
you terminate the process.

Note: starting OpenShift without passing the --master address will attempt to find the IP
address that will be visible inside running Docker containers. This is not always successful,
so if you have problems tell OpenShift what public address it will be via --master=<ip>.

You may also pass an optional argument to the start command to start OpenShift in one of the
following roles:

    $ openshift start master --nodes=<host1,host2,host3,...>

      Launches the server and control plane for OpenShift. You may pass a list of the node
      hostnames you want to use, or create nodes via the REST API or 'openshift kube'.

    $ openshift start node --master=<masterIP>

      Launches a new node and attempts to connect to the master on the provided IP.

You may also pass --etcd=<address> to connect to an external etcd server instead of running an
integrated instance, or --kubernetes=<addr> and --kubeconfig=<path> to connect to an existing
Kubernetes cluster.
`

const (
	unauthenticatedUsername = "system:anonymous"

	authenticatedGroup   = "system:authenticated"
	unauthenticatedGroup = "system:unauthenticated"
)

// config is a struct that the command stores flag values into.
type config struct {
	Docker *docker.Helper

	MasterAddr     flagtypes.Addr
	BindAddr       flagtypes.Addr
	EtcdAddr       flagtypes.Addr
	KubernetesAddr flagtypes.Addr
	PortalNet      flagtypes.IPNet
	// addresses for external clients
	MasterPublicAddr     flagtypes.Addr
	KubernetesPublicAddr flagtypes.Addr

	ImageFormat         string
	LatestReleaseImages bool

	ImageTemplate variable.ImageTemplate

	Hostname  string
	VolumeDir string

	EtcdDir string

	CertDir string

	StorageVersion string

	NodeList flagtypes.StringList

	// ClientConfig is used when connecting to Kubernetes from the master, or
	// when connecting to the master from a detached node. If the server is an
	// all-in-one, this value is not used.
	ClientConfig clientcmd.ClientConfig

	CORSAllowedOrigins flagtypes.StringList
}

// NewCommandStartServer provides a CLI handler for 'start' command
func NewCommandStartServer(name string) *cobra.Command {
	hostname, err := defaultHostname()
	if err != nil {
		hostname = "localhost"
		glog.Warningf("Unable to lookup hostname, using %q: %v", hostname, err)
	}

	cfg := &config{
		Docker: docker.NewHelper(),

		MasterAddr:           flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		BindAddr:             flagtypes.Addr{Value: "0.0.0.0:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		EtcdAddr:             flagtypes.Addr{Value: "0.0.0.0:4001", DefaultScheme: "http", DefaultPort: 4001}.Default(),
		KubernetesAddr:       flagtypes.Addr{DefaultScheme: "https", DefaultPort: 8443}.Default(),
		PortalNet:            flagtypes.DefaultIPNet("172.30.17.0/24"),
		MasterPublicAddr:     flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		KubernetesPublicAddr: flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),

		ImageTemplate: variable.NewDefaultImageTemplate(),

		Hostname: hostname,
		NodeList: flagtypes.StringList{"127.0.0.1"},
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [master|node]", name),
		Short: "Launch OpenShift",
		Long:  longCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			if err := start(cfg, args); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flag := cmd.Flags()

	flag.Var(&cfg.BindAddr, "listen", "The address to listen for connections on (host, host:port, or URL).")
	flag.Var(&cfg.MasterAddr, "master", "The master address for use by OpenShift components (host, host:port, or URL). Scheme and port default to the --listen scheme and port.")
	flag.Var(&cfg.MasterPublicAddr, "public-master", "The master address for use by public clients, if different (host, host:port, or URL). Defaults to same as --master.")
	flag.Var(&cfg.EtcdAddr, "etcd", "The address of the etcd server (host, host:port, or URL). If specified, no built-in etcd will be started.")
	flag.Var(&cfg.KubernetesAddr, "kubernetes", "The address of the Kubernetes server (host, host:port, or URL). If specified, no Kubernetes components will be started.")
	flag.Var(&cfg.KubernetesPublicAddr, "public-kubernetes", "The Kubernetes server address for use by public clients, if different. (host, host:port, or URL). Defaults to same as --kubernetes.")
	flag.Var(&cfg.PortalNet, "portal-net", "A CIDR notation IP range from which to assign portal IPs. This must not overlap with any IP ranges assigned to nodes for pods.")

	flag.StringVar(&cfg.ImageTemplate.Format, "images", cfg.ImageTemplate.Format, "When fetching images used by the cluster for important components, use this format on both master and nodes. The latest release will be used by default.")
	flag.BoolVar(&cfg.ImageTemplate.Latest, "latest-images", cfg.ImageTemplate.Latest, "If true, attempt to use the latest images for the cluster instead of the latest release.")

	flag.StringVar(&cfg.VolumeDir, "volume-dir", "openshift.local.volumes", "The volume storage directory.")
	flag.StringVar(&cfg.EtcdDir, "etcd-dir", "openshift.local.etcd", "The etcd data directory.")
	flag.StringVar(&cfg.CertDir, "cert-dir", "openshift.local.certificates", "The certificate data directory.")

	flag.StringVar(&cfg.Hostname, "hostname", cfg.Hostname, "The hostname to identify this node with the master.")
	flag.Var(&cfg.NodeList, "nodes", "The hostnames of each node. This currently must be specified up front. Comma delimited list")
	flag.Var(&cfg.CORSAllowedOrigins, "cors-allowed-origins", "List of allowed origins for CORS, comma separated.  An allowed origin can be a regular expression to support subdomain matching.  CORS is enabled for localhost, 127.0.0.1, and the asset server by default.")

	cfg.ClientConfig = cmdutil.DefaultClientConfig(flag)

	cfg.Docker.InstallFlags(flag)

	return cmd
}

// run launches the appropriate startup modes or returns an error.
func start(cfg *config, args []string) error {
	if len(args) > 1 {
		return errors.New("You may start an OpenShift all-in-one server with no arguments, or pass 'master' or 'node' to run in that role.")
	}

	var startEtcd, startNode, startMaster, startKube bool
	if len(args) == 1 {
		switch args[0] {
		case "master":
			startMaster = true
			startKube = !cfg.KubernetesAddr.Provided
			startEtcd = !cfg.EtcdAddr.Provided
			if err := defaultMasterAddress(cfg); err != nil {
				return err
			}
			glog.Infof("Starting an OpenShift master, reachable at %s (etcd: %s)", cfg.MasterAddr.String(), cfg.EtcdAddr.String())
			if cfg.MasterPublicAddr.Provided {
				glog.Infof("OpenShift master public address is %s", cfg.MasterPublicAddr.String())
			}

		case "node":
			startNode = true

			// TODO client config currently doesn't let you override the defaults
			// so it is defaulting to https://localhost:8443 for MasterAddr if
			// it isn't set by --master or --kubeconfig
			if !cfg.MasterAddr.Provided {
				config, err := cfg.ClientConfig.ClientConfig()
				if err != nil {
					glog.Fatalf("Unable to read client configuration: %v", err)
				}
				if len(config.Host) > 0 {
					cfg.MasterAddr.Set(config.Host)
				}
			}
			if !cfg.KubernetesAddr.Provided {
				cfg.KubernetesAddr = cfg.MasterAddr
			}
			glog.Infof("Starting an OpenShift node, connecting to %s", cfg.MasterAddr.String())

		default:
			return errors.New("You may start an OpenShift all-in-one server with no arguments, or pass 'master' or 'node' to run in that role.")
		}

	} else {
		startMaster = true
		startKube = !cfg.KubernetesAddr.Provided
		startEtcd = !cfg.EtcdAddr.Provided
		startNode = true
		if err := defaultMasterAddress(cfg); err != nil {
			return err
		}

		glog.Infof("Starting an OpenShift all-in-one, reachable at %s (etcd: %s)", cfg.MasterAddr.String(), cfg.EtcdAddr.String())
		if cfg.MasterPublicAddr.Provided {
			glog.Infof("OpenShift master public address is %s", cfg.MasterPublicAddr.String())
		}
	}

	if startKube {
		cfg.KubernetesAddr = cfg.MasterAddr
		cfg.KubernetesPublicAddr = cfg.MasterPublicAddr
	}

	if env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			glog.Infof("Starting profiling endpoint at http://127.0.0.1:6060/debug/pprof/")
			glog.Fatal(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}

	// define a function for resolving components to names
	imageResolverFn := cfg.ImageTemplate.ExpandOrDie
	useLocalImages := env("USE_LOCAL_IMAGES", "false") == "true"

	// the node can reuse an existing client
	var existingKubeClient *kclient.Client

	if startMaster {
		if len(cfg.NodeList) == 1 && cfg.NodeList[0] == "127.0.0.1" {
			cfg.NodeList[0] = cfg.Hostname
		}
		for i, s := range cfg.NodeList {
			s = strings.ToLower(s)
			cfg.NodeList[i] = s
			glog.Infof("  Node: %s", s)
		}

		glog.Infof("Using images from %q", imageResolverFn("<component>"))

		if startEtcd {
			etcdConfig := &etcd.Config{
				BindAddr:     cfg.BindAddr.Host,
				PeerBindAddr: cfg.BindAddr.Host,
				MasterAddr:   cfg.EtcdAddr.URL.Host,
				EtcdDir:      cfg.EtcdDir,
			}
			etcdConfig.Run()
		}

		// Connect and setup etcd interfaces
		etcdClient, err := getEtcdClient(cfg)
		if err != nil {
			return err
		}
		etcdHelper, err := origin.NewEtcdHelper(cfg.StorageVersion, etcdClient)
		if err != nil {
			return fmt.Errorf("Error setting up server storage: %v", err)
		}
		ketcdHelper, err := kmaster.NewEtcdHelper(etcdClient, klatest.Version)
		if err != nil {
			return fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
		}

		requestContextMapper := kapi.NewRequestContextMapper()
		masterAuthorizationNamespace := "master"

		// determine whether public API addresses were specified
		masterPublicAddr := cfg.MasterAddr
		if cfg.MasterPublicAddr.Provided {
			masterPublicAddr = cfg.MasterPublicAddr
		}
		k8sPublicAddr := masterPublicAddr      // assume same place as master
		if cfg.KubernetesPublicAddr.Provided { // specifically set, use that
			k8sPublicAddr = cfg.KubernetesPublicAddr
		} else if cfg.KubernetesAddr.Provided { // separate Kube, assume it is public
			k8sPublicAddr = cfg.KubernetesAddr
		}

		// Derive the asset bind address by incrementing the master bind address port by 1
		assetBindAddr := net.JoinHostPort(cfg.BindAddr.Host, strconv.Itoa(cfg.BindAddr.Port+1))
		// Derive the asset public address by incrementing the master public address port by 1
		assetPublicAddr := *masterPublicAddr.URL
		assetPublicAddr.Host = net.JoinHostPort(masterPublicAddr.Host, strconv.Itoa(masterPublicAddr.Port+1))

		// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
		// TODO: allow configuring this
		// TODO: remove hard-coding of development UI server
		assetPublicAddresses := []string{assetPublicAddr.String(), "http://localhost:9000", "https://localhost:9000"}

		// always include the all-in-one server's web console as an allowed CORS origin
		// always include localhost as an allowed CORS origin
		// always include master public address as an allowed CORS origin
		for _, origin := range []string{assetPublicAddr.Host, masterPublicAddr.URL.Host, "localhost", "127.0.0.1"} {
			// TODO: check if origin is already allowed
			cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, origin)
		}

		osmaster := &origin.MasterConfig{
			TLS:                  cfg.BindAddr.URL.Scheme == "https",
			MasterBindAddr:       cfg.BindAddr.URL.Host,
			MasterAddr:           cfg.MasterAddr.URL.String(),
			MasterPublicAddr:     masterPublicAddr.URL.String(),
			AssetBindAddr:        assetBindAddr,
			AssetPublicAddr:      assetPublicAddr.String(),
			KubernetesAddr:       cfg.KubernetesAddr.URL.String(),
			KubernetesPublicAddr: k8sPublicAddr.URL.String(),

			LogoutURI: env("OPENSHIFT_LOGOUT_URI", ""),

			CORSAllowedOrigins: cfg.CORSAllowedOrigins,

			EtcdHelper: etcdHelper,

			AdmissionControl:              admit.NewAlwaysAdmit(),
			Authorizer:                    newAuthorizer(etcdHelper, masterAuthorizationNamespace),
			AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),
			MasterAuthorizationNamespace:  masterAuthorizationNamespace,
			RequestContextMapper:          requestContextMapper,

			UseLocalImages: useLocalImages,
			ImageFor:       imageResolverFn,
		}

		if startKube {
			// We're running against our own kubernetes server
			osmaster.KubeClientConfig = kclient.Config{
				Host:    cfg.MasterAddr.URL.String(),
				Version: klatest.Version,
			}
		} else {
			// We're running against another kubernetes server
			osmaster.KubeClientConfig = *clientConfigFromKubeConfig(cfg)
		}

		// Build token auth for user's OAuth tokens
		authenticators := []authenticator.Request{}
		tokenAuthenticator, err := origin.GetEtcdTokenAuthenticator(etcdHelper)
		if err != nil {
			glog.Fatalf("Error creating TokenAuthenticator: %v", err)
		}
		authenticators = append(authenticators, bearertoken.New(tokenAuthenticator))
		// Allow token as access_token param for WebSockets
		// TODO: make the param name configurable
		// TODO: limit this authenticator to watch methods, if possible
		// TODO: prevent access_token param from getting logged, if possible
		authenticators = append(authenticators, paramtoken.New("access_token", tokenAuthenticator))

		var roots *x509.CertPool
		if osmaster.TLS {
			// Bootstrap CA
			// TODO: store this (or parts of this) in etcd?
			var err error
			ca, err := crypto.InitCA(cfg.CertDir, fmt.Sprintf("%s@%d", cfg.MasterAddr.Host, time.Now().Unix()))
			if err != nil {
				return fmt.Errorf("Unable to configure certificate authority: %v", err)
			}

			// Bootstrap server certs
			// 172.17.42.1 enables the router to call back out to the master
			// TODO: Remove 172.17.42.1 once we can figure out how to validate the master's cert from inside a pod, or tell pods the real IP for the master
			allHostnames := []string{"localhost", "127.0.0.1", "172.17.42.1", cfg.MasterAddr.Host, masterPublicAddr.URL.Host, k8sPublicAddr.URL.Host, assetPublicAddr.Host}
			certHostnames := []string{}
			for _, hostname := range allHostnames {
				if host, _, err := net.SplitHostPort(hostname); err == nil {
					// add the hostname without the port
					certHostnames = append(certHostnames, host)
				} else {
					// add the originally specified hostname
					certHostnames = append(certHostnames, hostname)
				}
			}
			serverCert, err := ca.MakeServerCert("master", pkgutil.UniqueStrings(certHostnames))
			if err != nil {
				return err
			}
			osmaster.MasterCertFile = serverCert.CertFile
			osmaster.MasterKeyFile = serverCert.KeyFile
			osmaster.AssetCertFile = serverCert.CertFile
			osmaster.AssetKeyFile = serverCert.KeyFile

			// Bootstrap clients
			osClientConfigTemplate := kclient.Config{Host: cfg.MasterAddr.URL.String(), Version: latest.Version}

			// OpenShift client
			openshiftClientUser := &user.DefaultInfo{Name: "system:openshift-client"}
			if osmaster.OSClientConfig, err = ca.MakeClientConfig("openshift-client", openshiftClientUser, osClientConfigTemplate); err != nil {
				return err
			}
			// OpenShift deployer client
			openshiftDeployerUser := &user.DefaultInfo{Name: "system:openshift-deployer", Groups: []string{"system:deployers"}}
			if osmaster.DeployerOSClientConfig, err = ca.MakeClientConfig("openshift-deployer", openshiftDeployerUser, osClientConfigTemplate); err != nil {
				return err
			}
			// Admin config (creates files on disk for osc)
			adminUser := &user.DefaultInfo{Name: "system:admin", Groups: []string{"system:cluster-admins"}}
			if _, err = ca.MakeClientConfig("admin", adminUser, osClientConfigTemplate); err != nil {
				return err
			}

			// One client config per node
			for _, node := range cfg.NodeList {
				nodeIdentityName := fmt.Sprintf("node-%s", node)
				nodeUserName := fmt.Sprintf("system:%s", nodeIdentityName)
				nodeUser := &user.DefaultInfo{Name: nodeUserName, Groups: []string{"system:nodes"}}
				if _, err = ca.MakeClientConfig(nodeIdentityName, nodeUser, osClientConfigTemplate); err != nil {
					return err
				}
			}

			// If we're running our own Kubernetes, build client credentials
			if startKube {
				kubeClientUser := &user.DefaultInfo{Name: "system:kube-client"}
				if osmaster.KubeClientConfig, err = ca.MakeClientConfig("kube-client", kubeClientUser, osmaster.KubeClientConfig); err != nil {
					return err
				}
			}

			// Save cert roots
			roots = x509.NewCertPool()
			for _, root := range ca.Config.Roots {
				roots.AddCert(root)
			}
			osmaster.ClientCAs = roots

			// build cert authenticator
			// TODO: add cert users to etcd?
			opts := x509request.DefaultVerifyOptions()
			opts.Roots = roots
			certauth := x509request.New(opts, x509request.SubjectToUserConversion)
			authenticators = append(authenticators, certauth)
		} else {
			// No security, use the same client config for all OpenShift clients
			osClientConfig := kclient.Config{Host: cfg.MasterAddr.URL.String(), Version: latest.Version}
			osmaster.OSClientConfig = osClientConfig
			osmaster.DeployerOSClientConfig = osClientConfig
		}

		// TODO: make anonymous auth optional?
		osmaster.Authenticator = &unionrequest.Authenticator{
			FailOnError: true,
			Handlers: []authenticator.Request{
				group.NewGroupAdder(unionrequest.NewUnionAuthentication(authenticators...), []string{authenticatedGroup}),
				authenticator.RequestFunc(func(req *http.Request) (user.Info, bool, error) {
					return &user.DefaultInfo{Name: unauthenticatedUsername, Groups: []string{unauthenticatedGroup}}, true, nil
				}),
			},
		}

		osmaster.BuildClients()

		// Default to a session authenticator (for browsers), and a basicauth authenticator (for clients responding to WWW-Authenticate challenges)
		defaultAuthRequestHandlers := strings.Join([]string{
			string(origin.AuthRequestHandlerSession),
			string(origin.AuthRequestHandlerBasicAuth),
		}, ",")
		auth := &origin.AuthConfig{
			MasterAddr:           cfg.MasterAddr.URL.String(),
			MasterPublicAddr:     masterPublicAddr.URL.String(),
			AssetPublicAddresses: assetPublicAddresses,
			MasterRoots:          roots,
			EtcdHelper:           etcdHelper,

			// Max token ages
			AuthorizeTokenMaxAgeSeconds: envInt("OPENSHIFT_OAUTH_AUTHORIZE_TOKEN_MAX_AGE_SECONDS", 300, 1),
			AccessTokenMaxAgeSeconds:    envInt("OPENSHIFT_OAUTH_ACCESS_TOKEN_MAX_AGE_SECONDS", 3600, 1),
			// Handlers
			AuthRequestHandlers: origin.ParseAuthRequestHandlerTypes(env("OPENSHIFT_OAUTH_REQUEST_HANDLERS", defaultAuthRequestHandlers)),
			AuthHandler:         origin.AuthHandlerType(env("OPENSHIFT_OAUTH_HANDLER", string(origin.AuthHandlerLogin))),
			GrantHandler:        origin.GrantHandlerType(env("OPENSHIFT_OAUTH_GRANT_HANDLER", string(origin.GrantHandlerAuto))),
			// RequestHeader config
			RequestHeaders: strings.Split(env("OPENSHIFT_OAUTH_REQUEST_HEADERS", "X-Remote-User"), ","),
			// Session config (default to unknowable secret)
			SessionSecrets:       []string{env("OPENSHIFT_OAUTH_SESSION_SECRET", uuid.NewUUID().String())},
			SessionMaxAgeSeconds: envInt("OPENSHIFT_OAUTH_SESSION_MAX_AGE_SECONDS", 300, 1),
			SessionName:          env("OPENSHIFT_OAUTH_SESSION_NAME", "ssn"),
			// Password config
			PasswordAuth: origin.PasswordAuthType(env("OPENSHIFT_OAUTH_PASSWORD_AUTH", string(origin.PasswordAuthAnyPassword))),
			BasicAuthURL: env("OPENSHIFT_OAUTH_BASIC_AUTH_URL", ""),
			// Token config
			TokenStore:    origin.TokenStoreType(env("OPENSHIFT_OAUTH_TOKEN_STORE", string(origin.TokenStoreOAuth))),
			TokenFilePath: env("OPENSHIFT_OAUTH_TOKEN_FILE_PATH", ""),
			// Google config
			GoogleClientID:     env("OPENSHIFT_OAUTH_GOOGLE_CLIENT_ID", ""),
			GoogleClientSecret: env("OPENSHIFT_OAUTH_GOOGLE_CLIENT_SECRET", ""),
			// GitHub config
			GithubClientID:     env("OPENSHIFT_OAUTH_GITHUB_CLIENT_ID", ""),
			GithubClientSecret: env("OPENSHIFT_OAUTH_GITHUB_CLIENT_SECRET", ""),
		}

		// Allow privileged containers
		// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
		capabilities.Initialize(capabilities.Capabilities{
			AllowPrivileged: true,
		})

		if startKube {
			portalNet := net.IPNet(cfg.PortalNet)
			masterIP := net.ParseIP(cfg.MasterAddr.Host)
			if masterIP == nil {
				addrs, err := net.LookupIP(cfg.MasterAddr.Host)
				if err != nil {
					glog.Fatalf("Unable to find an IP for %q - specify an IP directly? %v", cfg.MasterAddr.Host, err)
				}
				if len(addrs) == 0 {
					glog.Fatalf("Unable to find an IP for %q - specify an IP directly?", cfg.MasterAddr.Host)
				}
				masterIP = addrs[0]
			}

			kmaster := &kubernetes.MasterConfig{
				MasterIP:             masterIP,
				MasterPort:           cfg.MasterAddr.Port,
				NodeHosts:            cfg.NodeList,
				PortalNet:            &portalNet,
				RequestContextMapper: requestContextMapper,
				EtcdHelper:           ketcdHelper,
				KubeClient:           osmaster.KubeClient(),
				Authorizer:           apiserver.NewAlwaysAllowAuthorizer(),
				AdmissionControl:     admit.NewAlwaysAdmit(),
			}
			kmaster.EnsurePortalFlags()

			osmaster.Run([]origin.APIInstaller{kmaster}, []origin.APIInstaller{auth})

			kmaster.RunScheduler()
			kmaster.RunReplicationController()
			kmaster.RunEndpointController()
			kmaster.RunMinionController()
			kmaster.RunResourceQuotaManager()

		} else {
			proxy := &kubernetes.ProxyConfig{
				KubernetesAddr: cfg.KubernetesAddr.URL,
				ClientConfig:   &osmaster.KubeClientConfig,
			}
			osmaster.Run([]origin.APIInstaller{proxy}, []origin.APIInstaller{auth})
		}

		// TODO: recording should occur in individual components
		record.StartRecording(osmaster.KubeClient().Events(""), kapi.EventSource{Component: "master"})

		osmaster.RunAssetServer()
		osmaster.RunBuildController()
		osmaster.RunBuildImageChangeTriggerController()
		osmaster.RunDeploymentController()
		osmaster.RunDeploymentConfigController()
		osmaster.RunDeploymentConfigChangeController()
		osmaster.RunDeploymentImageChangeTriggerController()

		existingKubeClient = osmaster.KubeClient()
	}

	if startNode {
		if existingKubeClient == nil {
			config := clientConfigFromKubeConfig(cfg)
			cli, err := kclient.New(config)
			if err != nil {
				glog.Fatalf("Unable to create a client: %v", err)
			}
			existingKubeClient = cli
		}

		if !startMaster {
			// TODO: recording should occur in individual components
			record.StartRecording(existingKubeClient.Events(""), kapi.EventSource{Component: "node"})
		}

		nodeConfig := &kubernetes.NodeConfig{
			BindHost:   cfg.BindAddr.Host,
			NodeHost:   cfg.Hostname,
			MasterHost: cfg.MasterAddr.URL.String(),

			VolumeDir: cfg.VolumeDir,

			NetworkContainerImage: imageResolverFn("pod"),

			AllowDisabledDocker: startNode && startMaster,

			Client: existingKubeClient,
		}

		nodeConfig.EnsureVolumeDir()
		nodeConfig.EnsureDocker(cfg.Docker)

		nodeConfig.RunProxy()
		nodeConfig.RunKubelet()
	}

	select {}

	return nil
}

func newAuthorizer(etcdHelper tools.EtcdHelper, masterAuthorizationNamespace string) authorizer.Authorizer {
	authorizationEtcd := authorizationetcd.New(etcdHelper)
	authorizer := authorizer.NewAuthorizer(masterAuthorizationNamespace, authorizationEtcd, authorizationEtcd)
	return authorizer
}

func newAuthorizationAttributeBuilder(requestContextMapper kapi.RequestContextMapper) authorizer.AuthorizationAttributeBuilder {
	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, &apiserver.APIRequestInfoResolver{kutil.NewStringSet("api", "osapi"), latest.RESTMapper})
	return authorizationAttributeBuilder
}

// getEtcdClient creates an etcd client based on the provided config and waits
// until etcd server is reachable. It errors out and exits if the server cannot
// be reached for a certain amount of time.
func getEtcdClient(cfg *config) (*etcdclient.Client, error) {
	etcdServers := []string{cfg.EtcdAddr.URL.String()}
	etcdClient := etcdclient.NewClient(etcdServers)

	for i := 0; ; i++ {
		_, err := etcdClient.Get("/", false, false)
		if err == nil || tools.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			return nil, fmt.Errorf("Could not reach etcd: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	return etcdClient, nil
}

// defaultHostname returns the default hostname for this system.
func defaultHostname() (string, error) {
	// Note: We use exec here instead of os.Hostname() because we
	// want the FQDN, and this is the easiest way to get it.
	fqdn, err := exec.Command("hostname", "-f").Output()
	if err != nil {
		return "", fmt.Errorf("Couldn't determine hostname: %v", err)
	}
	return strings.TrimSpace(string(fqdn)), nil
}

// defaultMasterAddress checks for an unset master address and then attempts to use the first
// public IPv4 non-loopback address registered on this host. It will also update the
// EtcdAddr after if it was not provided.
// TODO: make me IPv6 safe
func defaultMasterAddress(cfg *config) error {
	if !cfg.MasterAddr.Provided {
		// If the user specifies a bind address, and the master is not provided, use the bind port by default
		port := cfg.MasterAddr.Port
		if cfg.BindAddr.Provided {
			port = cfg.BindAddr.Port
		}

		// If the user specifies a bind address, and the master is not provided, use the bind scheme by default
		scheme := cfg.MasterAddr.URL.Scheme
		if cfg.BindAddr.Provided {
			scheme = cfg.BindAddr.URL.Scheme
		}

		// use the default ip address for the system
		addr, err := util.DefaultLocalIP4()
		if err != nil {
			return fmt.Errorf("Unable to find the public address of this master: %v", err)
		}

		masterAddr := scheme + "://" + net.JoinHostPort(addr.String(), strconv.Itoa(port))
		if err := cfg.MasterAddr.Set(masterAddr); err != nil {
			return fmt.Errorf("Unable to set public address of this master: %v", err)
		}

		// Prefer to use the MasterAddr for etcd because some clients still connect to it.
		if !cfg.EtcdAddr.Provided {
			etcdAddr := net.JoinHostPort(addr.String(), strconv.Itoa(cfg.EtcdAddr.DefaultPort))
			if err := cfg.EtcdAddr.Set(etcdAddr); err != nil {
				return fmt.Errorf("Unable to set public address of etcd: %v", err)
			}
		}
	} else if !cfg.EtcdAddr.Provided {
		// Etcd should be reachable on the same address that the master is (for simplicity)
		etcdAddr := net.JoinHostPort(cfg.MasterAddr.Host, strconv.Itoa(cfg.EtcdAddr.DefaultPort))
		if err := cfg.EtcdAddr.Set(etcdAddr); err != nil {
			return fmt.Errorf("Unable to set public address of etcd: %v", err)
		}
	}
	return nil
}

// clientConfigFromKubeConfig reads the client configuration settings for connecting to
// a Kubernetes master.
func clientConfigFromKubeConfig(cfg *config) *kclient.Config {
	config, err := cfg.ClientConfig.ClientConfig()
	if err != nil {
		glog.Fatalf("Unable to read client configuration: %v", err)
	}
	if len(config.Version) == 0 {
		config.Version = klatest.Version
	}
	kclient.SetKubernetesDefaults(config)
	if cfg.KubernetesAddr.Provided {
		config.Host = cfg.KubernetesAddr.URL.String()
	}
	return config
}

func envInt(key string, defaultValue int32, minValue int32) int32 {
	value, err := strconv.ParseInt(env(key, fmt.Sprintf("%d", defaultValue)), 10, 32)
	if err != nil || int32(value) < minValue {
		glog.Warningf("Invalid %s. Defaulting to %d.", key, defaultValue)
		return defaultValue
	}
	return int32(value)
}

// env returns an environment variable or a default value if not specified.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

func expandImage(component, defaultValue string, latest bool) string {
	template := defaultValue
	if s, ok := imageComponentEnvExpander(component); ok {
		template = s
	}
	value, err := variable.ExpandStrict(template, func(key string) (string, bool) {
		switch key {
		case "component":
			return component, true
		case "version":
			if latest {
				return "latest", true
			}
		}
		return "", false
	}, variable.Versions)
	if err != nil {
		glog.Fatalf("Unable to find an image for %q due to an error processing the format: %v", err)
	}
	return value
}

func imageComponentEnvExpander(key string) (string, bool) {
	s := strings.Replace(strings.ToUpper(key), "-", "_", -1)
	val := os.Getenv(fmt.Sprintf("OPENSHIFT_%s_IMAGE", s))
	if len(val) == 0 {
		return "", false
	}
	return val, true
}
