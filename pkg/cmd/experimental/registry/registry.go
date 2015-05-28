package registry

import (
	"fmt"
	"io"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	dapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
)

const (
	registry_long = `Install or configure a Docker registry for OpenShift

This command sets up a Docker registry integrated with OpenShift to provide notifications when
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

NOTE: This command is intended to simplify the tasks of setting up a Docker registry in a new
  installation. Some configuration beyond this command is still required to make
  your registry persist data.`

	registry_example = `  // Check if default Docker registry ("docker-registry") has been created
  $ %[1]s %[2]s --dry-run

  // See what the registry would look like if created
  $ %[1]s %[2]s -o json

  // Create a registry if it does not exist with two replicas
  $ %[1]s %[2]s --replicas=2 --credentials=registry-user.kubeconfig

  // Use a different registry image and see the registry configuration
  $ %[1]s %[2]s -o yaml --images=myrepo/docker-registry:mytag`
)

type RegistryConfig struct {
	Type          string
	ImageTemplate variable.ImageTemplate
	Ports         string
	Replicas      int
	Labels        string
	Volume        string
	HostMount     string
	DryRun        bool
	Credentials   string

	// TODO: accept environment values.
}

var errExit = fmt.Errorf("exit")

const defaultLabel = "docker-registry=default"

func NewCmdRegistry(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &RegistryConfig{
		ImageTemplate: variable.NewDefaultImageTemplate(),

		Labels:   defaultLabel,
		Ports:    "5000",
		Volume:   "/registry",
		Replicas: 1,
	}

	cmd := &cobra.Command{
		Use:     name,
		Short:   "Install the OpenShift Docker registry",
		Long:    registry_long,
		Example: fmt.Sprintf(registry_example, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunCmdRegistry(f, cmd, out, cfg, args)
			if err != errExit {
				cmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&cfg.Type, "type", "docker-registry", "The registry image to use - if you specify --images this flag may be ignored.")
	cmd.Flags().StringVar(&cfg.ImageTemplate.Format, "images", cfg.ImageTemplate.Format, "The image to base this registry on - ${component} will be replaced with --type")
	cmd.Flags().BoolVar(&cfg.ImageTemplate.Latest, "latest-images", cfg.ImageTemplate.Latest, "If true, attempt to use the latest image for the registry instead of the latest release.")
	cmd.Flags().StringVar(&cfg.Ports, "ports", cfg.Ports, "A comma delimited list of ports or port pairs to expose on the registry pod. The default is set for 5000.")
	cmd.Flags().IntVar(&cfg.Replicas, "replicas", cfg.Replicas, "The replication factor of the registry; commonly 2 when high availability is desired.")
	cmd.Flags().StringVar(&cfg.Labels, "labels", cfg.Labels, "A set of labels to uniquely identify the registry and its components.")
	cmd.Flags().StringVar(&cfg.Volume, "volume", cfg.Volume, "The volume path to use for registry storage; defaults to /registry which is the default for origin-docker-registry.")
	cmd.Flags().StringVar(&cfg.HostMount, "mount-host", cfg.HostMount, "If set, the registry volume will be created as a host-mount at this path.")
	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Check if the registry exists instead of creating.")
	cmd.Flags().Bool("create", false, "deprecated; this is now the default behavior")
	cmd.Flags().StringVar(&cfg.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the registry should use to contact the master.")

	cmdutil.AddPrinterFlags(cmd)

	return cmd
}

func RunCmdRegistry(f *clientcmd.Factory, cmd *cobra.Command, out io.Writer, cfg *RegistryConfig, args []string) error {
	var name string
	switch len(args) {
	case 0:
		name = "docker-registry"
	default:
		return cmdutil.UsageError(cmd, "No arguments are allowed to this command")
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
			return cmdutil.UsageError(cmd, "You may not pass negative labels in %q", cfg.Labels)
		}
		label = valid
	}

	image := cfg.ImageTemplate.ExpandOrDie(cfg.Type)

	namespace, err := f.OpenShiftClientConfig.Namespace()
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
				return fmt.Errorf("can't check for existing docker-registry %q: %v", name, err)
			}
			generate = true
		}
	}

	if generate {
		if cfg.DryRun && !output {
			return fmt.Errorf("docker-registry %q does not exist (no service).", name)
		}

		// create new registry
		if len(cfg.Credentials) == 0 {
			return fmt.Errorf("registry does not exist; you must specify a .kubeconfig file path containing credentials for connecting the registry to the master with --credentials")
		}
		clientConfigLoadingRules := &kclientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.Credentials}
		credentials, err := clientConfigLoadingRules.Load()
		if err != nil {
			return fmt.Errorf("registry does not exist; the provided credentials %q could not be loaded: %v", cfg.Credentials, err)
		}
		config, err := kclientcmd.NewDefaultClientConfig(*credentials, &kclientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return fmt.Errorf("registry does not exist; the provided credentials %q could not be used: %v", cfg.Credentials, err)
		}
		if err := kclient.LoadTLSFiles(config); err != nil {
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

		env := app.Environment{
			"OPENSHIFT_MASTER":    config.Host,
			"OPENSHIFT_CA_DATA":   string(config.CAData),
			"OPENSHIFT_KEY_DATA":  string(config.KeyData),
			"OPENSHIFT_CERT_DATA": string(config.CertData),
			"OPENSHIFT_INSECURE":  insecure,
		}

		mountHost := len(cfg.HostMount) > 0
		podTemplate := &kapi.PodTemplateSpec{
			ObjectMeta: kapi.ObjectMeta{Labels: label},
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					{
						Name:  "registry",
						Image: image,
						Ports: ports,
						Env:   env.List(),
						VolumeMounts: []kapi.VolumeMount{
							{
								Name:      "registry-storage",
								MountPath: cfg.Volume,
							},
						},
						SecurityContext: &kapi.SecurityContext{
							Privileged: &mountHost,
						},
						// TODO reenable the liveness probe when we no longer support the v1 registry.
						/*
							LivenessProbe: &kapi.Probe{
								InitialDelaySeconds: 3,
								TimeoutSeconds:      5,
								Handler: kapi.Handler{
									HTTPGet: &kapi.HTTPGetAction{
										Path: "/healthz",
										Port: util.NewIntOrStringFromInt(5000),
									},
								},
							},
						*/
					},
				},
				Volumes: []kapi.Volume{
					{
						Name:         "registry-storage",
						VolumeSource: kapi.VolumeSource{},
					},
				},
			},
		}
		if mountHost {
			podTemplate.Spec.Volumes[0].HostPath = &kapi.HostPathVolumeSource{Path: cfg.HostMount}
		} else {
			podTemplate.Spec.Volumes[0].EmptyDir = &kapi.EmptyDirVolumeSource{}
		}

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
						Type: dapi.DeploymentStrategyTypeRecreate,
					},
					ControllerTemplate: kapi.ReplicationControllerSpec{
						Replicas: cfg.Replicas,
						Selector: label,
						Template: podTemplate,
					},
				},
			},
		}
		objects = app.AddServices(objects)
		// TODO: label all created objects with the same label
		list := &kapi.List{Items: objects}

		if output {
			if err := p.PrintObj(list, out); err != nil {
				return fmt.Errorf("unable to print object: %v", err)
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

	fmt.Fprintf(out, "Docker registry %q service exists\n", name)
	return nil
}
