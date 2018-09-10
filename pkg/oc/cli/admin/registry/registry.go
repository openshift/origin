package registry

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	kappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	corev1typedclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
	authv1 "github.com/openshift/api/authorization/v1"
	configcmd "github.com/openshift/origin/pkg/bulk"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/lib/newapp/app"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
)

var (
	registryLong = templates.LongDesc(`
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
		your registry persist data.`)

	registryExample = templates.Examples(`
		# Check if default Docker registry ("docker-registry") has been created
	  %[1]s %[2]s --dry-run

	  # See what the registry will look like if created
	  %[1]s %[2]s -o yaml

	  # Create a registry with two replicas if it does not exist
	  %[1]s %[2]s --replicas=2

	  # Use a different registry image
	  %[1]s %[2]s --images=myrepo/docker-registry:mytag

	  # Enforce quota and limits on images
	  %[1]s %[2]s --enforce-quota`)
)

// RegistryOptions contains the configuration for the registry as well as any other
// helpers required to run the command.
type RegistryOptions struct {
	Config *RegistryConfig

	// helpers required for Run.
	factory       kcmdutil.Factory
	cmd           *cobra.Command
	label         map[string]string
	nodeSelector  map[string]string
	ports         []corev1.ContainerPort
	namespace     string
	serviceClient corev1typedclient.ServicesGetter
	image         string

	genericclioptions.IOStreams
}

// RegistryConfig contains configuration for the registry that will be created.
type RegistryConfig struct {
	PrintFlags *genericclioptions.PrintFlags

	Printer printers.ResourcePrinter

	Action configcmd.BulkAction

	Name           string
	Type           string
	ImageTemplate  variable.ImageTemplate
	Ports          string
	Replicas       int32
	Labels         string
	Volume         string
	HostMount      string
	DryRun         bool
	Selector       string
	ServiceAccount string
	DaemonSet      bool
	EnforceQuota   bool

	// SupplementalGroups is list of int64, however cobra does not have appropriate func
	// for that type list.
	SupplementalGroups []string
	FSGroup            string

	ServingCertPath string
	ServingKeyPath  string

	ClusterIP string

	Local bool
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

func NewRegistryOpts(streams genericclioptions.IOStreams) *RegistryOptions {
	return &RegistryOptions{
		Config: &RegistryConfig{
			PrintFlags: genericclioptions.NewPrintFlags("configured").WithTypeSetter(scheme.Scheme),

			ImageTemplate:  variable.NewDefaultImageTemplate(),
			Name:           "registry",
			Labels:         defaultLabel,
			Ports:          strconv.Itoa(defaultPort),
			Volume:         "/registry",
			ServiceAccount: "registry",
			Replicas:       1,
			EnforceQuota:   false,
		},

		IOStreams: streams,
	}
}

// NewCmdRegistry implements the OpenShift cli registry command
func NewCmdRegistry(f kcmdutil.Factory, parentName, name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRegistryOpts(streams)

	cmd := &cobra.Command{
		Use:     name,
		Short:   "Install the integrated Docker registry",
		Long:    registryLong,
		Example: fmt.Sprintf(registryExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.RunCmdRegistry())
		},
	}

	cmd.Flags().StringVar(&o.Config.Type, "type", "docker-registry", "The registry image to use - if you specify --images this flag may be ignored.")
	cmd.Flags().StringVar(&o.Config.ImageTemplate.Format, "images", o.Config.ImageTemplate.Format, "The image to base this registry on - ${component} will be replaced with --type")
	cmd.Flags().BoolVar(&o.Config.ImageTemplate.Latest, "latest-images", o.Config.ImageTemplate.Latest, "If true, attempt to use the latest image for the registry instead of the latest release.")
	cmd.Flags().StringVar(&o.Config.Ports, "ports", o.Config.Ports, fmt.Sprintf("A comma delimited list of ports or port pairs to expose on the registry pod. The default is set for %d.", defaultPort))
	cmd.Flags().Int32Var(&o.Config.Replicas, "replicas", o.Config.Replicas, "The replication factor of the registry; commonly 2 when high availability is desired.")
	cmd.Flags().StringVar(&o.Config.Labels, "labels", o.Config.Labels, "A set of labels to uniquely identify the registry and its components.")
	cmd.Flags().StringVar(&o.Config.Volume, "volume", o.Config.Volume, "The volume path to use for registry storage; defaults to /registry which is the default for origin-docker-registry.")
	cmd.Flags().StringVar(&o.Config.HostMount, "mount-host", o.Config.HostMount, "If set, the registry volume will be created as a host-mount at this path.")
	cmd.Flags().Bool("create", false, "deprecated; this is now the default behavior")
	cmd.Flags().StringVar(&o.Config.ServiceAccount, "service-account", o.Config.ServiceAccount, "Name of the service account to use to run the registry pod.")
	cmd.Flags().StringVar(&o.Config.Selector, "selector", o.Config.Selector, "Selector used to filter nodes on deployment. Used to run registries on a specific set of nodes.")
	cmd.Flags().StringVar(&o.Config.ServingCertPath, "tls-certificate", o.Config.ServingCertPath, "An optional path to a PEM encoded certificate (which may contain the private key) for serving over TLS")
	cmd.Flags().StringVar(&o.Config.ServingKeyPath, "tls-key", o.Config.ServingKeyPath, "An optional path to a PEM encoded private key for serving over TLS")
	cmd.Flags().StringSliceVar(&o.Config.SupplementalGroups, "supplemental-groups", o.Config.SupplementalGroups, "Specify supplemental groups which is an array of ID's that grants group access to registry shared storage")
	cmd.Flags().StringVar(&o.Config.FSGroup, "fs-group", "", "Specify fsGroup which is an ID that grants group access to registry block storage")
	cmd.Flags().StringVar(&o.Config.ClusterIP, "cluster-ip", "", "Specify the ClusterIP value for the docker-registry service")
	cmd.Flags().BoolVar(&o.Config.DaemonSet, "daemonset", o.Config.DaemonSet, "If true, use a daemonset instead of a deployment config.")
	cmd.Flags().BoolVar(&o.Config.EnforceQuota, "enforce-quota", o.Config.EnforceQuota, "If true, the registry will refuse to write blobs if they exceed quota limits")
	cmd.Flags().BoolVar(&o.Config.Local, "local", o.Config.Local, "If true, do not contact the apiserver")

	o.Config.PrintFlags.AddFlags(cmd)
	o.Config.Action.BindForOutput(cmd.Flags(), "output", "template")
	return cmd
}

// Complete completes any options that are required by validate or run steps.
func (opts *RegistryOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return kcmdutil.UsageErrorf(cmd, "No arguments are allowed to this command")
	}

	opts.image = opts.Config.ImageTemplate.ExpandOrDie(opts.Config.Type)

	opts.label = map[string]string{
		"docker-registry": "default",
	}
	if opts.Config.Labels != defaultLabel {
		valid, remove, err := app.LabelsFromSpec(strings.Split(opts.Config.Labels, ","))
		if err != nil {
			return err
		}
		if len(remove) > 0 {
			return kcmdutil.UsageErrorf(cmd, "You may not pass negative labels in %q", opts.Config.Labels)
		}
		opts.label = valid
	}

	opts.nodeSelector = map[string]string{}
	if len(opts.Config.Selector) > 0 {
		valid, remove, err := app.LabelsFromSpec(strings.Split(opts.Config.Selector, ","))
		if err != nil {
			return err
		}
		if len(remove) > 0 {
			return kcmdutil.UsageErrorf(cmd, "You may not pass negative labels in selector %q", opts.Config.Selector)
		}
		opts.nodeSelector = valid
	}

	if len(opts.Config.FSGroup) > 0 {
		if _, err := strconv.ParseInt(opts.Config.FSGroup, 10, 64); err != nil {
			return kcmdutil.UsageErrorf(cmd, "invalid group ID %q specified for fsGroup (%v)", opts.Config.FSGroup, err)
		}
	}

	if len(opts.Config.SupplementalGroups) > 0 {
		for _, v := range opts.Config.SupplementalGroups {
			if val, err := strconv.ParseInt(v, 10, 64); err != nil || val == 0 {
				return kcmdutil.UsageErrorf(cmd, "invalid group ID %q specified for supplemental group (%v)", v, err)
			}
		}
	}
	if len(opts.Config.SupplementalGroups) > 0 && len(opts.Config.FSGroup) > 0 {
		return kcmdutil.UsageErrorf(cmd, "fsGroup and supplemental groups cannot be specified both at the same time")
	}

	var portsErr error
	if opts.ports, portsErr = app.ContainerPortsFromString(opts.Config.Ports); portsErr != nil {
		return portsErr
	}

	var nsErr error
	if opts.namespace, _, nsErr = f.ToRawKubeConfigLoader().Namespace(); nsErr != nil {
		return fmt.Errorf("error getting namespace: %v", nsErr)
	}

	if !opts.Config.Local {
		config, err := f.ToRESTConfig()
		if err != nil {
			return err
		}

		kclient, err := corev1typedclient.NewForConfig(config)
		if err != nil {
			return err
		}

		opts.serviceClient = kclient
	}

	if opts.Config.Local && !opts.Config.Action.DryRun {
		return fmt.Errorf("--local cannot be specified without --dry-run")
	}

	if opts.Config.DryRun {
		opts.Config.PrintFlags.Complete("%s (dry run)")
	}

	var err error
	opts.Config.Printer, err = opts.Config.PrintFlags.ToPrinter()
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

	opts.Config.Action.Bulk.Scheme = legacyscheme.Scheme
	opts.Config.Action.Out, opts.Config.Action.ErrOut = opts.Out, opts.ErrOut
	opts.Config.Action.Bulk.Op = configcmd.Creator{
		Client:     dynamicClient,
		RESTMapper: restMapper,
	}.Create
	opts.cmd = cmd
	opts.factory = f

	return nil
}

// RunCmdRegistry contains all the necessary functionality for the OpenShift cli registry command
func (opts *RegistryOptions) RunCmdRegistry() error {
	name := "docker-registry"

	var clusterIP string

	shouldPrint := opts.Config.PrintFlags.OutputFormat != nil && len(*opts.Config.PrintFlags.OutputFormat) > 0 && *opts.Config.PrintFlags.OutputFormat != "name"
	generate := shouldPrint
	if !opts.Config.Local {
		service, err := opts.serviceClient.Services(opts.namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if !generate {
				if !errors.IsNotFound(err) {
					return fmt.Errorf("can't check for existing docker-registry %q: %v", name, err)
				}
				if opts.Config.Action.DryRun {
					return fmt.Errorf("Docker registry %q service does not exist", name)
				}
				generate = true
			}
		} else {
			clusterIP = service.Spec.ClusterIP
		}
	}

	if !generate {
		fmt.Fprintf(opts.Out, "Docker registry %q service exists\n", name)
		return nil
	}

	if len(opts.Config.ClusterIP) > 0 {
		clusterIP = opts.Config.ClusterIP
	}

	// create new registry
	secretEnv := app.Environment{}
	if len(opts.Config.ServiceAccount) == 0 {
		return fmt.Errorf("registry could not be created; a service account must be provided")
	}

	var servingCert, servingKey []byte
	if len(opts.Config.ServingCertPath) > 0 {
		data, err := ioutil.ReadFile(opts.Config.ServingCertPath)
		if err != nil {
			return fmt.Errorf("registry does not exist; could not load TLS certificate file %q: %v", opts.Config.ServingCertPath, err)
		}
		servingCert = data
	}
	if len(opts.Config.ServingKeyPath) > 0 {
		data, err := ioutil.ReadFile(opts.Config.ServingKeyPath)
		if err != nil {
			return fmt.Errorf("registry does not exist; could not load TLS private key file %q: %v", opts.Config.ServingKeyPath, err)
		}
		servingKey = data
	}

	env := app.Environment{}
	env.Add(secretEnv)

	env["REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ENFORCEQUOTA"] = fmt.Sprintf("%t", opts.Config.EnforceQuota)
	healthzPort := defaultPort
	if len(opts.ports) > 0 {
		healthzPort = int(opts.ports[0].ContainerPort)
		env["REGISTRY_HTTP_ADDR"] = fmt.Sprintf(":%d", healthzPort)
		env["REGISTRY_HTTP_NET"] = "tcp"
	}
	secrets, volumes, mounts, extraEnv, tls, err := generateSecretsConfig(opts.Config, servingCert, servingKey)
	if err != nil {
		return err
	}
	env.Add(extraEnv)

	livenessProbe := generateLivenessProbeConfig(healthzPort, tls)
	readinessProbe := generateReadinessProbeConfig(healthzPort, tls)

	mountHost := len(opts.Config.HostMount) > 0
	podTemplate := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: opts.label},
		Spec: corev1.PodSpec{
			NodeSelector: opts.nodeSelector,
			Containers: []corev1.Container{
				{
					Name:  "registry",
					Image: opts.image,
					Ports: opts.ports,
					Env:   env.List(),
					VolumeMounts: append(mounts, corev1.VolumeMount{
						Name:      "registry-storage",
						MountPath: opts.Config.Volume,
					}),
					SecurityContext: &corev1.SecurityContext{
						Privileged: &mountHost,
					},
					LivenessProbe:  livenessProbe,
					ReadinessProbe: readinessProbe,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
			Volumes: append(volumes, corev1.Volume{
				Name:         "registry-storage",
				VolumeSource: corev1.VolumeSource{},
			}),
			ServiceAccountName: opts.Config.ServiceAccount,
			SecurityContext:    generateSecurityContext(opts.Config),
		},
	}
	if mountHost {
		podTemplate.Spec.Volumes[len(podTemplate.Spec.Volumes)-1].HostPath = &corev1.HostPathVolumeSource{Path: opts.Config.HostMount}
	} else {
		podTemplate.Spec.Volumes[len(podTemplate.Spec.Volumes)-1].EmptyDir = &corev1.EmptyDirVolumeSource{}
	}

	objects := []runtime.Object{}
	for _, s := range secrets {
		objects = append(objects, s)
	}

	objects = append(objects,
		&corev1.ServiceAccount{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ServiceAccount"},
			ObjectMeta: metav1.ObjectMeta{Name: opts.Config.ServiceAccount},
		},
		&authv1.ClusterRoleBinding{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta:   metav1.TypeMeta{APIVersion: authv1.SchemeGroupVersion.String(), Kind: "ClusterRoleBinding"},
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("registry-%s-role", opts.Config.Name)},
			Subjects: []corev1.ObjectReference{
				{
					Kind:      "ServiceAccount",
					Name:      opts.Config.ServiceAccount,
					Namespace: opts.namespace,
				},
			},
			RoleRef: corev1.ObjectReference{
				Kind: "ClusterRole",
				Name: "system:registry",
			},
		},
	)

	if opts.Config.DaemonSet {
		objects = append(objects, &kappsv1.DaemonSet{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta: metav1.TypeMeta{APIVersion: kappsv1.SchemeGroupVersion.String(), Kind: "DaemonSet"},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: opts.label,
			},
			Spec: kappsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: opts.label},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: podTemplate.ObjectMeta,
					Spec:       podTemplate.Spec,
				},
			},
		})
	} else {
		objects = append(objects, &appsv1.DeploymentConfig{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "DeploymentConfig"},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: opts.label,
			},
			Spec: appsv1.DeploymentConfigSpec{
				Replicas: opts.Config.Replicas,
				Selector: opts.label,
				Triggers: []appsv1.DeploymentTriggerPolicy{
					{Type: appsv1.DeploymentTriggerOnConfigChange},
				},
				Template: podTemplate,
			},
		})
	}

	objects = app.AddServices(objects, true)

	// Set registry service's sessionAffinity to ClientIP to prevent push
	// failures due to a use of poorly consistent storage shared by
	// multiple replicas. Also reuse the cluster IP if provided to avoid
	// changing the internal value.
	for _, obj := range objects {
		switch t := obj.(type) {
		case *corev1.Service:
			t.Spec.SessionAffinity = corev1.ServiceAffinityClientIP
			t.Spec.ClusterIP = clusterIP
		}
	}

	// TODO: label all created objects with the same label
	list := &kapi.List{Items: objects}

	if shouldPrint {
		printableList := &corev1.List{}
		for _, obj := range objects {
			printableList.Items = append(printableList.Items, runtime.RawExtension{
				Object: obj,
			})
		}

		if err := opts.Config.Printer.PrintObj(printableList, opts.Out); err != nil {
			return fmt.Errorf("unable to print object: %v", err)
		}
		return nil
	}

	if errs := opts.Config.Action.WithMessage(fmt.Sprintf("Creating registry %s", opts.Config.Name), "created").Run(list, opts.namespace); len(errs) > 0 {
		return kcmdutil.ErrExit
	}
	return nil
}

func generateLivenessProbeConfig(port int, https bool) *corev1.Probe {
	probeConfig := generateProbeConfig(port, https)
	probeConfig.InitialDelaySeconds = 10

	return probeConfig
}

func generateReadinessProbeConfig(port int, https bool) *corev1.Probe {
	return generateProbeConfig(port, https)
}

func generateProbeConfig(port int, https bool) *corev1.Probe {
	var scheme corev1.URIScheme
	if https {
		scheme = corev1.URISchemeHTTPS
	}
	return &corev1.Probe{
		TimeoutSeconds: healthzRouteTimeoutSeconds,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
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
	cfg *RegistryConfig, defaultCrt, defaultKey []byte,
) ([]*corev1.Secret, []corev1.Volume, []corev1.VolumeMount, app.Environment, bool, error) {
	var secrets []*corev1.Secret
	var volumes []corev1.Volume
	var mounts []corev1.VolumeMount
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
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-certs", cfg.Name),
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       defaultCrt,
				corev1.TLSPrivateKeyKey: defaultKey,
			},
		}
		secrets = append(secrets, secret)
		volume := corev1.Volume{
			Name: "server-certificate",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
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

		extraEnv.Add(app.Environment{
			"REGISTRY_HTTP_TLS_CERTIFICATE": path.Join(defaultCertificateDir, corev1.TLSCertKey),
			"REGISTRY_HTTP_TLS_KEY":         path.Join(defaultCertificateDir, corev1.TLSPrivateKeyKey),
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

func generateSecurityContext(conf *RegistryConfig) *corev1.PodSecurityContext {
	result := &corev1.PodSecurityContext{}
	if len(conf.SupplementalGroups) > 0 {
		result.SupplementalGroups = []int64{}
		for _, val := range conf.SupplementalGroups {
			// The errors are handled by Complete()
			if groupID, err := strconv.ParseInt(val, 10, 64); err == nil {
				result.SupplementalGroups = append(result.SupplementalGroups, groupID)
			}
		}
	}
	if len(conf.FSGroup) > 0 {
		if groupID, err := strconv.ParseInt(conf.FSGroup, 10, 64); err == nil {
			result.FSGroup = &groupID
		}
	}
	return result
}
