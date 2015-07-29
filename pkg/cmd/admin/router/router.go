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

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/serviceaccount"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

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

If a router does not exist with the given name, the --create flag can be passed to
create a deployment configuration and service that will run the router. If you are
running your router in production, you should pass --replicas=2 or higher to ensure
you have failover protection.`

	routerExample = `  // Check the default router ("router")
  $ %[1]s %[2]s --dry-run

  // See what the router would look like if created
  $ %[1]s %[2]s -o json

  // Create a router if it does not exist
  $ %[1]s %[2]s router-west --create --replicas=2

  // Use a different router image and see the router configuration
  $ %[1]s %[2]s region-west -o yaml --images=myrepo/somerouter:mytag`
)

type RouterConfig struct {
	Type               string
	ImageTemplate      variable.ImageTemplate
	Ports              string
	Replicas           int
	Labels             string
	DryRun             bool
	Credentials        string
	DefaultCertificate string
	Selector           string
	ServiceAccount     string
	StatsPort          int
	StatsPassword      string
	StatsUsername      string
}

var errExit = fmt.Errorf("exit")

const defaultLabel = "router=<name>"

// NewCmdRouter implements the OpenShift cli router command
func NewCmdRouter(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &RouterConfig{
		ImageTemplate: variable.NewDefaultImageTemplate(),

		Labels:   defaultLabel,
		Ports:    "80:80,443:443",
		Replicas: 1,

		StatsUsername: "admin",
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
	cmd.Flags().IntVar(&cfg.StatsPort, "stats-port", 1936, "If the underlying router implementation can provide statistics this is a hint to expose it on this port.")
	cmd.Flags().StringVar(&cfg.StatsPassword, "stats-password", cfg.StatsPassword, "If the underlying router implementation can provide statistics this is the requested password for auth.  If not set a password will be generated.")
	cmd.Flags().StringVar(&cfg.StatsUsername, "stats-user", cfg.StatsUsername, "If the underlying router implementation can provide statistics this is the requested username for auth.")

	cmd.MarkFlagFilename("credentials", "kubeconfig")

	cmdutil.AddPrinterFlags(cmd)

	return cmd
}

func loadDefaultCert(file string) (string, error) {
	if len(file) == 0 {
		return "", nil
	}
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(bytes), err
}

// RunCmdRouter contains all the necessary functionality for the OpenShift cli router command
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
	_, kClient, err := f.Clients()
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}

	p, output, err := cmdutil.PrinterForCommand(cmd)
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

		err := validateServiceAccount(kClient, namespace, cfg.ServiceAccount)
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

		defaultCert, err := loadDefaultCert(cfg.DefaultCertificate)
		if err != nil {
			return fmt.Errorf("router could not be created; error reading default certificate file", err)
		}

		if len(cfg.StatsPassword) == 0 {
			cfg.StatsPassword = generateStatsPassword()
			fmt.Fprintf(out, "password for stats user %s has been set to %s\n", cfg.StatsUsername, cfg.StatsPassword)
		}

		env := app.Environment{
			"OPENSHIFT_MASTER":         config.Host,
			"OPENSHIFT_CA_DATA":        string(config.CAData),
			"OPENSHIFT_KEY_DATA":       string(config.KeyData),
			"OPENSHIFT_CERT_DATA":      string(config.CertData),
			"OPENSHIFT_INSECURE":       insecure,
			"DEFAULT_CERTIFICATE":      defaultCert,
			"ROUTER_SERVICE_NAME":      name,
			"ROUTER_SERVICE_NAMESPACE": namespace,
			"STATS_PORT":               strconv.Itoa(cfg.StatsPort),
			"STATS_USERNAME":           cfg.StatsUsername,
			"STATS_PASSWORD":           cfg.StatsPassword,
		}

		updatePercent := int(-10)

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
								ServiceAccountName: cfg.ServiceAccount,
								NodeSelector:       nodeSelector,
								Containers: []kapi.Container{
									{
										Name:  "router",
										Image: image,
										Ports: ports,
										Env:   env.List(),
										LivenessProbe: &kapi.Probe{
											Handler: kapi.Handler{
												TCPSocket: &kapi.TCPSocketAction{
													Port: kutil.IntOrString{
														IntVal: ports[0].ContainerPort,
													},
												},
											},
											InitialDelaySeconds: 10,
										},
										ImagePullPolicy: kapi.PullIfNotPresent,
									},
								},
							},
						},
					},
				},
			},
		}
		objects = app.AddServices(objects, true)
		// TODO: label all created objects with the same label - router=<name>
		list := &kapi.List{Items: objects}

		if output {
			if err := p.PrintObj(list, out); err != nil {
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

// generateStatsPassword creates a random password
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

func validateServiceAccount(kClient *kclient.Client, ns string, sa string) error {
	// get cluster sccs
	sccList, err := kClient.SecurityContextConstraints().List(labels.Everything(), fields.Everything())
	if err != nil {
		return fmt.Errorf("unable to validate service account %v", err)
	}

	// get set of sccs applicable to the service account
	userInfo := serviceaccount.UserInfo(ns, sa, "")
	for _, scc := range sccList.Items {
		if admission.ConstraintAppliesTo(&scc, userInfo) {
			if scc.AllowHostPorts {
				return nil
			}
		}
	}

	return fmt.Errorf("unable to validate service account, host ports are forbidden")
}
