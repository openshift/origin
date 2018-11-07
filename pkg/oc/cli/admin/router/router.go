package router

import (
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/printers"
	"k8s.io/kubernetes/pkg/serviceaccount"

	appsv1 "github.com/openshift/api/apps/v1"
	authv1 "github.com/openshift/api/authorization/v1"
	securityv1 "github.com/openshift/api/security/v1"
	securityv1typedclient "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
	"github.com/openshift/origin/pkg/bulk"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/lib/newapp/app"
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

	defaultMutualTLSAuth = "none"
	clientCertConfigDir  = "/etc/pki/tls/client-certs"
	clientCertConfigCA   = "ca.pem"
	clientCertConfigCRL  = "crl.pem"

	defaultCertificatePath = path.Join(defaultCertificateDir, "tls.crt")
)

// RouterOptions contains the configuration parameters necessary to
// launch a router, including general parameters, type of router, and
// type-specific parameters.
type RouterOptions struct {
	PrintFlags *genericclioptions.PrintFlags
	Printer    printers.ResourcePrinter

	Action bulk.BulkAction

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

	// ExtendedLogging specifies whether to inject a sidecar container
	// running rsyslogd into the router pod and configure the router to send
	// access logs to that sidecar.
	ExtendedLogging bool

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

	// Number of threads to start per process
	Threads int32

	Local bool

	// MutualTLSAuth controls access to the router using a mutually agreed
	// upon TLS authentication mechanism (example client certificates).
	// One of: required | optional | none  - the default is none.
	MutualTLSAuth string

	// MutualTLSAuthCA contains the CA certificates that will be used
	// to verify a client's certificate.
	MutualTLSAuthCA string

	// MutualTLSAuthCRL contains the certificate revocation list used to
	// verify a client's certificate.
	MutualTLSAuthCRL string

	// MutualTLSAuthFilter contains the value to filter requests based on
	// a client certificate subject field substring match.
	MutualTLSAuthFilter string

	CoreClient     corev1client.CoreV1Interface
	SecurityClient securityv1typedclient.SecurityV1Interface

	Namespace string
	Output    bool

	genericclioptions.IOStreams
}

const (
	defaultLabel = "router=<name>"

	// Default port numbers to expose and bind/listen on.
	defaultPorts = "80:80,443:443"

	// Default stats port.
	defaultStatsPort = 1936

	rsyslogConfigurationFile = `$ModLoad imuxsock
$SystemLogSocketName /var/lib/rsyslog/rsyslog.sock
$ModLoad omstdout.so
*.* :omstdout:
`
)

func NewRouterOptions(streams genericclioptions.IOStreams) *RouterOptions {
	return &RouterOptions{
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),

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

		MutualTLSAuth: defaultMutualTLSAuth,

		IOStreams: streams,
	}
}

// NewCmdRouter implements the OpenShift CLI router command.
func NewCmdRouter(f kcmdutil.Factory, parentName, name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRouterOptions(streams)
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Install a router",
		Long:    routerLong,
		Example: fmt.Sprintf(routerExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.Type, "type", "haproxy-router", "The type of router to use - if you specify --images this flag may be ignored.")
	cmd.Flags().StringVar(&o.Subdomain, "subdomain", "", "The template for the route subdomain exposed by this router, used for routes that are not externally specified. E.g. '${name}-${namespace}.apps.mycompany.com'")
	cmd.Flags().StringVar(&o.ForceSubdomain, "force-subdomain", "", "A router path format to force on all routes used by this router (will ignore the route host value)")
	cmd.Flags().StringVar(&o.ImageTemplate.Format, "images", o.ImageTemplate.Format, "The image to base this router on - ${component} will be replaced with --type")
	cmd.Flags().BoolVar(&o.ImageTemplate.Latest, "latest-images", o.ImageTemplate.Latest, "If true, attempt to use the latest images for the router instead of the latest release.")
	cmd.Flags().StringVar(&o.Ports, "ports", o.Ports, "A comma delimited list of ports or port pairs that set the port in the router pod containerPort and hostPort. It also sets service port and targetPort to expose on the router pod. This does not modify the env variables. That can be done using oc set env or by editing the router's dc. This is used when host-network=false.")
	cmd.Flags().StringVar(&o.RouterCanonicalHostname, "router-canonical-hostname", o.RouterCanonicalHostname, "CanonicalHostname is the external host name for the router that can be used as a CNAME for the host requested for this route. This value is optional and may not be set in all cases.")
	cmd.Flags().Int32Var(&o.Replicas, "replicas", o.Replicas, "The replication factor of the router; commonly 2 when high availability is desired.")
	cmd.Flags().StringVar(&o.Labels, "labels", o.Labels, "A set of labels to uniquely identify the router and its components.")
	cmd.Flags().BoolVar(&o.SecretsAsEnv, "secrets-as-env", o.SecretsAsEnv, "If true, use environment variables for master secrets.")
	cmd.Flags().Bool("create", false, "deprecated; this is now the default behavior")
	cmd.Flags().StringVar(&o.DefaultCertificate, "default-cert", o.DefaultCertificate, "Optional path to a certificate file that be used as the default certificate.  The file should contain the cert, key, and any CA certs necessary for the router to serve the certificate. Does not apply to external appliance based routers (e.g. F5)")
	cmd.Flags().StringVar(&o.Selector, "selector", o.Selector, "Selector used to filter nodes on deployment. Used to run routers on a specific set of nodes.")
	cmd.Flags().StringVar(&o.ServiceAccount, "service-account", o.ServiceAccount, "Name of the service account to use to run the router pod.")
	cmd.Flags().IntVar(&o.StatsPort, "stats-port", o.StatsPort, "If the underlying router implementation can provide statistics this is a hint to expose it on this port. Specify 0 if you want to turn off exposing the statistics.")
	cmd.Flags().StringVar(&o.StatsPassword, "stats-password", o.StatsPassword, "If the underlying router implementation can provide statistics this is the requested password for auth.  If not set a password will be generated. Not available for external appliance based routers (e.g. F5)")
	cmd.Flags().StringVar(&o.StatsUsername, "stats-user", o.StatsUsername, "If the underlying router implementation can provide statistics this is the requested username for auth. Not available for external appliance based routers (e.g. F5)")
	cmd.Flags().BoolVar(&o.ExtendedLogging, "extended-logging", o.ExtendedLogging, "If true, then configure the router with additional logging.")
	cmd.Flags().BoolVar(&o.HostNetwork, "host-network", o.HostNetwork, "If true (the default), then use host networking rather than using a separate container network stack. Not required for external appliance based routers (e.g. F5)")
	cmd.Flags().BoolVar(&o.HostPorts, "host-ports", o.HostPorts, "If true (the default), when not using host networking host ports will be exposed. Not required for external appliance based routers (e.g. F5)")
	cmd.Flags().StringVar(&o.ExternalHost, "external-host", o.ExternalHost, "If the underlying router implementation connects with an external host, this is the external host's hostname.")
	cmd.Flags().StringVar(&o.ExternalHostUsername, "external-host-username", o.ExternalHostUsername, "If the underlying router implementation connects with an external host, this is the username for authenticating with the external host.")
	cmd.Flags().StringVar(&o.ExternalHostPassword, "external-host-password", o.ExternalHostPassword, "If the underlying router implementation connects with an external host, this is the password for authenticating with the external host.")
	cmd.Flags().StringVar(&o.ExternalHostHttpVserver, "external-host-http-vserver", o.ExternalHostHttpVserver, "If the underlying router implementation uses virtual servers, this is the name of the virtual server for HTTP connections.")
	cmd.Flags().StringVar(&o.ExternalHostHttpsVserver, "external-host-https-vserver", o.ExternalHostHttpsVserver, "If the underlying router implementation uses virtual servers, this is the name of the virtual server for HTTPS connections.")
	cmd.Flags().StringVar(&o.ExternalHostPrivateKey, "external-host-private-key", o.ExternalHostPrivateKey, "If the underlying router implementation requires an SSH private key, this is the path to the private key file.")
	cmd.Flags().StringVar(&o.ExternalHostInternalIP, "external-host-internal-ip", o.ExternalHostInternalIP, "If the underlying router implementation requires the use of a specific network interface to connect to the pod network, this is the IP address of that internal interface.")
	cmd.Flags().StringVar(&o.ExternalHostVxLANGateway, "external-host-vxlan-gw", o.ExternalHostVxLANGateway, "If the underlying router implementation requires VxLAN access to the pod network, this is the gateway address that should be used in cidr format.")
	cmd.Flags().BoolVar(&o.ExternalHostInsecure, "external-host-insecure", o.ExternalHostInsecure, "If the underlying router implementation connects with an external host over a secure connection, this causes the router to skip strict certificate verification with the external host.")
	cmd.Flags().StringVar(&o.ExternalHostPartitionPath, "external-host-partition-path", o.ExternalHostPartitionPath, "If the underlying router implementation uses partitions for control boundaries, this is the path to use for that partition.")
	cmd.Flags().BoolVar(&o.DisableNamespaceOwnershipCheck, "disable-namespace-ownership-check", o.DisableNamespaceOwnershipCheck, "Disables the namespace ownership check and allows different namespaces to claim either different paths to a route host or overlapping host names in case of a wildcard route. The default behavior (false) to restrict claims to the oldest namespace that has claimed either the host or the subdomain. Please be aware that if namespace ownership checks are disabled, routes in a different namespace can use this mechanism to 'steal' sub-paths for existing domains. This is only safe if route creation privileges are restricted, or if all the users can be trusted.")
	cmd.Flags().StringVar(&o.MaxConnections, "max-connections", o.MaxConnections, "Specifies the maximum number of concurrent connections. Not supported for F5.")
	cmd.Flags().StringVar(&o.Ciphers, "ciphers", o.Ciphers, "Specifies the cipher suites to use. You can choose a predefined cipher set ('modern', 'intermediate', or 'old') or specify exact cipher suites by passing a : separated list. Not supported for F5.")
	cmd.Flags().BoolVar(&o.StrictSNI, "strict-sni", o.StrictSNI, "Use strict-sni bind processing (do not use default cert). Not supported for F5.")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, do not contact the apiserver")
	cmd.Flags().Int32Var(&o.Threads, "threads", o.Threads, "Specifies the number of threads for the haproxy router.")

	cmd.Flags().StringVar(&o.MutualTLSAuth, "mutual-tls-auth", o.MutualTLSAuth, "Controls access to the router using mutually agreed upon TLS configuration (example client certificates). You can choose one of 'required', 'optional', or 'none'. The default is none.")
	cmd.Flags().StringVar(&o.MutualTLSAuthCA, "mutual-tls-auth-ca", o.MutualTLSAuthCA, "Optional path to a file containing one or more CA certificates used for mutual TLS authentication. The CA certificate[s] are used by the router to verify a client's certificate.")
	cmd.Flags().StringVar(&o.MutualTLSAuthCRL, "mutual-tls-auth-crl", o.MutualTLSAuthCRL, "Optional path to a file containing the certificate revocation list used for mutual TLS authentication. The certificate revocation list is used by the router to verify a client's certificate.")
	cmd.Flags().StringVar(&o.MutualTLSAuthFilter, "mutual-tls-auth-filter", o.MutualTLSAuthFilter, "Optional regular expression to filter the client certificates. If the client certificate subject field does _not_ match this regular expression, requests will be rejected by the router.")

	o.Action.BindForOutput(cmd.Flags(), "output", "template")
	cmd.Flags().String("output-version", "", "The preferred API versions of the output objects")

	o.PrintFlags.AddFlags(cmd)

	return cmd
}

// generateMutualTLSSecretName generates a mutual TLS auth secret name.
func generateMutualTLSSecretName(prefix string) string {
	return fmt.Sprintf("%s-mutual-tls-auth", prefix)
}

// generateSecretsConfig generates any Secret and Volume objects, such
// as SSH private keys, that are necessary for the router container.
func (o *RouterOptions) generateSecretsConfig(namespace, certName string, defaultCert, mtlsAuthCA, mtlsAuthCRL []byte) ([]*corev1.Secret, []corev1.Volume, []corev1.VolumeMount, error) {
	var secrets []*corev1.Secret
	var volumes []corev1.Volume
	var mounts []corev1.VolumeMount

	if len(o.ExternalHostPrivateKey) != 0 {
		privkeyData, err := fileutil.LoadData(o.ExternalHostPrivateKey)
		if err != nil {
			return secrets, volumes, mounts, fmt.Errorf("error reading private key for external host: %v", err)
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: privkeySecretName,
			},
			Data: map[string][]byte{privkeyName: privkeyData},
		}
		secrets = append(secrets, secret)

		volume := corev1.Volume{
			Name: secretsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: privkeySecretName,
				},
			},
		}
		volumes = append(volumes, volume)

		mount := corev1.VolumeMount{
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
		secret := &corev1.Secret{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{
				Name: certName,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       defaultCert,
				corev1.TLSPrivateKeyKey: keys,
			},
		}
		secrets = append(secrets, secret)
	}

	if o.Type == "haproxy-router" && o.StatsPort != 0 {
		metricsCertName := "router-metrics-tls"
		if len(defaultCert) == 0 {
			// when we are generating a serving cert, we need to reuse the existing cert
			metricsCertName = certName
		}
		volumes = append(volumes, corev1.Volume{
			Name: "metrics-server-certificate",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: metricsCertName,
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "metrics-server-certificate",
			ReadOnly:  true,
			MountPath: "/etc/pki/tls/metrics/",
		})
	}

	// The secret in this volume is either the one created for the
	// user supplied default cert (pem format) or the secret generated
	// by the service anotation (cert only format).
	// In either case the secret has the same name and it has the same mount point.
	volume := corev1.Volume{
		Name: "server-certificate",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: certName,
			},
		},
	}
	volumes = append(volumes, volume)

	mount := corev1.VolumeMount{
		Name:      volume.Name,
		ReadOnly:  true,
		MountPath: defaultCertificateDir,
	}
	mounts = append(mounts, mount)

	mtlsSecretData := map[string][]byte{}
	if len(mtlsAuthCA) > 0 {
		mtlsSecretData[clientCertConfigCA] = mtlsAuthCA
	}
	if len(mtlsAuthCRL) > 0 {
		mtlsSecretData[clientCertConfigCRL] = mtlsAuthCRL
	}

	if len(mtlsSecretData) > 0 {
		secretName := generateMutualTLSSecretName(o.Name)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			Data: mtlsSecretData,
		}
		secrets = append(secrets, secret)

		volume := corev1.Volume{
			Name: "mutual-tls-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		volumes = append(volumes, volume)

		mount := corev1.VolumeMount{
			Name:      volume.Name,
			ReadOnly:  true,
			MountPath: clientCertConfigDir,
		}
		mounts = append(mounts, mount)
	}

	return secrets, volumes, mounts, nil
}

func (o *RouterOptions) generateProbeConfigForRouter(path string, ports []corev1.ContainerPort) *corev1.Probe {
	var probe *corev1.Probe

	if o.Type == "haproxy-router" {
		probe = &corev1.Probe{}
		probePort := defaultStatsPort
		if o.StatsPort > 0 {
			probePort = o.StatsPort
		}

		probe.Handler.HTTPGet = &corev1.HTTPGetAction{
			Path: path,
			Port: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: int32(probePort),
			},
		}

		// Workaround for misconfigured environments where the Node's InternalIP is
		// physically present on the Node.  In those environments the probes will
		// fail unless a host firewall port is opened
		if o.HostNetwork {
			probe.Handler.HTTPGet.Host = "localhost"
		}
	}

	return probe
}

func (o *RouterOptions) generateLivenessProbeConfig(ports []corev1.ContainerPort) *corev1.Probe {
	probe := o.generateProbeConfigForRouter("/healthz", ports)
	if probe != nil {
		probe.InitialDelaySeconds = 10
	}
	return probe
}

func (o *RouterOptions) generateReadinessProbeConfig(ports []corev1.ContainerPort) *corev1.Probe {
	probe := o.generateProbeConfigForRouter("healthz/ready", ports)
	if probe != nil {
		probe.InitialDelaySeconds = 10
	}
	return probe
}

func (o *RouterOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 0:
		// uses default value
	case 1:
		o.Name = args[0]
	default:
		return fmt.Errorf("you may pass zero or one arguments to provide a name for the router")
	}
	// HostNetwork overrides HostPorts
	if o.HostNetwork {
		o.HostPorts = false
	}
	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	restMapper, err := f.ToRESTMapper()
	if err != nil {
		return err
	}
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.Action.Bulk.Scheme = legacyscheme.Scheme
	o.Action.Out, o.Action.ErrOut = o.Out, o.ErrOut
	o.Action.Bulk.Op = bulk.Creator{
		Client:     dynamicClient,
		RESTMapper: restMapper,
	}.Create

	o.CoreClient, err = corev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.SecurityClient, err = securityv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	if !o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	output := kcmdutil.GetFlagString(cmd, "output")
	o.Output = output != "" && output != "name" && output != "compact"

	return nil
}

func (o *RouterOptions) Validate() error {
	if o.Local && !o.Action.DryRun {
		return fmt.Errorf("--local cannot be specified without --dry-run")
	}
	if len(o.StatsUsername) > 0 {
		if strings.Contains(o.StatsUsername, ":") {
			return fmt.Errorf("username %s must not contain ':'", o.StatsUsername)
		}
	}
	if len(o.Subdomain) > 0 && len(o.ForceSubdomain) > 0 {
		return fmt.Errorf("only one of --subdomain, --force-subdomain can be specified")
	}

	return nil
}

func (o *RouterOptions) Run() error {
	var defaultOutputErr error

	ports, err := app.ContainerPortsFromString(o.Ports)
	if err != nil {
		return fmt.Errorf("unable to parse --ports: %v", err)
	}

	// For the host networking case, ensure the ports match.
	if o.HostNetwork {
		for i := 0; i < len(ports); i++ {
			if ports[i].HostPort != 0 && ports[i].ContainerPort != ports[i].HostPort {
				return fmt.Errorf("when using host networking mode, container port %d and host port %d must be equal", ports[i].ContainerPort, ports[i].HostPort)
			}
		}
	}

	if o.StatsPort > 0 {
		port := corev1.ContainerPort{
			Name:          "stats",
			ContainerPort: int32(o.StatsPort),
			Protocol:      corev1.ProtocolTCP,
		}
		if o.HostPorts {
			port.HostPort = int32(o.StatsPort)
		}
		ports = append(ports, port)
	}

	label := map[string]string{"router": o.Name}
	if o.Labels != defaultLabel {
		valid, remove, err := app.LabelsFromSpec(strings.Split(o.Labels, ","))
		if err != nil {
			glog.Fatal(err)
		}
		if len(remove) > 0 {
			return fmt.Errorf("you may not pass negative labels in %q", o.Labels)
		}
		label = valid
	}

	nodeSelector := map[string]string{}
	if len(o.Selector) > 0 {
		valid, remove, err := app.LabelsFromSpec(strings.Split(o.Selector, ","))
		if err != nil {
			glog.Fatal(err)
		}
		if len(remove) > 0 {
			return fmt.Errorf("you may not pass negative labels in selector %q", o.Selector)
		}
		nodeSelector = valid
	}

	image := o.ImageTemplate.ExpandOrDie(o.Type)

	var clusterIP string

	generate := o.Output
	if !o.Local {
		if len(o.MutualTLSAuthCA) > 0 || len(o.MutualTLSAuthCRL) > 0 {
			secretName := generateMutualTLSSecretName(o.Name)
			if _, err := o.CoreClient.Secrets(o.Namespace).Get(secretName, metav1.GetOptions{}); err == nil {
				return fmt.Errorf("router could not be created: mutual tls secret %q already exists", secretName)
			}
		}

		service, err := o.CoreClient.Services(o.Namespace).Get(o.Name, metav1.GetOptions{})
		if err != nil {
			if !generate {
				if !errors.IsNotFound(err) {
					return fmt.Errorf("can't check for existing router %q: %v", o.Name, err)
				}
				if !o.Output && o.Action.DryRun {
					return fmt.Errorf("router %q service does not exist", o.Name)
				}
				generate = true
			}
		} else {
			clusterIP = service.Spec.ClusterIP
		}
	}

	if !generate {
		fmt.Fprintf(o.Out, "Router %q service exists\n", o.Name)
		return nil
	}

	if len(o.ServiceAccount) == 0 {
		return fmt.Errorf("you must specify a service account for the router with --service-account")
	}

	if !o.Local {
		if err := validateServiceAccount(o.SecurityClient, o.Namespace, o.ServiceAccount, o.HostNetwork, o.HostPorts); err != nil {
			err = fmt.Errorf("router could not be created; %v", err)
			if !o.Output {
				return err
			}
			fmt.Fprintf(o.ErrOut, "error: %v\n", err)
			defaultOutputErr = kcmdutil.ErrExit
		}
	}

	// create new router
	secretEnv := app.Environment{}

	defaultCert, err := fileutil.LoadData(o.DefaultCertificate)
	if err != nil {
		return fmt.Errorf("router could not be created; error reading default certificate file: %v", err)
	}

	mtlsAuthOptions := []string{"required", "optional", "none"}
	allowedMutualTLSAuthOptions := sets.NewString(mtlsAuthOptions...)
	if !allowedMutualTLSAuthOptions.Has(o.MutualTLSAuth) {
		return fmt.Errorf("invalid mutual tls auth option %v, expected one of %v", o.MutualTLSAuth, mtlsAuthOptions)
	}
	mtlsAuthCA, err := fileutil.LoadData(o.MutualTLSAuthCA)
	if err != nil {
		return fmt.Errorf("reading ca certificates for mutual tls auth: %v", err)
	}
	mtlsAuthCRL, err := fileutil.LoadData(o.MutualTLSAuthCRL)
	if err != nil {
		return fmt.Errorf("reading certificate revocation list for mutual tls auth: %v", err)
	}

	if len(o.StatsPassword) == 0 {
		o.StatsPassword = generateStatsPassword()
		if !o.Output {
			fmt.Fprintf(o.ErrOut, "info: password for stats user %s has been set to %s\n", o.StatsUsername, o.StatsPassword)
		}
	}

	env := app.Environment{
		"ROUTER_SUBDOMAIN":                      o.Subdomain,
		"ROUTER_SERVICE_NAME":                   o.Name,
		"ROUTER_SERVICE_NAMESPACE":              o.Namespace,
		"ROUTER_SERVICE_HTTP_PORT":              "80",
		"ROUTER_SERVICE_HTTPS_PORT":             "443",
		"ROUTER_EXTERNAL_HOST_HOSTNAME":         o.ExternalHost,
		"ROUTER_EXTERNAL_HOST_USERNAME":         o.ExternalHostUsername,
		"ROUTER_EXTERNAL_HOST_PASSWORD":         o.ExternalHostPassword,
		"ROUTER_EXTERNAL_HOST_HTTP_VSERVER":     o.ExternalHostHttpVserver,
		"ROUTER_EXTERNAL_HOST_HTTPS_VSERVER":    o.ExternalHostHttpsVserver,
		"ROUTER_EXTERNAL_HOST_INSECURE":         strconv.FormatBool(o.ExternalHostInsecure),
		"ROUTER_EXTERNAL_HOST_PARTITION_PATH":   o.ExternalHostPartitionPath,
		"ROUTER_EXTERNAL_HOST_PRIVKEY":          privkeyPath,
		"ROUTER_EXTERNAL_HOST_INTERNAL_ADDRESS": o.ExternalHostInternalIP,
		"ROUTER_EXTERNAL_HOST_VXLAN_GW_CIDR":    o.ExternalHostVxLANGateway,
		"ROUTER_CIPHERS":                        o.Ciphers,
		"STATS_PORT":                            strconv.Itoa(o.StatsPort),
		"STATS_USERNAME":                        o.StatsUsername,
		"STATS_PASSWORD":                        o.StatsPassword,
		"ROUTER_THREADS":                        strconv.Itoa(int(o.Threads)),
	}

	if len(o.MaxConnections) > 0 {
		env["ROUTER_MAX_CONNECTIONS"] = o.MaxConnections
	}
	if len(o.ForceSubdomain) > 0 {
		env["ROUTER_SUBDOMAIN"] = o.ForceSubdomain
		env["ROUTER_OVERRIDE_HOSTNAME"] = "true"
	}
	if o.DisableNamespaceOwnershipCheck {
		env["ROUTER_DISABLE_NAMESPACE_OWNERSHIP_CHECK"] = "true"
	}
	if o.StrictSNI {
		env["ROUTER_STRICT_SNI"] = "true"
	}
	if len(o.RouterCanonicalHostname) > 0 {
		if errs := validation.IsDNS1123Subdomain(o.RouterCanonicalHostname); len(errs) != 0 {
			return fmt.Errorf("invalid canonical hostname (RFC 1123): %s", o.RouterCanonicalHostname)
		}
		if errs := validation.IsValidIP(o.RouterCanonicalHostname); len(errs) == 0 {
			return fmt.Errorf("canonical hostname must not be an IP address: %s", o.RouterCanonicalHostname)
		}
		env["ROUTER_CANONICAL_HOSTNAME"] = o.RouterCanonicalHostname
	}
	// automatically start the internal metrics agent if we are handling a known type
	if o.Type == "haproxy-router" && o.StatsPort != 0 {
		env["ROUTER_LISTEN_ADDR"] = fmt.Sprintf("0.0.0.0:%d", o.StatsPort)
		env["ROUTER_METRICS_TYPE"] = "haproxy"
		env["ROUTER_METRICS_TLS_CERT_FILE"] = "/etc/pki/tls/metrics/tls.crt"
		env["ROUTER_METRICS_TLS_KEY_FILE"] = "/etc/pki/tls/metrics/tls.key"
	}
	mtlsAuth := strings.TrimSpace(o.MutualTLSAuth)
	if len(mtlsAuth) > 0 && mtlsAuth != defaultMutualTLSAuth {
		env["ROUTER_MUTUAL_TLS_AUTH"] = o.MutualTLSAuth
		if len(mtlsAuthCA) > 0 {
			env["ROUTER_MUTUAL_TLS_AUTH_CA"] = path.Join(clientCertConfigDir, clientCertConfigCA)
		}
		if len(mtlsAuthCRL) > 0 {
			env["ROUTER_MUTUAL_TLS_AUTH_CRL"] = path.Join(clientCertConfigDir, clientCertConfigCRL)
		}
		if len(o.MutualTLSAuthFilter) > 0 {
			env["ROUTER_MUTUAL_TLS_AUTH_FILTER"] = strings.Replace(o.MutualTLSAuthFilter, " ", "\\ ", -1)
		}
	}

	env.Add(secretEnv)
	if len(defaultCert) > 0 {
		if o.SecretsAsEnv {
			env.Add(app.Environment{"DEFAULT_CERTIFICATE": string(defaultCert)})
		} else {
			env.Add(app.Environment{"DEFAULT_CERTIFICATE_PATH": defaultCertificatePath})
		}
	}
	env.Add(app.Environment{"DEFAULT_CERTIFICATE_DIR": defaultCertificateDir})
	var certName = fmt.Sprintf("%s-certs", o.Name)
	secrets, volumes, routerMounts, err := o.generateSecretsConfig(o.Namespace, certName, defaultCert, mtlsAuthCA, mtlsAuthCRL)
	if err != nil {
		return fmt.Errorf("router could not be created: %v", err)
	}

	var configMaps []*corev1.ConfigMap

	if o.Type == "haproxy-router" && o.ExtendedLogging {
		configMaps = append(configMaps, &corev1.ConfigMap{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "rsyslog-config",
			},
			Data: map[string]string{
				"rsyslog.conf": rsyslogConfigurationFile,
			},
		})
		volumes = append(volumes, corev1.Volume{
			Name: "rsyslog-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "rsyslog-config",
					},
				},
			},
		})
		// Ideally we would use a Unix domain socket in the abstract
		// namespace, but rsyslog does not support that, so we need a
		// filesystem that is common to the router and syslog
		// containers.
		volumes = append(volumes, corev1.Volume{
			Name: "rsyslog-socket",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		routerMounts = append(routerMounts, corev1.VolumeMount{
			Name:      "rsyslog-socket",
			MountPath: "/var/lib/rsyslog",
		})

		env["ROUTER_SYSLOG_ADDRESS"] = "/var/lib/rsyslog/rsyslog.sock"
	}

	livenessProbe := o.generateLivenessProbeConfig(ports)
	readinessProbe := o.generateReadinessProbeConfig(ports)

	exposedPorts := make([]corev1.ContainerPort, len(ports))
	copy(exposedPorts, ports)
	if !o.HostPorts {
		for i := range exposedPorts {
			exposedPorts[i].HostPort = 0
		}
	}
	containers := []corev1.Container{
		{
			Name:            "router",
			Image:           image,
			Ports:           exposedPorts,
			Env:             env.List(),
			LivenessProbe:   livenessProbe,
			ReadinessProbe:  readinessProbe,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    routerMounts,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
	}
	if o.Type == "haproxy-router" && o.ExtendedLogging {
		containers = append(containers, corev1.Container{
			Name:  "syslog",
			Image: image,
			Command: []string{
				"/sbin/rsyslogd", "-n",
				// TODO: Once we have rsyslog 8.32 or later,
				// we can switch to -i NONE.
				"-i", "/tmp/rsyslog.pid",
				"-f", "/etc/rsyslog/rsyslog.conf",
			},
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "rsyslog-config",
					MountPath: "/etc/rsyslog",
				},
				{
					Name:      "rsyslog-socket",
					MountPath: "/var/lib/rsyslog",
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		})
	}

	objects := []runtime.Object{}
	for _, s := range secrets {
		objects = append(objects, s)
	}
	for _, cm := range configMaps {
		objects = append(objects, cm)
	}

	objects = append(objects,
		&corev1.ServiceAccount{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ServiceAccount"},
			ObjectMeta: metav1.ObjectMeta{Name: o.ServiceAccount},
		},
		&authv1.ClusterRoleBinding{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta:   metav1.TypeMeta{APIVersion: authv1.SchemeGroupVersion.String(), Kind: "ClusterRoleBinding"},
			ObjectMeta: metav1.ObjectMeta{Name: generateRoleBindingName(o.Name)},
			Subjects: []corev1.ObjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      o.ServiceAccount,
					Namespace: o.Namespace,
				},
			},
			RoleRef: corev1.ObjectReference{
				Kind: "ClusterRole",
				Name: "system:router",
			},
		},
	)

	objects = append(objects, &appsv1.DeploymentConfig{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "DeploymentConfig"},
		ObjectMeta: metav1.ObjectMeta{
			// this is ok because we know exactly how we want to be serialized
			Name:   o.Name,
			Labels: label,
		},
		Spec: appsv1.DeploymentConfigSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyTypeRolling,
				RollingParams: &appsv1.RollingDeploymentStrategyParams{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "25%",
					},
				},
			},
			Replicas: o.Replicas,
			Selector: label,
			Triggers: []appsv1.DeploymentTriggerPolicy{
				{Type: appsv1.DeploymentTriggerOnConfigChange},
			},
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: label},
				Spec: corev1.PodSpec{
					SecurityContext:    &corev1.PodSecurityContext{},
					HostNetwork:        o.HostNetwork,
					ServiceAccountName: o.ServiceAccount,
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
		case *corev1.Service:
			if t.Annotations == nil {
				t.Annotations = make(map[string]string)
			}
			t.Annotations["prometheus.openshift.io/username"] = o.StatsUsername
			t.Annotations["prometheus.openshift.io/password"] = o.StatsPassword
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
			} else if o.Type == "haproxy-router" && o.StatsPort != 0 {
				// Generate a serving cert for metrics only
				t.Annotations["service.alpha.openshift.io/serving-cert-secret-name"] = "router-metrics-tls"
			}
		}
	}
	// TODO: label all created objects with the same label - router=<name>
	list := &kapi.List{Items: objects}

	if o.Output {
		unstructuredList := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"kind":       "List",
				"apiVersion": "v1",
				"metadata":   map[string]interface{}{},
			},
		}
		for _, item := range objects {
			unstructuredItem, err := runtime.DefaultUnstructuredConverter.ToUnstructured(item)
			if err != nil {
				return err
			}
			unstructuredList.Items = append(unstructuredList.Items, unstructured.Unstructured{Object: unstructuredItem})
		}
		if err := o.Printer.PrintObj(unstructuredList, o.Out); err != nil {
			return err
		}
		return defaultOutputErr
	}

	levelPrefixFilter := func(e error) string {
		// Avoid failing when service accounts or role bindings already exist.
		if ignoreError(e, o.ServiceAccount, generateRoleBindingName(o.Name)) {
			return "warning"
		}
		return "error"
	}

	o.Action.Bulk.IgnoreError = func(e error) bool {
		return levelPrefixFilter(e) == "warning"
	}

	if errs := o.Action.WithMessageAndPrefix(fmt.Sprintf("Creating router %s", o.Name), "created", levelPrefixFilter).Run(list, o.Namespace); len(errs) > 0 {
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

func validateServiceAccount(client securityv1typedclient.SecurityV1Interface, ns string, serviceAccount string, hostNetwork, hostPorts bool) error {
	if !hostNetwork && !hostPorts {
		return nil
	}
	// get cluster sccs
	sccList, err := client.SecurityContextConstraints().List(metav1.ListOptions{})
	if err != nil {
		if !errors.IsUnauthorized(err) {
			return fmt.Errorf("could not retrieve list of security constraints to verify service account %q: %v", serviceAccount, err)
		}
		return nil
	}

	// get set of sccs applicable to the service account
	userInfo := serviceaccount.UserInfo(ns, serviceAccount, "")
	for _, scc := range sccList.Items {
		if constraintAppliesTo(&scc, userInfo, "", nil) {
			switch {
			case hostPorts && scc.AllowHostPorts:
				return nil
			case hostNetwork && scc.AllowHostNetwork:
				return nil
			}
		}
	}

	if hostNetwork {
		errMsg := "service account %q is not allowed to access the host network on nodes, grant access with: oc adm policy add-scc-to-user %s -z %s"
		return fmt.Errorf(errMsg, serviceAccount, bootstrappolicy.SecurityContextConstraintsHostNetwork, serviceAccount)
	}
	if hostPorts {
		errMsg := "service account %q is not allowed to access host ports on nodes, grant access with: oc adm policy add-scc-to-user %s -z %s"
		return fmt.Errorf(errMsg, serviceAccount, bootstrappolicy.SecurityContextConstraintsHostNetwork, serviceAccount)
	}
	return nil
}

// constraintAppliesTo inspects the constraint's users and groups against the userInfo to determine
// if it is usable by the userInfo.  This is a copy from some server code.
// TODO remove this and have the router SA check do a SAR check instead.
// Anything we do here needs to work with a deny authorizer so the choices are limited to SAR / Authorizer
func constraintAppliesTo(constraint *securityv1.SecurityContextConstraints, userInfo user.Info, namespace string, a authorizer.Authorizer) bool {
	for _, user := range constraint.Users {
		if userInfo.GetName() == user {
			return true
		}
	}
	for _, userGroup := range userInfo.GetGroups() {
		if constraintSupportsGroup(userGroup, constraint.Groups) {
			return true
		}
	}
	if a != nil {
		return authorizedForSCC(constraint, userInfo, namespace, a)
	}
	return false
}

// constraintSupportsGroup checks that group is in constraintGroups.
func constraintSupportsGroup(group string, constraintGroups []string) bool {
	for _, g := range constraintGroups {
		if g == group {
			return true
		}
	}
	return false
}

// authorizedForSCC returns true if info is authorized to perform the "use" verb on the SCC resource.
func authorizedForSCC(constraint *securityv1.SecurityContextConstraints, info user.Info, namespace string, a authorizer.Authorizer) bool {
	// check against the namespace that the pod is being created in to allow per-namespace SCC grants.
	attr := authorizer.AttributesRecord{
		User:            info,
		Verb:            "use",
		Namespace:       namespace,
		Name:            constraint.Name,
		APIGroup:        securityv1.GroupName,
		Resource:        "securitycontextconstraints",
		ResourceRequest: true,
	}
	decision, reason, err := a.Authorize(attr)
	if err != nil {
		glog.V(5).Infof("cannot authorize for SCC: %v %q %v", decision, reason, err)
		return false
	}
	return decision == authorizer.DecisionAllow
}
