package router

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/serviceaccount"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	authapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	configcmd "github.com/openshift/origin/pkg/bulk"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/print"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/generate/app"
	securityclientinternal "github.com/openshift/origin/pkg/security/generated/internalclientset"
	oscc "github.com/openshift/origin/pkg/security/securitycontextconstraints"
	fileutil "github.com/openshift/origin/pkg/util/file"
)

var (
	routerLong = templates.LongDesc(`
		Install or configure a router

		This command helps to setup a router to take edge traffic and balance it to
		your application. With no arguments, the command will check for an existing router
		service called 'router' and create one if it does not exist. If you want to test whether
		a router has already been created add the --dry-run flag and the command will exit with
		1 if the registry does not exist.

		If a router does not exist with the given name, this command will
		create a deployment configuration and service that will run the router. If you are
		running your router in production, you should pass --replicas=2 or higher to ensure
		you have failover protection.`)

	routerExample = templates.Examples(`
		# Check the default router ("router")
	  %[1]s %[2]s --dry-run

	  # See what the router would look like if created
	  %[1]s %[2]s -o yaml

	  # Create a router with two replicas if it does not exist
	  %[1]s %[2]s router-west --replicas=2

	  # Use a different router image
	  %[1]s %[2]s region-west --images=myrepo/somerouter:mytag

	  # Run the router with a hint to the underlying implementation to _not_ expose statistics.
	  %[1]s %[2]s router-west --stats-port=0`)

	secretsVolumeName = "secret-volume"
	secretsPath       = "/etc/secret-volume"

	// this is the official private certificate path on Red Hat distros, and is at least structurally more
	// correct than ubuntu based distributions which don't distinguish between public and private certs.
	// Since Origin is CentOS based this is more likely to work.  Ubuntu images should symlink this directory
	// into /etc/ssl/certs to be compatible.
	defaultCertificateDir = "/etc/pki/tls/private"

	privkeySecretName = "external-host-private-key-secret"
	privkeyVolumeName = "external-host-private-key-volume"
	privkeyName       = "router.pem"
	privkeyPath       = secretsPath + "/" + privkeyName

	defaultCertificatePath = path.Join(defaultCertificateDir, "tls.crt")
)

// RouterConfig contains the configuration parameters necessary to
// launch a router, including general parameters, type of router, and
// type-specific parameters.
type RouterConfig struct {
	Action configcmd.BulkAction

	// Name is the router name, set as an argument
	Name string

	// RouterCanonicalHostname is the (optional) external host name of the router
	RouterCanonicalHostname string

	// Type is the router type, which determines which plugin to use (f5
	// or template).
	Type string

	// Subdomain is the subdomain served by this router. This may not be
	// accepted by all routers.
	Subdomain string
	// ForceSubdomain overrides the user's requested spec.host value on a
	// route and replaces it with this template. May not be used with Subdomain.
	ForceSubdomain string

	// ImageTemplate specifies the image from which the router will be created.
	ImageTemplate variable.ImageTemplate

	// Ports specifies the container ports for the router.
	Ports string

	// Replicas specifies the initial replica count for the router.
	Replicas int32

	// Labels specifies the label or labels that will be assigned to the router
	// pod.
	Labels string

	// DryRun specifies that the router command should not launch a router but
	// should instead exit with code 1 to indicate if a router is already running
	// or code 0 otherwise.
	DryRun bool

	// SecretsAsEnv sets the credentials as env vars, instead of secrets.
	SecretsAsEnv bool

	// DefaultCertificate holds the certificate that will be used if no more
	// specific certificate is found.  This is typically a wildcard certificate.
	DefaultCertificate string

	// Selector specifies a label or set of labels that determines the nodes on
	// which the router pod can be scheduled.
	Selector string

	// StatsPort specifies a port at which the router can provide statistics.
	StatsPort int

	// StatsPassword specifies a password required to authenticate connections to
	// the statistics port.
	StatsPassword string

	// StatsUsername specifies a username required to authenticate connections to
	// the statistics port.
	StatsUsername string

	// HostNetwork specifies whether to configure the router pod to use the host's
	// network namespace or the container's.
	HostNetwork bool

	// HostPorts will expose host ports for each router port if host networking is
	// not set.
	HostPorts bool

	// ServiceAccount specifies the service account under which the router will
	// run.
	ServiceAccount string

	// ExternalHost specifies the hostname or IP address of an external host for
	// router plugins that integrate with an external load balancer (such as f5).
	ExternalHost string

	// ExternalHostUsername specifies the username for authenticating with the
	// external host.
	ExternalHostUsername string

	// ExternalHostPassword specifies the password for authenticating with the
	// external host.
	ExternalHostPassword string

	// ExternalHostHttpVserver specifies the virtual server for HTTP connections.
	ExternalHostHttpVserver string

	// ExternalHostHttpsVserver specifies the virtual server for HTTPS connections.
	ExternalHostHttpsVserver string

	// ExternalHostPrivateKey specifies an SSH private key for authenticating with
	// the external host.
	ExternalHostPrivateKey string

	// ExternalHostInternalIP specifies the IP address of the internal interface that is
	// used by the external host to connect to the pod network
	ExternalHostInternalIP string

	// ExternalHostVxLANGateway specifies the gateway IP and mask (cidr) of the IP
	// address to be used to connect to the pod network from the external host
	ExternalHostVxLANGateway string

	// ExternalHostInsecure specifies that the router should skip strict
	// certificate verification when connecting to the external host.
	ExternalHostInsecure bool

	// ExternalHostPartitionPath specifies the partition path to use.
	// This is used by some routers to create access access control
	// boundaries for users and applications.
	ExternalHostPartitionPath string

	// DisableNamespaceOwnershipCheck overrides the same namespace check
	// for different paths to a route host or for overlapping host names
	// in case of wildcard routes.
	// E.g. Setting this flag to false allows www.example.org/path1 and
	//      www.example.org/path2 to be claimed by namespaces nsone and
	//      nstwo respectively. And for wildcard routes, this allows
	//      overlapping host names (*.example.test vs foo.example.test)
	//      to be claimed by different namespaces.
	//
	// Warning: Please be aware that if namespace ownership checks are
	//          disabled, routes in a different namespace can use this
	//          mechanism to "steal" sub-paths for existing domains.
	//          This is only safe if route creation privileges are
	//          restricted, or if all the users can be trusted.
	DisableNamespaceOwnershipCheck bool

	// MaxConnections specifies the maximum number of concurrent
	// connections.
	MaxConnections string

	// Ciphers is the set of ciphers to use with bind
	// modern | intermediate | old | set of cihers
	Ciphers string

	// Strict SNI (do not use default cert)
	StrictSNI bool

	Local bool
}

const (
	defaultLabel = "router=<name>"

	// Default port numbers to expose and bind/listen on.
	defaultPorts = "80:80,443:443"

	// Default stats and healthz port.
	defaultStatsPort   = 1936
	defaultHealthzPort = defaultStatsPort
)

// NewCmdRouter implements the OpenShift CLI router command.
func NewCmdRouter(f *clientcmd.Factory, parentName, name string, out, errout io.Writer) *cobra.Command {
	cfg := &RouterConfig{
		Name:          "router",
		ImageTemplate: variable.NewDefaultImageTemplate(),

		ServiceAccount: "router",

		Labels:   defaultLabel,
		Ports:    defaultPorts,
		Replicas: 1,

		StatsUsername: "admin",
		StatsPort:     defaultStatsPort,
		HostNetwork:   true,
		HostPorts:     true,
	}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Install a router",
		Long:    routerLong,
		Example: fmt.Sprintf(routerExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunCmdRouter(f, cmd, out, errout, cfg, args)
			if err != kcmdutil.ErrExit {
				kcmdutil.CheckErr(err)
			} else {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&cfg.Type, "type", "haproxy-router", "The type of router to use - if you specify --images this flag may be ignored.")
	cmd.Flags().StringVar(&cfg.Subdomain, "subdomain", "", "The template for the route subdomain exposed by this router, used for routes that are not externally specified. E.g. '${name}-${namespace}.apps.mycompany.com'")
	cmd.Flags().StringVar(&cfg.ForceSubdomain, "force-subdomain", "", "A router path format to force on all routes used by this router (will ignore the route host value)")
	cmd.Flags().StringVar(&cfg.ImageTemplate.Format, "images", cfg.ImageTemplate.Format, "The image to base this router on - ${component} will be replaced with --type")
	cmd.Flags().BoolVar(&cfg.ImageTemplate.Latest, "latest-images", cfg.ImageTemplate.Latest, "If true, attempt to use the latest images for the router instead of the latest release.")
	cmd.Flags().StringVar(&cfg.Ports, "ports", cfg.Ports, "A comma delimited list of ports or port pairs that set the port in the router pod containerPort and hostPort. It also sets service port and targetPort to expose on the router pod. This does not modify the env variables. That can be done using oc env or by editing the router's dc. This is used when host-network=false.")
	cmd.Flags().StringVar(&cfg.RouterCanonicalHostname, "router-canonical-hostname", cfg.RouterCanonicalHostname, "CanonicalHostname is the external host name for the router that can be used as a CNAME for the host requested for this route. This value is optional and may not be set in all cases.")
	cmd.Flags().Int32Var(&cfg.Replicas, "replicas", cfg.Replicas, "The replication factor of the router; commonly 2 when high availability is desired.")
	cmd.Flags().StringVar(&cfg.Labels, "labels", cfg.Labels, "A set of labels to uniquely identify the router and its components.")
	cmd.Flags().BoolVar(&cfg.SecretsAsEnv, "secrets-as-env", cfg.SecretsAsEnv, "If true, use environment variables for master secrets.")
	cmd.Flags().Bool("create", false, "deprecated; this is now the default behavior")
	cmd.Flags().StringVar(&cfg.DefaultCertificate, "default-cert", cfg.DefaultCertificate, "Optional path to a certificate file that be used as the default certificate.  The file should contain the cert, key, and any CA certs necessary for the router to serve the certificate. Does not apply to external appliance based routers (e.g. F5)")
	cmd.Flags().StringVar(&cfg.Selector, "selector", cfg.Selector, "Selector used to filter nodes on deployment. Used to run routers on a specific set of nodes.")
	cmd.Flags().StringVar(&cfg.ServiceAccount, "service-account", cfg.ServiceAccount, "Name of the service account to use to run the router pod.")
	cmd.Flags().IntVar(&cfg.StatsPort, "stats-port", cfg.StatsPort, "If the underlying router implementation can provide statistics this is a hint to expose it on this port. Specify 0 if you want to turn off exposing the statistics.")
	cmd.Flags().StringVar(&cfg.StatsPassword, "stats-password", cfg.StatsPassword, "If the underlying router implementation can provide statistics this is the requested password for auth.  If not set a password will be generated. Not available for external appliance based routers (e.g. F5)")
	cmd.Flags().StringVar(&cfg.StatsUsername, "stats-user", cfg.StatsUsername, "If the underlying router implementation can provide statistics this is the requested username for auth. Not available for external appliance based routers (e.g. F5)")
	cmd.Flags().BoolVar(&cfg.HostNetwork, "host-network", cfg.HostNetwork, "If true (the default), then use host networking rather than using a separate container network stack. Not required for external appliance based routers (e.g. F5)")
	cmd.Flags().BoolVar(&cfg.HostPorts, "host-ports", cfg.HostPorts, "If true (the default), when not using host networking host ports will be exposed. Not required for external appliance based routers (e.g. F5)")
	cmd.Flags().StringVar(&cfg.ExternalHost, "external-host", cfg.ExternalHost, "If the underlying router implementation connects with an external host, this is the external host's hostname.")
	cmd.Flags().StringVar(&cfg.ExternalHostUsername, "external-host-username", cfg.ExternalHostUsername, "If the underlying router implementation connects with an external host, this is the username for authenticating with the external host.")
	cmd.Flags().StringVar(&cfg.ExternalHostPassword, "external-host-password", cfg.ExternalHostPassword, "If the underlying router implementation connects with an external host, this is the password for authenticating with the external host.")
	cmd.Flags().StringVar(&cfg.ExternalHostHttpVserver, "external-host-http-vserver", cfg.ExternalHostHttpVserver, "If the underlying router implementation uses virtual servers, this is the name of the virtual server for HTTP connections.")
	cmd.Flags().StringVar(&cfg.ExternalHostHttpsVserver, "external-host-https-vserver", cfg.ExternalHostHttpsVserver, "If the underlying router implementation uses virtual servers, this is the name of the virtual server for HTTPS connections.")
	cmd.Flags().StringVar(&cfg.ExternalHostPrivateKey, "external-host-private-key", cfg.ExternalHostPrivateKey, "If the underlying router implementation requires an SSH private key, this is the path to the private key file.")
	cmd.Flags().StringVar(&cfg.ExternalHostInternalIP, "external-host-internal-ip", cfg.ExternalHostInternalIP, "If the underlying router implementation requires the use of a specific network interface to connect to the pod network, this is the IP address of that internal interface.")
	cmd.Flags().StringVar(&cfg.ExternalHostVxLANGateway, "external-host-vxlan-gw", cfg.ExternalHostVxLANGateway, "If the underlying router implementation requires VxLAN access to the pod network, this is the gateway address that should be used in cidr format.")
	cmd.Flags().BoolVar(&cfg.ExternalHostInsecure, "external-host-insecure", cfg.ExternalHostInsecure, "If the underlying router implementation connects with an external host over a secure connection, this causes the router to skip strict certificate verification with the external host.")
	cmd.Flags().StringVar(&cfg.ExternalHostPartitionPath, "external-host-partition-path", cfg.ExternalHostPartitionPath, "If the underlying router implementation uses partitions for control boundaries, this is the path to use for that partition.")
	cmd.Flags().BoolVar(&cfg.DisableNamespaceOwnershipCheck, "disable-namespace-ownership-check", cfg.DisableNamespaceOwnershipCheck, "Disables the namespace ownership check and allows different namespaces to claim either different paths to a route host or overlapping host names in case of a wildcard route. The default behavior (false) to restrict claims to the oldest namespace that has claimed either the host or the subdomain. Please be aware that if namespace ownership checks are disabled, routes in a different namespace can use this mechanism to 'steal' sub-paths for existing domains. This is only safe if route creation privileges are restricted, or if all the users can be trusted.")
	cmd.Flags().StringVar(&cfg.MaxConnections, "max-connections", cfg.MaxConnections, "Specifies the maximum number of concurrent connections. Not supported for F5.")
	cmd.Flags().StringVar(&cfg.Ciphers, "ciphers", cfg.Ciphers, "Specifies the cipher suites to use. You can choose a predefined cipher set ('modern', 'intermediate', or 'old') or specify exact cipher suites by passing a : separated list. Not supported for F5.")
	cmd.Flags().BoolVar(&cfg.StrictSNI, "strict-sni", cfg.StrictSNI, "Use strict-sni bind processing (do not use default cert). Not supported for F5.")
	cmd.Flags().BoolVar(&cfg.Local, "local", cfg.Local, "If true, do not contact the apiserver")

	cfg.Action.BindForOutput(cmd.Flags())
	cmd.Flags().String("output-version", "", "The preferred API versions of the output objects")

	return cmd
}

// generateSecretsConfig generates any Secret and Volume objects, such
// as SSH private keys, that are necessary for the router container.
func generateSecretsConfig(cfg *RouterConfig, namespace string, defaultCert []byte, certName string) ([]*kapi.Secret, []kapi.Volume, []kapi.VolumeMount, error) {
	var secrets []*kapi.Secret
	var volumes []kapi.Volume
	var mounts []kapi.VolumeMount

	if len(cfg.ExternalHostPrivateKey) != 0 {
		privkeyData, err := fileutil.LoadData(cfg.ExternalHostPrivateKey)
		if err != nil {
			return secrets, volumes, mounts, fmt.Errorf("error reading private key for external host: %v", err)
		}

		secret := &kapi.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: privkeySecretName,
			},
			Data: map[string][]byte{privkeyName: privkeyData},
		}
		secrets = append(secrets, secret)

		volume := kapi.Volume{
			Name: secretsVolumeName,
			VolumeSource: kapi.VolumeSource{
				Secret: &kapi.SecretVolumeSource{
					SecretName: privkeySecretName,
				},
			},
		}
		volumes = append(volumes, volume)

		mount := kapi.VolumeMount{
			Name:      secretsVolumeName,
			ReadOnly:  true,
			MountPath: secretsPath,
		}
		mounts = append(mounts, mount)
	}

	if len(defaultCert) > 0 {
		// When the user sets the default cert from the "oc adm router --default-cert ..."
		// command we end up here. In this case the default cert must be in pem format.
		// The secret has a crt and key. The crt contains the supplied default cert (pem)
		// and the key is extracted from the default cert but its ultimately not used.
		// NOTE: If the default cert is not provided by the user, we generate one by
		// adding an annotation to the service associated with the router (see RunCmdRouter())
		keys, err := cmdutil.PrivateKeysFromPEM(defaultCert)
		if err != nil {
			return nil, nil, nil, err
		}
		if len(keys) == 0 {
			return nil, nil, nil, fmt.Errorf("the default cert must contain a private key")
		}
		// The TLSCertKey contains the pem file passed in as the default cert
		secret := &kapi.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: certName,
			},
			Type: kapi.SecretTypeTLS,
			Data: map[string][]byte{
				kapi.TLSCertKey:       defaultCert,
				kapi.TLSPrivateKeyKey: keys,
			},
		}
		secrets = append(secrets, secret)
	}

	if cfg.Type == "haproxy-router" && cfg.StatsPort != 0 {
		metricsCertName := "router-metrics-tls"
		if len(defaultCert) == 0 {
			// when we are generating a serving cert, we need to reuse the existing cert
			metricsCertName = certName
		}
		volumes = append(volumes, kapi.Volume{
			Name: "metrics-server-certificate",
			VolumeSource: kapi.VolumeSource{
				Secret: &kapi.SecretVolumeSource{
					SecretName: metricsCertName,
				},
			},
		})
		mounts = append(mounts, kapi.VolumeMount{
			Name:      "metrics-server-certificate",
			ReadOnly:  true,
			MountPath: "/etc/pki/tls/metrics/",
		})
	}

	// The secret in this volume is either the one created for the
	// user supplied default cert (pem format) or the secret generated
	// by the service anotation (cert only format).
	// In either case the secret has the same name and it has the same mount point.
	volume := kapi.Volume{
		Name: "server-certificate",
		VolumeSource: kapi.VolumeSource{
			Secret: &kapi.SecretVolumeSource{
				SecretName: certName,
			},
		},
	}
	volumes = append(volumes, volume)

	mount := kapi.VolumeMount{
		Name:      volume.Name,
		ReadOnly:  true,
		MountPath: defaultCertificateDir,
	}
	mounts = append(mounts, mount)

	return secrets, volumes, mounts, nil
}

func generateProbeConfigForRouter(cfg *RouterConfig, ports []kapi.ContainerPort) *kapi.Probe {
	var probe *kapi.Probe

	if cfg.Type == "haproxy-router" {
		probe = &kapi.Probe{}
		healthzPort := defaultHealthzPort
		if cfg.StatsPort > 0 {
			healthzPort = cfg.StatsPort
		}

		probe.Handler.HTTPGet = &kapi.HTTPGetAction{
			Path: "/healthz",
			Port: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: int32(healthzPort),
			},
		}

		// Workaround for misconfigured environments where the Node's InternalIP is
		// physically present on the Node.  In those environments the probes will
		// fail unless a host firewall port is opened
		if cfg.HostNetwork {
			probe.Handler.HTTPGet.Host = "localhost"
		}
	}

	return probe
}

func generateLivenessProbeConfig(cfg *RouterConfig, ports []kapi.ContainerPort) *kapi.Probe {
	probe := generateProbeConfigForRouter(cfg, ports)
	if probe != nil {
		probe.InitialDelaySeconds = 10
	}
	return probe
}

func generateReadinessProbeConfig(cfg *RouterConfig, ports []kapi.ContainerPort) *kapi.Probe {
	probe := generateProbeConfigForRouter(cfg, ports)
	if probe != nil {
		probe.InitialDelaySeconds = 10
	}
	return probe
}

// RunCmdRouter contains all the necessary functionality for the
// OpenShift CLI router command.
func RunCmdRouter(f *clientcmd.Factory, cmd *cobra.Command, out, errout io.Writer, cfg *RouterConfig, args []string) error {
	switch len(args) {
	case 0:
		// uses default value
	case 1:
		cfg.Name = args[0]
	default:
		return kcmdutil.UsageErrorf(cmd, "You may pass zero or one arguments to provide a name for the router")
	}
	if cfg.Local && !cfg.Action.DryRun {
		return fmt.Errorf("--local cannot be specified without --dry-run")
	}

	name := cfg.Name

	var defaultOutputErr error

	if len(cfg.StatsUsername) > 0 {
		if strings.Contains(cfg.StatsUsername, ":") {
			return kcmdutil.UsageErrorf(cmd, "username %s must not contain ':'", cfg.StatsUsername)
		}
	}

	if len(cfg.Subdomain) > 0 && len(cfg.ForceSubdomain) > 0 {
		return kcmdutil.UsageErrorf(cmd, "only one of --subdomain, --force-subdomain can be specified")
	}

	ports, err := app.ContainerPortsFromString(cfg.Ports)
	if err != nil {
		return fmt.Errorf("unable to parse --ports: %v", err)
	}

	// HostNetwork overrides HostPorts
	if cfg.HostNetwork {
		cfg.HostPorts = false
	}

	// For the host networking case, ensure the ports match.
	if cfg.HostNetwork {
		for i := 0; i < len(ports); i++ {
			if ports[i].HostPort != 0 && ports[i].ContainerPort != ports[i].HostPort {
				return fmt.Errorf("when using host networking mode, container port %d and host port %d must be equal", ports[i].ContainerPort, ports[i].HostPort)
			}
		}
	}

	if cfg.StatsPort > 0 {
		port := kapi.ContainerPort{
			Name:          "stats",
			ContainerPort: int32(cfg.StatsPort),
			Protocol:      kapi.ProtocolTCP,
		}
		if cfg.HostPorts {
			port.HostPort = int32(cfg.StatsPort)
		}
		ports = append(ports, port)
	}

	label := map[string]string{"router": name}
	if cfg.Labels != defaultLabel {
		valid, remove, err := app.LabelsFromSpec(strings.Split(cfg.Labels, ","))
		if err != nil {
			glog.Fatal(err)
		}
		if len(remove) > 0 {
			return kcmdutil.UsageErrorf(cmd, "You may not pass negative labels in %q", cfg.Labels)
		}
		label = valid
	}

	nodeSelector := map[string]string{}
	if len(cfg.Selector) > 0 {
		valid, remove, err := app.LabelsFromSpec(strings.Split(cfg.Selector, ","))
		if err != nil {
			glog.Fatal(err)
		}
		if len(remove) > 0 {
			return kcmdutil.UsageErrorf(cmd, "You may not pass negative labels in selector %q", cfg.Selector)
		}
		nodeSelector = valid
	}

	image := cfg.ImageTemplate.ExpandOrDie(cfg.Type)

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}

	cfg.Action.Bulk.Mapper = clientcmd.ResourceMapper(f)
	cfg.Action.Out, cfg.Action.ErrOut = out, errout
	cfg.Action.Bulk.Op = configcmd.Create

	var clusterIP string

	output := cfg.Action.ShouldPrint()
	generate := output
	if !cfg.Local {
		kClient, err := f.ClientSet()
		if err != nil {
			return fmt.Errorf("error getting client: %v", err)
		}
		service, err := kClient.Core().Services(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if !generate {
				if !errors.IsNotFound(err) {
					return fmt.Errorf("can't check for existing router %q: %v", name, err)
				}
				if !output && cfg.Action.DryRun {
					return fmt.Errorf("Router %q service does not exist", name)
				}
				generate = true
			}
		} else {
			clusterIP = service.Spec.ClusterIP
		}
	}

	if !generate {
		fmt.Fprintf(out, "Router %q service exists\n", name)
		return nil
	}

	if len(cfg.ServiceAccount) == 0 {
		return fmt.Errorf("you must specify a service account for the router with --service-account")
	}

	if !cfg.Local {
		clientConfig, err := f.ClientConfig()
		if err != nil {
			return err
		}
		securityClient, err := securityclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return err
		}
		if err := validateServiceAccount(securityClient, namespace, cfg.ServiceAccount, cfg.HostNetwork, cfg.HostPorts); err != nil {
			err = fmt.Errorf("router could not be created; %v", err)
			if !cfg.Action.ShouldPrint() {
				return err
			}
			fmt.Fprintf(errout, "error: %v\n", err)
			defaultOutputErr = kcmdutil.ErrExit
		}
	}

	// create new router
	secretEnv := app.Environment{}

	defaultCert, err := fileutil.LoadData(cfg.DefaultCertificate)
	if err != nil {
		return fmt.Errorf("router could not be created; error reading default certificate file: %v", err)
	}

	if len(cfg.StatsPassword) == 0 {
		cfg.StatsPassword = generateStatsPassword()
		if !cfg.Action.ShouldPrint() {
			fmt.Fprintf(errout, "info: password for stats user %s has been set to %s\n", cfg.StatsUsername, cfg.StatsPassword)
		}
	}

	env := app.Environment{
		"ROUTER_SUBDOMAIN":                      cfg.Subdomain,
		"ROUTER_SERVICE_NAME":                   name,
		"ROUTER_SERVICE_NAMESPACE":              namespace,
		"ROUTER_SERVICE_HTTP_PORT":              "80",
		"ROUTER_SERVICE_HTTPS_PORT":             "443",
		"ROUTER_EXTERNAL_HOST_HOSTNAME":         cfg.ExternalHost,
		"ROUTER_EXTERNAL_HOST_USERNAME":         cfg.ExternalHostUsername,
		"ROUTER_EXTERNAL_HOST_PASSWORD":         cfg.ExternalHostPassword,
		"ROUTER_EXTERNAL_HOST_HTTP_VSERVER":     cfg.ExternalHostHttpVserver,
		"ROUTER_EXTERNAL_HOST_HTTPS_VSERVER":    cfg.ExternalHostHttpsVserver,
		"ROUTER_EXTERNAL_HOST_INSECURE":         strconv.FormatBool(cfg.ExternalHostInsecure),
		"ROUTER_EXTERNAL_HOST_PARTITION_PATH":   cfg.ExternalHostPartitionPath,
		"ROUTER_EXTERNAL_HOST_PRIVKEY":          privkeyPath,
		"ROUTER_EXTERNAL_HOST_INTERNAL_ADDRESS": cfg.ExternalHostInternalIP,
		"ROUTER_EXTERNAL_HOST_VXLAN_GW_CIDR":    cfg.ExternalHostVxLANGateway,
		"ROUTER_CIPHERS":                        cfg.Ciphers,
		"STATS_PORT":                            strconv.Itoa(cfg.StatsPort),
		"STATS_USERNAME":                        cfg.StatsUsername,
		"STATS_PASSWORD":                        cfg.StatsPassword,
	}
	if len(cfg.MaxConnections) > 0 {
		env["ROUTER_MAX_CONNECTIONS"] = cfg.MaxConnections
	}
	if len(cfg.ForceSubdomain) > 0 {
		env["ROUTER_SUBDOMAIN"] = cfg.ForceSubdomain
		env["ROUTER_OVERRIDE_HOSTNAME"] = "true"
	}
	if cfg.DisableNamespaceOwnershipCheck {
		env["ROUTER_DISABLE_NAMESPACE_OWNERSHIP_CHECK"] = "true"
	}
	if cfg.StrictSNI {
		env["ROUTER_STRICT_SNI"] = "true"
	}
	if len(cfg.RouterCanonicalHostname) > 0 {
		if errs := validation.IsDNS1123Subdomain(cfg.RouterCanonicalHostname); len(errs) != 0 {
			return fmt.Errorf("invalid canonical hostname (RFC 1123): %s", cfg.RouterCanonicalHostname)
		}
		if errs := validation.IsValidIP(cfg.RouterCanonicalHostname); len(errs) == 0 {
			return fmt.Errorf("canonical hostname must not be an IP address: %s", cfg.RouterCanonicalHostname)
		}
		env["ROUTER_CANONICAL_HOSTNAME"] = cfg.RouterCanonicalHostname
	}
	// automatically start the internal metrics agent if we are handling a known type
	if cfg.Type == "haproxy-router" && cfg.StatsPort != 0 {
		env["ROUTER_LISTEN_ADDR"] = fmt.Sprintf("0.0.0.0:%d", cfg.StatsPort)
		env["ROUTER_METRICS_TYPE"] = "haproxy"
		env["ROUTER_METRICS_TLS_CERT_FILE"] = "/etc/pki/tls/metrics/tls.crt"
		env["ROUTER_METRICS_TLS_KEY_FILE"] = "/etc/pki/tls/metrics/tls.key"
	}
	env.Add(secretEnv)
	if len(defaultCert) > 0 {
		if cfg.SecretsAsEnv {
			env.Add(app.Environment{"DEFAULT_CERTIFICATE": string(defaultCert)})
		} else {
			env.Add(app.Environment{"DEFAULT_CERTIFICATE_PATH": defaultCertificatePath})
		}
	}
	env.Add(app.Environment{"DEFAULT_CERTIFICATE_DIR": defaultCertificateDir})
	var certName = fmt.Sprintf("%s-certs", cfg.Name)
	secrets, volumes, mounts, err := generateSecretsConfig(cfg, namespace, defaultCert, certName)
	if err != nil {
		return fmt.Errorf("router could not be created: %v", err)
	}

	livenessProbe := generateLivenessProbeConfig(cfg, ports)
	readinessProbe := generateReadinessProbeConfig(cfg, ports)

	exposedPorts := make([]kapi.ContainerPort, len(ports))
	copy(exposedPorts, ports)
	if !cfg.HostPorts {
		for i := range exposedPorts {
			exposedPorts[i].HostPort = 0
		}
	}
	containers := []kapi.Container{
		{
			Name:            "router",
			Image:           image,
			Ports:           exposedPorts,
			Env:             env.List(),
			LivenessProbe:   livenessProbe,
			ReadinessProbe:  readinessProbe,
			ImagePullPolicy: kapi.PullIfNotPresent,
			VolumeMounts:    mounts,
			Resources: kapi.ResourceRequirements{
				Requests: kapi.ResourceList{
					kapi.ResourceCPU:    resource.MustParse("100m"),
					kapi.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
	}

	objects := []runtime.Object{}
	for _, s := range secrets {
		objects = append(objects, s)
	}

	objects = append(objects,
		&kapi.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: cfg.ServiceAccount}},
		&authapi.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: generateRoleBindingName(cfg.Name)},
			Subjects: []kapi.ObjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      cfg.ServiceAccount,
					Namespace: namespace,
				},
			},
			RoleRef: kapi.ObjectReference{
				Kind: "ClusterRole",
				Name: "system:router",
			},
		},
	)

	objects = append(objects, &appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: label,
		},
		Spec: appsapi.DeploymentConfigSpec{
			Strategy: appsapi.DeploymentStrategy{
				Type:          appsapi.DeploymentStrategyTypeRolling,
				RollingParams: &appsapi.RollingDeploymentStrategyParams{MaxUnavailable: intstr.FromString("25%")},
			},
			Replicas: cfg.Replicas,
			Selector: label,
			Triggers: []appsapi.DeploymentTriggerPolicy{
				{Type: appsapi.DeploymentTriggerOnConfigChange},
			},
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: label},
				Spec: kapi.PodSpec{
					SecurityContext: &kapi.PodSecurityContext{
						HostNetwork: cfg.HostNetwork,
					},
					ServiceAccountName: cfg.ServiceAccount,
					NodeSelector:       nodeSelector,
					Containers:         containers,
					Volumes:            volumes,
				},
			},
		},
	})

	objects = app.AddServices(objects, false)
	// set the service port to the provided output port value
	for i := range objects {
		switch t := objects[i].(type) {
		case *kapi.Service:
			if t.Annotations == nil {
				t.Annotations = make(map[string]string)
			}
			t.Annotations["prometheus.io/scrape"] = "true"
			t.Annotations["prometheus.io/port"] = "1936"
			t.Annotations["prometheus.openshift.io/username"] = cfg.StatsUsername
			t.Annotations["prometheus.openshift.io/password"] = cfg.StatsPassword
			t.Spec.ClusterIP = clusterIP
			for j, servicePort := range t.Spec.Ports {
				for _, targetPort := range ports {
					if targetPort.ContainerPort == servicePort.Port && targetPort.HostPort != 0 {
						t.Spec.Ports[j].Port = targetPort.HostPort
					}
				}
			}
			if len(defaultCert) == 0 {
				// When a user does not provide the default cert (pem), create one via a Service annotation
				// The secret generated by the service annotaion contains a tls.crt and tls.key
				// which ultimately need to be combined into a pem
				t.Annotations["service.alpha.openshift.io/serving-cert-secret-name"] = certName
			} else if cfg.Type == "haproxy-router" && cfg.StatsPort != 0 {
				// Generate a serving cert for metrics only
				t.Annotations["service.alpha.openshift.io/serving-cert-secret-name"] = "router-metrics-tls"
			}
		}
	}
	// TODO: label all created objects with the same label - router=<name>
	list := &kapi.List{Items: objects}

	if cfg.Action.ShouldPrint() {
		fn := print.VersionedPrintObject(legacyscheme.Scheme, legacyscheme.Registry, kcmdutil.PrintObject, cmd, out)
		if err := fn(list); err != nil {
			return fmt.Errorf("unable to print object: %v", err)
		}
		return defaultOutputErr
	}

	levelPrefixFilter := func(e error) string {
		// Avoid failing when service accounts or role bindings already exist.
		if ignoreError(e, cfg.ServiceAccount, generateRoleBindingName(cfg.Name)) {
			return "warning"
		}
		return "error"
	}

	cfg.Action.Bulk.IgnoreError = func(e error) bool {
		return levelPrefixFilter(e) == "warning"
	}

	if errs := cfg.Action.WithMessageAndPrefix(fmt.Sprintf("Creating router %s", cfg.Name), "created", levelPrefixFilter).Run(list, namespace); len(errs) > 0 {
		return kcmdutil.ErrExit
	}
	return nil
}

// ignoreError will return true if the error is an already exists status error and
// 1. it is for a cluster role binding named roleBindingName, or
// 2. it is for a service account name saName
func ignoreError(e error, saName string, roleBindingName string) bool {
	if !errors.IsAlreadyExists(e) {
		return false
	}
	statusError, ok := e.(*errors.StatusError)
	if !ok {
		return false
	}
	details := statusError.Status().Details
	if details == nil {
		return false
	}
	return (details.Kind == "serviceaccounts" && details.Name == saName) ||
		(details.Kind == "clusterrolebinding" /*pre-3.7*/ && details.Name == roleBindingName) ||
		(details.Kind == "clusterrolebindings" /*3.7+*/ && details.Name == roleBindingName)
}

// generateRoleBindingName generates a name for the rolebinding object if it is
// being created.
func generateRoleBindingName(name string) string {
	return fmt.Sprintf("router-%s-role", name)
}

// generateStatsPassword creates a random password.
func generateStatsPassword() string {
	rand := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	allowableChars := []rune("abcdefghijlkmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	allowableCharLength := len(allowableChars)
	password := []string{}
	for i := 0; i < 10; i++ {
		char := allowableChars[rand.Intn(allowableCharLength)]
		password = append(password, string(char))
	}
	return strings.Join(password, "")
}

func validateServiceAccount(client securityclientinternal.Interface, ns string, serviceAccount string, hostNetwork, hostPorts bool) error {
	if !hostNetwork && !hostPorts {
		return nil
	}
	// get cluster sccs
	sccList, err := client.Security().SecurityContextConstraints().List(metav1.ListOptions{})
	if err != nil {
		if !errors.IsUnauthorized(err) {
			return fmt.Errorf("could not retrieve list of security constraints to verify service account %q: %v", serviceAccount, err)
		}
		return nil
	}

	// get set of sccs applicable to the service account
	userInfo := serviceaccount.UserInfo(ns, serviceAccount, "")
	for _, scc := range sccList.Items {
		if oscc.ConstraintAppliesTo(&scc, userInfo, "", nil) {
			switch {
			case hostPorts && scc.AllowHostPorts:
				return nil
			case hostNetwork && scc.AllowHostNetwork:
				return nil
			}
		}
	}

	if hostNetwork {
		errMsg := "service account %q is not allowed to access the host network on nodes, grant access with oc adm policy add-scc-to-user %s -z %s"
		return fmt.Errorf(errMsg, serviceAccount, bootstrappolicy.SecurityContextConstraintsHostNetwork, serviceAccount)
	}
	if hostPorts {
		errMsg := "service account %q is not allowed to access host ports on nodes, grant access with oc adm policy add-scc-to-user %s -z %s"
		return fmt.Errorf(errMsg, serviceAccount, bootstrappolicy.SecurityContextConstraintsHostNetwork, serviceAccount)
	}
	return nil
}
