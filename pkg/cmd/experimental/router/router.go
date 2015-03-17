package router

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
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	dapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
)

const longDesc = `
Install or configure an OpenShift router

This command helps to setup an OpenShift router to take edge traffic and balance it to
your application. With no arguments, the command will check for an existing router
service called 'router' and perform some diagnostics to ensure the router is properly
configured and functioning.

If a router does not exist with the given name, the --create flag can be passed to
create a deployment configuration and service that will run the router. If you are
running your router in production, you should pass --replicas=2 or higher to ensure
you have failover protection.

Examples:
  Check the default router ("router"):

  $ %[1]s %[2]s

  See what the router would look like if created:

  $ %[1]s %[2]s -o json

  Create a router if it does not exist:

  $ %[1]s %[2]s router-west --create --replicas=2

  Use a different router image and see the router configuration:

  $ %[1]s %[2]s region-west -o yaml --images=myrepo/somerouter:mytag

ALPHA: This command is currently being actively developed. It is intended to simplify
  the tasks of setting up routers in a new installation.
`

type config struct {
	Type          string
	ImageTemplate variable.ImageTemplate
	Ports         string
	Replicas      int
	Labels        string
	Create        bool
	Credentials   string
}

const defaultLabel = "router=<name>"

func NewCmdRouter(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &config{
		ImageTemplate: variable.NewDefaultImageTemplate(),

		Labels:   defaultLabel,
		Ports:    "80:80,443:443",
		Replicas: 1,
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [<name>]", name),
		Short: "Install and check OpenShift routers",
		Long:  fmt.Sprintf(longDesc, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			var name string
			switch len(args) {
			case 0:
				name = "router"
			case 1:
				name = args[0]
			default:
				glog.Fatalf("You may pass zero or one arguments to provide a name for the router")
			}

			ports, err := app.ContainerPortsFromString(cfg.Ports)
			if err != nil {
				glog.Fatal(err)
			}

			label := map[string]string{"router": name}
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
			_, kClient, err := f.Clients(cmd)
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
						glog.Fatalf("Can't check for existing router %q: %v", name, err)
					}
					generate = true
				}
			}

			if generate {
				if !cfg.Create && !output {
					glog.Fatalf("Router %q does not exist (no service). Pass --create to install.", name)
				}

				// create new router
				if len(cfg.Credentials) == 0 {
					glog.Fatalf("You must specify a .kubeconfig file path containing credentials for connecting the router to the master with --credentials")
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
								Template: &kapi.PodTemplateSpec{
									ObjectMeta: kapi.ObjectMeta{Labels: label},
									Spec: kapi.PodSpec{
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
												},
											},
										},
									},
								},
							},
						},
					},
				}
				objects = app.AddServices(objects)
				// TODO: label all created objects with the same label - router=<name>
				list := &kapi.List{Items: objects}

				if output {
					if err := p.PrintObj(list, out); err != nil {
						glog.Fatalf("Unable to print object: %v", err)
					}
					return
				}

				bulk := configcmd.Bulk{
					Factory: f.Factory,
					Command: cmd,
					After:   configcmd.NewPrintNameOrErrorAfter(out, os.Stderr),
				}
				if errs := bulk.Create(list, namespace); len(errs) != 0 {
					os.Exit(1)
				}
				return
			}

			fmt.Fprintf(out, "Router %q service exists", name)
		},
	}

	cmd.Flags().StringVar(&cfg.Type, "type", "haproxy-router", "The type of router to use - if you specify --images this flag may be ignored.")
	cmd.Flags().StringVar(&cfg.ImageTemplate.Format, "images", cfg.ImageTemplate.Format, "The image to base this router on - ${component} will be replaced with --type")
	cmd.Flags().BoolVar(&cfg.ImageTemplate.Latest, "latest-images", cfg.ImageTemplate.Latest, "If true, attempt to use the latest images for the router instead of the latest release.")
	cmd.Flags().StringVar(&cfg.Ports, "ports", cfg.Ports, "A comma delimited list of ports or port pairs to expose on the router pod. The default is set for HAProxy.")
	cmd.Flags().IntVar(&cfg.Replicas, "replicas", cfg.Replicas, "The replication factor of the router; commonly 2 when high availability is desired.")
	cmd.Flags().StringVar(&cfg.Labels, "labels", cfg.Labels, "A set of labels to uniquely identify the router and its components.")
	cmd.Flags().BoolVar(&cfg.Create, "create", cfg.Create, "Create the router if it does not exist.")
	cmd.Flags().StringVar(&cfg.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the router should use to contact the master.")

	cmdutil.AddPrinterFlags(cmd)

	return cmd
}

/*
// Example with generation - this does not have port metadata so its slightly less
// clear to end users.

registry, imageNamespace, imageName, tag, err := imageapi.SplitDockerPullSpec(image)
if err != nil {
	glog.Fatalf("The image value %q is not valid: %v", image, err)
}

image := &app.ImageRef{
	Namespace: imageNamespace,
	Name:      imageName,
	Registry:  registry,
	Tag:       tag,
}
pipeline, err := app.NewImagePipeline(name, image)
if err != nil {
	glog.Fatalf("Unable to set up an image for the router: %v", err)
}
if err := pipeline.NeedsDeployment(nil); err != nil {
	glog.Fatalf("Unable to set up a deployment for the router: %v", err)
}
objects, err := pipeline.Objects(app.NewAcceptFirst())
if err != nil {
	glog.Fatalf("Unable to configure objects for deployment: %v", err)
}
objects = app.AddServices(objects)
*/
