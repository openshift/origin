package registry

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/intstr"

	authapi "github.com/openshift/origin/pkg/authorization/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
)

const (
	registryLong = `
Install or configure an integrated Docker registry

This command sets up a Docker registry integrated with your cluster to provide notifications when
images are pushed. With no arguments, the command will check for the existing registry service
called 'docker-registry' and try to create it. If you want to test whether the registry has
been created add the --dry-run flag and the command will exit with 1 if the registry does not
exist.

To run a highly available registry, you should be using a remote storage mechanism like an
object store (several are supported by the Docker registry). The default Docker registry image
is configured to accept configuration as environment variables - refer to the configuration file in
that image for more on setting up alternative storage. Once you've made those changes, you can
pass --replicas=2 or higher to ensure you have failover protection. The default registry setup
uses a local volume and the data will be lost if you delete the running pod.

If multiple ports are specified using the option --ports, the first specified port will be
chosen for use as the REGISTRY_HTTP_ADDR and will be passed to Docker registry.

NOTE: This command is intended to simplify the tasks of setting up a Docker registry in a new
  installation. Some configuration beyond this command is still required to make
  your registry persist data.`

	registryExample = `  # Check if default Docker registry ("docker-registry") has been created
  $ %[1]s %[2]s --dry-run

  # See what the registry will look like if created
  $ %[1]s %[2]s -o yaml

  # Create a registry with two replicas if it does not exist
  $ %[1]s %[2]s --replicas=2

  # Use a different registry image
  $ %[1]s %[2]s --images=myrepo/docker-registry:mytag`
)

type RegistryConfig struct {
	Action configcmd.BulkAction

	Name           string
	Type           string
	ImageTemplate  variable.ImageTemplate
	Ports          string
	Replicas       int
	Labels         string
	Volume         string
	HostMount      string
	DryRun         bool
	Credentials    string
	Selector       string
	ServiceAccount string

	ServingCertPath string
	ServingKeyPath  string

	// TODO: accept environment values.
}

// randomSecretSize is the number of random bytes to generate.
const randomSecretSize = 32

const (
	defaultLabel = "docker-registry=default"
	defaultPort  = 5000
	/* TODO: `/healthz` has been deprecated by `/`; keep it temporarily for backwards compatibility until
	 * a next major release with a strict requirement on newer registry image
	 * NOTE that `/` is supported since ose `v3.1.1.0`
	 * To make the transition safe, we could change `HTTPGetAction` to an `ExecAction` which would first curl
	 * `/` and then fallback to `/healthz` if unreachable. Reachable endpoint could be cached on tmpfs inside
	 * a container and be used on subsequent checks. */
	healthzRoute               = "/healthz"
	healthzRouteTimeoutSeconds = 5
	// this is the official private certificate path on Red Hat distros, and is at least structurally more
	// correct than ubuntu based distributions which don't distinguish between public and private certs.
	// Since Origin is CentOS based this is more likely to work.  Ubuntu images should symlink this directory
	// into /etc/ssl/certs to be compatible.
	defaultCertificateDir = "/etc/pki/tls/private"
)

// NewCmdRegistry implements the OpenShift cli registry command
func NewCmdRegistry(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &RegistryConfig{
		ImageTemplate:  variable.NewDefaultImageTemplate(),
		Name:           "registry",
		Labels:         defaultLabel,
		Ports:          strconv.Itoa(defaultPort),
		Volume:         "/registry",
		ServiceAccount: "registry",
		Replicas:       1,
	}

	cmd := &cobra.Command{
		Use:     name,
		Short:   "Install the integrated Docker registry",
		Long:    registryLong,
		Example: fmt.Sprintf(registryExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunCmdRegistry(f, cmd, out, cfg, args)
			if err != cmdutil.ErrExit {
				kcmdutil.CheckErr(err)
			} else {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&cfg.Type, "type", "docker-registry", "The registry image to use - if you specify --images this flag may be ignored.")
	cmd.Flags().StringVar(&cfg.ImageTemplate.Format, "images", cfg.ImageTemplate.Format, "The image to base this registry on - ${component} will be replaced with --type")
	cmd.Flags().BoolVar(&cfg.ImageTemplate.Latest, "latest-images", cfg.ImageTemplate.Latest, "If true, attempt to use the latest image for the registry instead of the latest release.")
	cmd.Flags().StringVar(&cfg.Ports, "ports", cfg.Ports, fmt.Sprintf("A comma delimited list of ports or port pairs to expose on the registry pod. The default is set for %d.", defaultPort))
	cmd.Flags().IntVar(&cfg.Replicas, "replicas", cfg.Replicas, "The replication factor of the registry; commonly 2 when high availability is desired.")
	cmd.Flags().StringVar(&cfg.Labels, "labels", cfg.Labels, "A set of labels to uniquely identify the registry and its components.")
	cmd.Flags().StringVar(&cfg.Volume, "volume", cfg.Volume, "The volume path to use for registry storage; defaults to /registry which is the default for origin-docker-registry.")
	cmd.Flags().StringVar(&cfg.HostMount, "mount-host", cfg.HostMount, "If set, the registry volume will be created as a host-mount at this path.")
	cmd.Flags().Bool("create", false, "deprecated; this is now the default behavior")
	cmd.Flags().StringVar(&cfg.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the registry should use to contact the master.")
	cmd.Flags().StringVar(&cfg.ServiceAccount, "service-account", cfg.ServiceAccount, "Name of the service account to use to run the registry pod.")
	cmd.Flags().StringVar(&cfg.Selector, "selector", cfg.Selector, "Selector used to filter nodes on deployment. Used to run registries on a specific set of nodes.")
	cmd.Flags().StringVar(&cfg.ServingCertPath, "tls-certificate", cfg.ServingCertPath, "An optional path to a PEM encoded certificate (which may contain the private key) for serving over TLS")
	cmd.Flags().StringVar(&cfg.ServingKeyPath, "tls-key", cfg.ServingKeyPath, "An optional path to a PEM encoded private key for serving over TLS")

	// autocompletion hints
	cmd.MarkFlagFilename("credentials", "kubeconfig")

	// Deprecate credentials
	cmd.Flags().MarkDeprecated("credentials", "use --service-account to specify the service account the registry will use to make API calls")

	cfg.Action.BindForOutput(cmd.Flags())
	cmd.Flags().String("output-version", "", "The preferred API versions of the output objects")

	return cmd
}

// RunCmdRegistry contains all the necessary functionality for the OpenShift cli registry command
func RunCmdRegistry(f *clientcmd.Factory, cmd *cobra.Command, out io.Writer, cfg *RegistryConfig, args []string) error {
	var name string
	switch len(args) {
	case 0:
		name = "docker-registry"
	default:
		return kcmdutil.UsageError(cmd, "No arguments are allowed to this command")
	}

	ports, err := app.ContainerPortsFromString(cfg.Ports)
	if err != nil {
		return err
	}

	label := map[string]string{
		"docker-registry": "default",
	}
	if cfg.Labels != defaultLabel {
		valid, remove, err := app.LabelsFromSpec(strings.Split(cfg.Labels, ","))
		if err != nil {
			return err
		}
		if len(remove) > 0 {
			return kcmdutil.UsageError(cmd, "You may not pass negative labels in %q", cfg.Labels)
		}
		label = valid
	}

	nodeSelector := map[string]string{}
	if len(cfg.Selector) > 0 {
		valid, remove, err := app.LabelsFromSpec(strings.Split(cfg.Selector, ","))
		if err != nil {
			return err
		}
		if len(remove) > 0 {
			return kcmdutil.UsageError(cmd, "You may not pass negative labels in selector %q", cfg.Selector)
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

	cfg.Action.Bulk.Mapper = clientcmd.ResourceMapper(f)
	cfg.Action.Out, cfg.Action.ErrOut = out, cmd.Out()
	cfg.Action.Bulk.Op = configcmd.Create

	var clusterIP string

	output := cfg.Action.ShouldPrint()
	generate := output
	if !generate {
		service, err := kClient.Services(namespace).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) && !generate {
				return fmt.Errorf("can't check for existing docker-registry %q: %v", name, err)
			}
			if !output && cfg.Action.DryRun {
				return fmt.Errorf("Docker registry %q service does not exist", name)
			}
			generate = true
		} else {
			clusterIP = service.Spec.ClusterIP
		}
	}

	if !generate {
		fmt.Fprintf(out, "Docker registry %q service exists\n", name)
		return nil
	}

	// create new registry
	secretEnv := app.Environment{}
	switch {
	case len(cfg.ServiceAccount) == 0 && len(cfg.Credentials) == 0:
		return fmt.Errorf("registry could not be created; a service account or the path to a .kubeconfig file must be provided")
	case len(cfg.Credentials) > 0:
		clientConfigLoadingRules := &kclientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.Credentials}
		credentials, err := clientConfigLoadingRules.Load()
		if err != nil {
			return fmt.Errorf("registry does not exist; the provided credentials %q could not be loaded: %v", cfg.Credentials, err)
		}
		config, err := kclientcmd.NewDefaultClientConfig(*credentials, &kclientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return fmt.Errorf("registry does not exist; the provided credentials %q could not be used: %v", cfg.Credentials, err)
		}
		if err := restclient.LoadTLSFiles(config); err != nil {
			return fmt.Errorf("registry does not exist; the provided credentials %q could not load certificate info: %v", cfg.Credentials, err)
		}
		insecure := "false"
		if config.Insecure {
			insecure = "true"
		} else {
			if len(config.KeyData) == 0 || len(config.CertData) == 0 {
				return fmt.Errorf("registry does not exist; the provided credentials %q are missing the client certificate and/or key", cfg.Credentials)
			}
		}

		secretEnv = app.Environment{
			"OPENSHIFT_MASTER":    config.Host,
			"OPENSHIFT_CA_DATA":   string(config.CAData),
			"OPENSHIFT_KEY_DATA":  string(config.KeyData),
			"OPENSHIFT_CERT_DATA": string(config.CertData),
			"OPENSHIFT_INSECURE":  insecure,
		}
	}

	needServiceAccountRole := len(cfg.ServiceAccount) > 0 && len(cfg.Credentials) == 0

	var servingCert, servingKey []byte
	if len(cfg.ServingCertPath) > 0 {
		data, err := ioutil.ReadFile(cfg.ServingCertPath)
		if err != nil {
			return fmt.Errorf("registry does not exist; could not load TLS certificate file %q: %v", cfg.ServingCertPath, err)
		}
		servingCert = data
	}
	if len(cfg.ServingKeyPath) > 0 {
		data, err := ioutil.ReadFile(cfg.ServingKeyPath)
		if err != nil {
			return fmt.Errorf("registry does not exist; could not load TLS private key file %q: %v", cfg.ServingKeyPath, err)
		}
		servingCert = data
	}

	env := app.Environment{}
	env.Add(secretEnv)

	healthzPort := defaultPort
	if len(ports) > 0 {
		healthzPort = ports[0].ContainerPort
		env["REGISTRY_HTTP_ADDR"] = fmt.Sprintf(":%d", healthzPort)
		env["REGISTRY_HTTP_NET"] = "tcp"
	}
	secrets, volumes, mounts, extraEnv, tls, err := generateSecretsConfig(cfg, namespace, servingCert, servingKey)
	if err != nil {
		return err
	}
	env.Add(extraEnv)

	livenessProbe := generateLivenessProbeConfig(healthzPort, tls)
	readinessProbe := generateReadinessProbeConfig(healthzPort, tls)

	mountHost := len(cfg.HostMount) > 0
	podTemplate := &kapi.PodTemplateSpec{
		ObjectMeta: kapi.ObjectMeta{Labels: label},
		Spec: kapi.PodSpec{
			NodeSelector: nodeSelector,
			Containers: []kapi.Container{
				{
					Name:  "registry",
					Image: image,
					Ports: ports,
					Env:   env.List(),
					VolumeMounts: append(mounts, kapi.VolumeMount{
						Name:      "registry-storage",
						MountPath: cfg.Volume,
					}),
					SecurityContext: &kapi.SecurityContext{
						Privileged: &mountHost,
					},
					LivenessProbe:  livenessProbe,
					ReadinessProbe: readinessProbe,
				},
			},
			Volumes: append(volumes, kapi.Volume{
				Name:         "registry-storage",
				VolumeSource: kapi.VolumeSource{},
			}),
			ServiceAccountName: cfg.ServiceAccount,
		},
	}
	if mountHost {
		podTemplate.Spec.Volumes[len(podTemplate.Spec.Volumes)-1].HostPath = &kapi.HostPathVolumeSource{Path: cfg.HostMount}
	} else {
		podTemplate.Spec.Volumes[len(podTemplate.Spec.Volumes)-1].EmptyDir = &kapi.EmptyDirVolumeSource{}
	}

	objects := []runtime.Object{}
	for _, s := range secrets {
		objects = append(objects, s)
	}
	if needServiceAccountRole {
		objects = append(objects,
			&kapi.ServiceAccount{ObjectMeta: kapi.ObjectMeta{Name: cfg.ServiceAccount}},
			&authapi.ClusterRoleBinding{
				ObjectMeta: kapi.ObjectMeta{Name: fmt.Sprintf("registry-%s-role", cfg.Name)},
				Subjects: []kapi.ObjectReference{
					{
						Kind:      "ServiceAccount",
						Name:      cfg.ServiceAccount,
						Namespace: namespace,
					},
				},
				RoleRef: kapi.ObjectReference{
					Kind: "ClusterRole",
					Name: "system:registry",
				},
			},
		)
	}

	objects = append(objects, &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:   name,
			Labels: label,
		},
		Spec: deployapi.DeploymentConfigSpec{
			Replicas: cfg.Replicas,
			Selector: label,
			Triggers: []deployapi.DeploymentTriggerPolicy{
				{Type: deployapi.DeploymentTriggerOnConfigChange},
			},
			Template: podTemplate,
		},
	})
	objects = app.AddServices(objects, true)

	// Set registry service's sessionAffinity to ClientIP to prevent push
	// failures due to a use of poorly consistent storage shared by
	// multiple replicas. Also reuse the cluster IP if provided to avoid
	// changing the internal value.
	for _, obj := range objects {
		switch t := obj.(type) {
		case *kapi.Service:
			t.Spec.SessionAffinity = kapi.ServiceAffinityClientIP
			t.Spec.ClusterIP = clusterIP
		}
	}

	// TODO: label all created objects with the same label
	list := &kapi.List{Items: objects}

	if cfg.Action.ShouldPrint() {
		mapper, _ := f.Object(false)
		fn := cmdutil.VersionedPrintObject(f.PrintObject, cmd, mapper, out)
		if err := fn(list); err != nil {
			return fmt.Errorf("unable to print object: %v", err)
		}
		return nil
	}

	if errs := cfg.Action.WithMessage(fmt.Sprintf("Creating registry %s", cfg.Name), "created").Run(list, namespace); len(errs) > 0 {
		return cmdutil.ErrExit
	}
	return nil
}

func generateLivenessProbeConfig(port int, https bool) *kapi.Probe {
	var scheme kapi.URIScheme
	if https {
		scheme = kapi.URISchemeHTTPS
	}
	return &kapi.Probe{
		InitialDelaySeconds: 10,
		TimeoutSeconds:      healthzRouteTimeoutSeconds,
		Handler: kapi.Handler{
			HTTPGet: &kapi.HTTPGetAction{
				Scheme: scheme,
				Path:   healthzRoute,
				Port:   intstr.FromInt(port),
			},
		},
	}
}

func generateReadinessProbeConfig(port int, https bool) *kapi.Probe {
	var scheme kapi.URIScheme
	if https {
		scheme = kapi.URISchemeHTTPS
	}
	return &kapi.Probe{
		TimeoutSeconds: healthzRouteTimeoutSeconds,
		Handler: kapi.Handler{
			HTTPGet: &kapi.HTTPGetAction{
				Scheme: scheme,
				Path:   healthzRoute,
				Port:   intstr.FromInt(port),
			},
		},
	}
}

// generateSecretsConfig generates any Secret and Volume objects, such
// as the TLS serving cert that are necessary for the registry container.
// Runs true if the registry should be served over TLS.
func generateSecretsConfig(
	cfg *RegistryConfig, namespace string, defaultCrt, defaultKey []byte,
) ([]*kapi.Secret, []kapi.Volume, []kapi.VolumeMount, app.Environment, bool, error) {
	var secrets []*kapi.Secret
	var volumes []kapi.Volume
	var mounts []kapi.VolumeMount
	extraEnv := app.Environment{}

	if len(defaultCrt) > 0 && len(defaultKey) == 0 {
		keys, err := cmdutil.PrivateKeysFromPEM(defaultCrt)
		if err != nil {
			return nil, nil, nil, nil, false, err
		}
		if len(keys) == 0 {
			return nil, nil, nil, nil, false, fmt.Errorf("the default cert must contain a private key")
		}
		defaultKey = keys
	}

	if len(defaultCrt) > 0 {
		secret := &kapi.Secret{
			ObjectMeta: kapi.ObjectMeta{
				Name: fmt.Sprintf("%s-certs", cfg.Name),
			},
			Type: kapi.SecretTypeTLS,
			Data: map[string][]byte{
				kapi.TLSCertKey:       defaultCrt,
				kapi.TLSPrivateKeyKey: defaultKey,
			},
		}
		secrets = append(secrets, secret)
		volume := kapi.Volume{
			Name: "server-certificate",
			VolumeSource: kapi.VolumeSource{
				Secret: &kapi.SecretVolumeSource{
					SecretName: secret.Name,
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

		extraEnv.Add(app.Environment{
			"REGISTRY_HTTP_TLS_CERTIFICATE": path.Join(defaultCertificateDir, kapi.TLSCertKey),
			"REGISTRY_HTTP_TLS_KEY":         path.Join(defaultCertificateDir, kapi.TLSPrivateKeyKey),
		})
	}

	secretBytes := make([]byte, randomSecretSize)
	if _, err := cryptorand.Read(secretBytes); err != nil {
		return nil, nil, nil, nil, false, fmt.Errorf("registry does not exist; could not generate random bytes for HTTP secret: %v", err)
	}
	httpSecretString := base64.StdEncoding.EncodeToString(secretBytes)
	extraEnv["REGISTRY_HTTP_SECRET"] = httpSecretString

	return secrets, volumes, mounts, extraEnv, len(defaultCrt) > 0, nil
}
