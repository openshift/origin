package router

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/fields"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	dapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/security/admission"
)

const (
	routerLong = `
Install or configure a router

This command helps to setup a router to take edge traffic and balance it to
your application. With no arguments, the command will check for an existing router
service called 'router' and create one if it does not exist. If you want to test whether
a router has already been created add the --dry-run flag and the command will exit with
1 if the registry does not exist.

If a router does not exist with the given name, this command will
create a deployment configuration and service that will run the router. If you are
running your router in production, you should pass --replicas=2 or higher to ensure
you have failover protection.`

	routerExample = `  # Check the default router ("router")
  $ %[1]s %[2]s --dry-run

  # See what the router would look like if created
  $ %[1]s %[2]s -o json --credentials=/path/to/openshift-router.kubeconfig --service-account=myserviceaccount

  # Create a router if it does not exist
  $ %[1]s %[2]s router-west --credentials=/path/to/openshift-router.kubeconfig --service-account=myserviceaccount --replicas=2

  # Use a different router image and see the router configuration
  $ %[1]s %[2]s region-west -o yaml --credentials=/path/to/openshift-router.kubeconfig --service-account=myserviceaccount --images=myrepo/somerouter:mytag

  # Run the router with a hint to the underlying implementation to _not_ expose statistics.
  $ %[1]s %[2]s router-west --credentials=/path/to/openshift-router.kubeconfig --service-account=myserviceaccount --stats-port=0
  `

	secretsVolumeName = "secret-volume"
	secretsPath       = "/etc/secret-volume"

	privkeySecretName = "external-host-private-key-secret"
	privkeyVolumeName = "external-host-private-key-volume"
	privkeyName       = "router.pem"
	privkeyPath       = secretsPath + "/" + privkeyName
)

// RouterConfig contains the configuration parameters necessary to
// launch a router, including general parameters, type of router, and
// type-specific parameters.
type RouterConfig struct {
	// Type is the router type, which determines which plugin to use (f5
	// or template).
	Type string

	// ImageTemplate specifies the image from which the router will be created.
	ImageTemplate variable.ImageTemplate

	// Ports specifies the container ports for the router.
	Ports string

	// Replicas specifies the initial replica count for the router.
	Replicas int

	// Labels specifies the label or labels that will be assigned to the router
	// pod.
	Labels string

	// DryRun specifies that the router command should not launch a router but
	// should instead exit with code 1 to indicate if a router is already running
	// or code 0 otherwise.
	DryRun bool

	// Credentials specifies the path to a .kubeconfig file with the credentials
	// with which the router may contact the master.
	Credentials string

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

	// ExternalHostInsecure specifies that the router should skip strict
	// certificate verification when connecting to the external host.
	ExternalHostInsecure bool

	// ExternalHostPartitionPath specifies the partition path to use.
	// This is used by some routers to create access access control
	// boundaries for users and applications.
	ExternalHostPartitionPath string
}

var errExit = fmt.Errorf("exit")

const (
	defaultLabel = "router=<name>"

	// Default port numbers to expose and bind/listen on.
	defaultPorts = "80:80,443:443"
)

// NewCmdRouter implements the OpenShift CLI router command.
func NewCmdRouter(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &RouterConfig{
		ImageTemplate: variable.NewDefaultImageTemplate(),

		Labels:   defaultLabel,
		Ports:    defaultPorts,
		Replicas: 1,

		StatsUsername: "admin",
		StatsPort:     1936,
		HostNetwork:   true,
	}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Install a router",
		Long:    routerLong,
		Example: fmt.Sprintf(routerExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunCmdRouter(f, cmd, out, cfg, args)
			if err != errExit {
				cmdutil.CheckErr(err)
			} else {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&cfg.Type, "type", "haproxy-router", "The type of router to use - if you specify --images this flag may be ignored.")
	cmd.Flags().StringVar(&cfg.ImageTemplate.Format, "images", cfg.ImageTemplate.Format, "The image to base this router on - ${component} will be replaced with --type")
	cmd.Flags().BoolVar(&cfg.ImageTemplate.Latest, "latest-images", cfg.ImageTemplate.Latest, "If true, attempt to use the latest images for the router instead of the latest release.")
	cmd.Flags().StringVar(&cfg.Ports, "ports", cfg.Ports, "A comma delimited list of ports or port pairs to expose on the router pod. The default is set for HAProxy.")
	cmd.Flags().IntVar(&cfg.Replicas, "replicas", cfg.Replicas, "The replication factor of the router; commonly 2 when high availability is desired.")
	cmd.Flags().StringVar(&cfg.Labels, "labels", cfg.Labels, "A set of labels to uniquely identify the router and its components.")
	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Exit with code 1 if the specified router does not exist.")
	cmd.Flags().Bool("create", false, "deprecated; this is now the default behavior")
	cmd.Flags().StringVar(&cfg.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the router should use to contact the master.")
	cmd.Flags().StringVar(&cfg.DefaultCertificate, "default-cert", cfg.DefaultCertificate, "Optional path to a certificate file that be used as the default certificate.  The file should contain the cert, key, and any CA certs necessary for the router to serve the certificate.")
	cmd.Flags().StringVar(&cfg.Selector, "selector", cfg.Selector, "Selector used to filter nodes on deployment. Used to run routers on a specific set of nodes.")
	cmd.Flags().StringVar(&cfg.ServiceAccount, "service-account", cfg.ServiceAccount, "Name of the service account to use to run the router pod.")
	cmd.Flags().IntVar(&cfg.StatsPort, "stats-port", cfg.StatsPort, "If the underlying router implementation can provide statistics this is a hint to expose it on this port. Specify 0 if you want to turn off exposing the statistics.")
	cmd.Flags().StringVar(&cfg.StatsPassword, "stats-password", cfg.StatsPassword, "If the underlying router implementation can provide statistics this is the requested password for auth.  If not set a password will be generated.")
	cmd.Flags().StringVar(&cfg.StatsUsername, "stats-user", cfg.StatsUsername, "If the underlying router implementation can provide statistics this is the requested username for auth.")
	cmd.Flags().BoolVar(&cfg.HostNetwork, "host-network", cfg.HostNetwork, "If true (the default), then use host networking rather than using a separate container network stack.")
	cmd.Flags().StringVar(&cfg.ExternalHost, "external-host", cfg.ExternalHost, "If the underlying router implementation connects with an external host, this is the external host's hostname.")
	cmd.Flags().StringVar(&cfg.ExternalHostUsername, "external-host-username", cfg.ExternalHostUsername, "If the underlying router implementation connects with an external host, this is the username for authenticating with the external host.")
	cmd.Flags().StringVar(&cfg.ExternalHostPassword, "external-host-password", cfg.ExternalHostPassword, "If the underlying router implementation connects with an external host, this is the password for authenticating with the external host.")
	cmd.Flags().StringVar(&cfg.ExternalHostHttpVserver, "external-host-http-vserver", cfg.ExternalHostHttpVserver, "If the underlying router implementation uses virtual servers, this is the name of the virtual server for HTTP connections.")
	cmd.Flags().StringVar(&cfg.ExternalHostHttpsVserver, "external-host-https-vserver", cfg.ExternalHostHttpsVserver, "If the underlying router implementation uses virtual servers, this is the name of the virtual server for HTTPS connections.")
	cmd.Flags().StringVar(&cfg.ExternalHostPrivateKey, "external-host-private-key", cfg.ExternalHostPrivateKey, "If the underlying router implementation requires an SSH private key, this is the path to the private key file.")
	cmd.Flags().BoolVar(&cfg.ExternalHostInsecure, "external-host-insecure", cfg.ExternalHostInsecure, "If the underlying router implementation connects with an external host over a secure connection, this causes the router to skip strict certificate verification with the external host.")
	cmd.Flags().StringVar(&cfg.ExternalHostPartitionPath, "external-host-partition-path", cfg.ExternalHostPartitionPath, "If the underlying router implementation uses partitions for control boundaries, this is the path to use for that partition.")

	cmd.MarkFlagFilename("credentials", "kubeconfig")

	cmdutil.AddPrinterFlags(cmd)

	return cmd
}

// Read the specified file and return it as a bytes array.
func loadData(file string) ([]byte, error) {
	if len(file) == 0 {
		return []byte{}, nil
	}

	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}

// Read the specified certificate file and return it as a string.
func loadCert(file string) (string, error) {
	bytes, err := loadData(file)

	return string(bytes), err
}

// Read the specified key file and return it as a bytes array.
func loadKey(file string) ([]byte, error) {
	return loadData(file)
}

// generateSecretsConfig generates any Secret and Volume objects, such
// as SSH private keys, that are necessary for the router container.
func generateSecretsConfig(cfg *RouterConfig, kClient *kclient.Client,
	namespace string) ([]*kapi.Secret, []kapi.Volume, []kapi.VolumeMount,
	error) {
	secrets := []*kapi.Secret{}
	volumes := []kapi.Volume{}
	mounts := []kapi.VolumeMount{}

	if len(cfg.ExternalHostPrivateKey) != 0 {
		privkeyData, err := loadKey(cfg.ExternalHostPrivateKey)
		if err != nil {
			return secrets, volumes, mounts, fmt.Errorf("error reading private key"+
				" for external host: %v", err)
		}

		serviceAccount, err := kClient.ServiceAccounts(namespace).Get(cfg.ServiceAccount)
		if err != nil {
			return secrets, volumes, mounts, fmt.Errorf("error looking up"+
				" service account %s: %v", cfg.ServiceAccount, err)
		}

		privkeySecret := &kapi.Secret{
			ObjectMeta: kapi.ObjectMeta{
				Name: privkeySecretName,
				Annotations: map[string]string{
					kapi.ServiceAccountNameKey: serviceAccount.Name,
					kapi.ServiceAccountUIDKey:  string(serviceAccount.UID),
				},
			},
			Data: map[string][]byte{privkeyName: privkeyData},
		}

		secrets = append(secrets, privkeySecret)
	}

	// We need a secrets volume and mount iff we have secrets.
	if len(secrets) != 0 {
		secretsVolume := kapi.Volume{
			Name: secretsVolumeName,
			VolumeSource: kapi.VolumeSource{
				Secret: &kapi.SecretVolumeSource{
					SecretName: privkeySecretName,
				},
			},
		}

		secretsMount := kapi.VolumeMount{
			Name:      secretsVolumeName,
			ReadOnly:  true,
			MountPath: secretsPath,
		}

		volumes = []kapi.Volume{secretsVolume}
		mounts = []kapi.VolumeMount{secretsMount}
	}

	return secrets, volumes, mounts, nil
}

func generateLivenessProbeConfig(cfg *RouterConfig,
	ports []kapi.ContainerPort) *kapi.Probe {
	var probe *kapi.Probe

	if cfg.Type == "haproxy-router" {
		probe = &kapi.Probe{
			Handler: kapi.Handler{
				TCPSocket: &kapi.TCPSocketAction{
					Port: kutil.IntOrString{
						IntVal: ports[0].ContainerPort,
					},
				},
			},
			InitialDelaySeconds: 10,
		}
	}

	return probe
}

// RunCmdRouter contains all the necessary functionality for the
// OpenShift CLI router command.
func RunCmdRouter(f *clientcmd.Factory, cmd *cobra.Command, out io.Writer, cfg *RouterConfig, args []string) error {
	var name string
	switch len(args) {
	case 0:
		name = "router"
	case 1:
		name = args[0]
	default:
		return cmdutil.UsageError(cmd, "You may pass zero or one arguments to provide a name for the router")
	}

	if len(cfg.StatsUsername) > 0 {
		if strings.Contains(cfg.StatsUsername, ":") {
			return cmdutil.UsageError(cmd, "username %s must not contain ':'", cfg.StatsUsername)
		}
	}

	ports, err := app.ContainerPortsFromString(cfg.Ports)
	if err != nil {
		glog.Fatal(err)
	}

	// For the host networking case, ensure the ports match.
	if cfg.HostNetwork {
		for i := 0; i < len(ports); i++ {
			if ports[i].ContainerPort != ports[i].HostPort {
				return cmdutil.UsageError(cmd, "For host networking mode, please ensure that the container [%v] and host [%v] ports match", ports[i].ContainerPort, ports[i].HostPort)
			}
		}
	}

	if cfg.StatsPort > 0 {
		ports = append(ports, kapi.ContainerPort{
			Name:          "stats",
			HostPort:      cfg.StatsPort,
			ContainerPort: cfg.StatsPort,
			Protocol:      kapi.ProtocolTCP,
		})
	}

	label := map[string]string{"router": name}
	if cfg.Labels != defaultLabel {
		valid, remove, err := app.LabelsFromSpec(strings.Split(cfg.Labels, ","))
		if err != nil {
			glog.Fatal(err)
		}
		if len(remove) > 0 {
			return cmdutil.UsageError(cmd, "You may not pass negative labels in %q", cfg.Labels)
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
			return cmdutil.UsageError(cmd, "You may not pass negative labels in selector %q", cfg.Selector)
		}
		nodeSelector = valid
	}

	image := cfg.ImageTemplate.ExpandOrDie(cfg.Type)

	namespace, _, err := f.OpenShiftClientConfig.Namespace()
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}
	osClient, kClient, err := f.Clients()
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}

	_, output, err := cmdutil.PrinterForCommand(cmd)
	if err != nil {
		return fmt.Errorf("unable to configure printer: %v", err)
	}

	generate := output
	if !generate {
		_, err = kClient.Services(namespace).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("can't check for existing router %q: %v", name, err)
			}
			generate = true
		}
	}

	if generate {
		if cfg.DryRun && !output {
			return fmt.Errorf("router %q does not exist (no service)", name)
		}

		if len(cfg.ServiceAccount) == 0 {
			return fmt.Errorf("router could not be created; you must specify a service account with --service-account")
		}

		err := validateServiceAccount(osClient, namespace, cfg.ServiceAccount)
		if err != nil {
			return fmt.Errorf("router could not be created; %v", err)
		}

		// create new router
		if len(cfg.Credentials) == 0 {
			return fmt.Errorf("router could not be created; you must specify a .kubeconfig file path containing credentials for connecting the router to the master with --credentials")
		}

		clientConfigLoadingRules := &kclientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.Credentials, Precedence: []string{}}
		credentials, err := clientConfigLoadingRules.Load()
		if err != nil {
			return fmt.Errorf("router could not be created; the provided credentials %q could not be loaded: %v", cfg.Credentials, err)
		}
		config, err := kclientcmd.NewDefaultClientConfig(*credentials, &kclientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return fmt.Errorf("router could not be created; the provided credentials %q could not be used: %v", cfg.Credentials, err)
		}
		if err := kclient.LoadTLSFiles(config); err != nil {
			return fmt.Errorf("router could not be created; the provided credentials %q could not load certificate info: %v", cfg.Credentials, err)
		}
		insecure := "false"
		if config.Insecure {
			insecure = "true"
		}

		defaultCert, err := loadCert(cfg.DefaultCertificate)
		if err != nil {
			return fmt.Errorf("router could not be created; error reading default certificate file: %v", err)
		}

		if len(cfg.StatsPassword) == 0 {
			cfg.StatsPassword = generateStatsPassword()
			fmt.Fprintf(out, "password for stats user %s has been set to %s\n", cfg.StatsUsername, cfg.StatsPassword)
		}

		env := app.Environment{
			"OPENSHIFT_MASTER":                    config.Host,
			"OPENSHIFT_CA_DATA":                   string(config.CAData),
			"OPENSHIFT_KEY_DATA":                  string(config.KeyData),
			"OPENSHIFT_CERT_DATA":                 string(config.CertData),
			"OPENSHIFT_INSECURE":                  insecure,
			"DEFAULT_CERTIFICATE":                 defaultCert,
			"ROUTER_SERVICE_NAME":                 name,
			"ROUTER_SERVICE_NAMESPACE":            namespace,
			"ROUTER_EXTERNAL_HOST_HOSTNAME":       cfg.ExternalHost,
			"ROUTER_EXTERNAL_HOST_USERNAME":       cfg.ExternalHostUsername,
			"ROUTER_EXTERNAL_HOST_PASSWORD":       cfg.ExternalHostPassword,
			"ROUTER_EXTERNAL_HOST_HTTP_VSERVER":   cfg.ExternalHostHttpVserver,
			"ROUTER_EXTERNAL_HOST_HTTPS_VSERVER":  cfg.ExternalHostHttpsVserver,
			"ROUTER_EXTERNAL_HOST_INSECURE":       strconv.FormatBool(cfg.ExternalHostInsecure),
			"ROUTER_EXTERNAL_HOST_PARTITION_PATH": cfg.ExternalHostPartitionPath,
			"ROUTER_EXTERNAL_HOST_PRIVKEY":        privkeyPath,
			"STATS_PORT":                          strconv.Itoa(cfg.StatsPort),
			"STATS_USERNAME":                      cfg.StatsUsername,
			"STATS_PASSWORD":                      cfg.StatsPassword,
		}

		updatePercent := int(-25)

		secrets, volumes, mounts, err := generateSecretsConfig(cfg, kClient,
			namespace)
		if err != nil {
			return fmt.Errorf("router could not be created: %v", err)
		}

		livenessProbe := generateLivenessProbeConfig(cfg, ports)

		objects := []runtime.Object{
			&dapi.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{
					Name:   name,
					Labels: label,
				},
				Triggers: []dapi.DeploymentTriggerPolicy{
					{Type: dapi.DeploymentTriggerOnConfigChange},
				},
				Template: dapi.DeploymentTemplate{
					Strategy: dapi.DeploymentStrategy{
						Type:          dapi.DeploymentStrategyTypeRolling,
						RollingParams: &dapi.RollingDeploymentStrategyParams{UpdatePercent: &updatePercent},
					},
					ControllerTemplate: kapi.ReplicationControllerSpec{
						Replicas: cfg.Replicas,
						Selector: label,
						Template: &kapi.PodTemplateSpec{
							ObjectMeta: kapi.ObjectMeta{Labels: label},
							Spec: kapi.PodSpec{
								HostNetwork:        cfg.HostNetwork,
								ServiceAccountName: cfg.ServiceAccount,
								NodeSelector:       nodeSelector,
								Containers: []kapi.Container{
									{
										Name:            "router",
										Image:           image,
										Ports:           ports,
										Env:             env.List(),
										LivenessProbe:   livenessProbe,
										ImagePullPolicy: kapi.PullIfNotPresent,
										VolumeMounts:    mounts,
									},
								},
								Volumes: volumes,
							},
						},
					},
				},
			},
		}

		if len(secrets) != 0 {
			serviceAccount, err := kClient.ServiceAccounts(namespace).Get(cfg.ServiceAccount)
			if err != nil {
				return fmt.Errorf("error looking up service account %s: %v",
					cfg.ServiceAccount, err)
			}

			for _, secret := range secrets {
				objects = append(objects, secret)

				serviceAccount.Secrets = append(serviceAccount.Secrets,
					kapi.ObjectReference{Name: secret.Name})
			}

			_, err = kClient.ServiceAccounts(namespace).Update(serviceAccount)
			if err != nil {
				return fmt.Errorf("error adding secret key to service account %s: %v",
					cfg.ServiceAccount, err)
			}
		}

		objects = app.AddServices(objects, true)
		// TODO: label all created objects with the same label - router=<name>
		list := &kapi.List{Items: objects}

		if output {
			if err := f.PrintObject(cmd, list, out); err != nil {
				return fmt.Errorf("Unable to print object: %v", err)
			}
			return nil
		}

		mapper, typer := f.Factory.Object()
		bulk := configcmd.Bulk{
			Mapper:            mapper,
			Typer:             typer,
			RESTClientFactory: f.Factory.RESTClient,

			After: configcmd.NewPrintNameOrErrorAfter(out, os.Stderr),
		}
		if errs := bulk.Create(list, namespace); len(errs) != 0 {
			return errExit
		}
		return nil
	}

	fmt.Fprintf(out, "Router %q service exists\n", name)
	return nil
}

// generateStatsPassword creates a random password.
func generateStatsPassword() string {
	allowableChars := []rune("abcdefghijlkmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	allowableCharLength := len(allowableChars)
	password := []string{}
	for i := 0; i < 10; i++ {
		char := allowableChars[rand.Intn(allowableCharLength)]
		password = append(password, string(char))
	}
	return strings.Join(password, "")
}

func validateServiceAccount(osClient osclient.Interface, ns string, sa string) error {
	// get cluster sccs
	sccList, err := osClient.PodSecurityPolicies().List(labels.Everything(), fields.Everything())
	if err != nil {
		return fmt.Errorf("unable to validate service account %v", err)
	}

	// get set of sccs applicable to the service account
	userInfo := serviceaccount.UserInfo(ns, sa, "")
	for _, scc := range sccList.Items {
		if admission.ConstraintAppliesTo(&scc, userInfo) {
			// TODO: this needs to check for the host ports we're exposing now
			if len(scc.Spec.HostPorts) > 0 {
				return nil
			}
		}
	}

	return fmt.Errorf("unable to validate service account, host ports are forbidden")
}
