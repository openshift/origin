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
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	dapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
)

const longDesc = `
Install or configure a Docker registry for OpenShift

This command sets up a Docker registry integrated with OpenShift to provide notifications when
images are pushed. With no arguments, the command will check for the existing registry service
called 'docker-registry' and perform some diagnostics to ensure the registry is properly
configured and functioning.

If a registry service does not exist, the --create flag can be passed to
create a deployment configuration and service that will run the registry.

To run a highly available registry, you should be using a remote storage mechanism like an
object store (several are supported by the Docker registry). The default Docker registry image
is configured to accept configuration as environment variables - refer to the config file in
that image for more on setting up alternative storage. Once you've made those changes, you can
pass --replicas=2 or higher to ensure you have failover protection. The default registry setup
uses a local volume and the data will be lost if you delete the running pod.

Examples:
  Check the default Docker registry ("docker-registry"):

  $ %[1]s %[2]s

  See what the registry would look like if created:

  $ %[1]s %[2]s -o json

  Create a registry if it does not exist with two replicas:

  $ %[1]s %[2]s --create --replicas=2

  Use a different registry image and see the registry configuration:

  $ %[1]s %[2]s -o yaml --images=myrepo/docker-registry:mytag

ALPHA: This command is currently being actively developed. It is intended to simplify
  the tasks of setting up a Docker registry in a new installation. Some configuration
  beyond this command is still required to make your registry permanent.
`

type config struct {
	Type          string
	ImageTemplate variable.ImageTemplate
	Ports         string
	Replicas      int
	Labels        string
	Volume        string
	HostMount     string
	Create        bool
	Credentials   string

	// TODO: accept environment values.
}

const defaultLabel = "docker-registry=default"

func NewCmdRegistry(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &config{
		ImageTemplate: variable.NewDefaultImageTemplate(),

		Labels:   defaultLabel,
		Ports:    "5000",
		Volume:   "/registry",
		Replicas: 1,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Install and check OpenShift Docker registry",
		Long:  fmt.Sprintf(longDesc, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			var name string
			switch len(args) {
			case 0:
				name = "docker-registry"
			default:
				glog.Fatalf("No arguments are allowed to this command")
			}

			ports, err := app.ContainerPortsFromString(cfg.Ports)
			if err != nil {
				glog.Fatal(err)
			}

			label := map[string]string{
				"docker-registry": "default",
			}
			if cfg.Labels != defaultLabel {
				valid, remove, err := app.LabelsFromSpec(strings.Split(cfg.Labels, ","))
				if err != nil {
					glog.Fatal(err)
				}
				if len(remove) > 0 {
					glog.Fatalf("You may not pass negative labels in %q", cfg.Labels)
				}
				label = valid
			}

			image := cfg.ImageTemplate.ExpandOrDie(cfg.Type)

			namespace, err := f.OpenShiftClientConfig.Namespace()
			if err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			_, kClient, err := f.Clients()
			if err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}

			p, output, err := cmdutil.PrinterForCommand(cmd)
			if err != nil {
				glog.Fatalf("Unable to configure printer: %v", err)
			}

			generate := output
			if !generate {
				_, err = kClient.Services(namespace).Get(name)
				if err != nil {
					if !errors.IsNotFound(err) {
						glog.Fatalf("Can't check for existing docker-registry %q: %v", name, err)
					}
					generate = true
				}
			}

			if generate {
				if !cfg.Create && !output {
					glog.Fatalf("Docker-registry %q does not exist (no service). Pass --create to install.", name)
				}

				// create new registry
				if len(cfg.Credentials) == 0 {
					glog.Fatalf("You must specify a .kubeconfig file path containing credentials for connecting the registry to the master with --credentials")
				}
				clientConfigLoadingRules := &kclientcmd.ClientConfigLoadingRules{cfg.Credentials, []string{}}
				credentials, err := clientConfigLoadingRules.Load()
				if err != nil {
					glog.Fatalf("The provided credentials %q could not be loaded: %v", cfg.Credentials, err)
				}
				config, err := kclientcmd.NewDefaultClientConfig(*credentials, &kclientcmd.ConfigOverrides{}).ClientConfig()
				if err != nil {
					glog.Fatalf("The provided credentials %q could not be used: %v", cfg.Credentials, err)
				}
				if err := kclient.LoadTLSFiles(config); err != nil {
					glog.Fatalf("The provided credentials %q could not load certificate info: %v", cfg.Credentials, err)
				}
				insecure := "false"
				if config.Insecure {
					insecure = "true"
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
								Privileged: mountHost,
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
						glog.Fatalf("Unable to print object: %v", err)
					}
					return
				}

				bulk := configcmd.Bulk{
					Factory: f.Factory,
					After:   configcmd.NewPrintNameOrErrorAfter(out, os.Stderr),
				}
				if errs := bulk.Create(list, namespace); len(errs) != 0 {
					os.Exit(1)
				}
				return
			}

			fmt.Fprintf(out, "Docker registry %q service exists", name)
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
	cmd.Flags().BoolVar(&cfg.Create, "create", cfg.Create, "Create the registry if it does not exist.")
	cmd.Flags().StringVar(&cfg.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the registry should use to contact the master.")

	cmdutil.AddPrinterFlags(cmd)

	return cmd
}
