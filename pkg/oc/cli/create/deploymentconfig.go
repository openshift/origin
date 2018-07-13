package create

import (
	"fmt"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	appsv1 "github.com/openshift/api/apps/v1"
	appsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
)

var DeploymentConfigRecommendedName = "deploymentconfig"

var (
	deploymentConfigLong = templates.LongDesc(`
		Create a deployment config that uses a given image.

		Deployment configs define the template for a pod and manages deploying new images or configuration changes.`)

	deploymentConfigExample = templates.Examples(`
		# Create an nginx deployment config named my-nginx
  	%[1]s my-nginx --image=nginx`)
)

type CreateDeploymentConfigOptions struct {
	CreateSubcommandOptions *CreateSubcommandOptions

	Image string
	Args  []string

	Client appsv1client.DeploymentConfigsGetter
}

// NewCmdCreateDeploymentConfig is a macro command to create a new deployment config.
func NewCmdCreateDeploymentConfig(name, fullName string, f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateDeploymentConfigOptions{
		CreateSubcommandOptions: NewCreateSubcommandOptions(streams),
	}
	cmd := &cobra.Command{
		Use:     name + " NAME --image=IMAGE -- [COMMAND] [args...]",
		Short:   "Create deployment config with default options that uses a given image.",
		Long:    deploymentConfigLong,
		Example: fmt.Sprintf(deploymentConfigExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Run())
		},
		Aliases: []string{"dc"},
	}
	cmd.Flags().StringVar(&o.Image, "image", o.Image, "The image for the container to run.")
	cmd.MarkFlagRequired("image")

	o.CreateSubcommandOptions.PrintFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *CreateDeploymentConfigOptions) Complete(cmd *cobra.Command, f genericclioptions.RESTClientGetter, args []string) error {
	if len(args) > 1 {
		o.Args = args[1:]
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = appsv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return o.CreateSubcommandOptions.Complete(f, cmd, args)
}

func (o *CreateDeploymentConfigOptions) Run() error {
	labels := map[string]string{"deployment-config.name": o.CreateSubcommandOptions.Name}
	deploymentConfig := &appsv1.DeploymentConfig{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "DeploymentConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: o.CreateSubcommandOptions.Name},
		Spec: appsv1.DeploymentConfigSpec{
			Selector: labels,
			Replicas: 1,
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "default-container",
							Image: o.Image,
							Args:  o.Args,
						},
					},
				},
			},
		},
	}

	if !o.CreateSubcommandOptions.DryRun {
		var err error
		deploymentConfig, err = o.Client.DeploymentConfigs(o.CreateSubcommandOptions.Namespace).Create(deploymentConfig)
		if err != nil {
			return err
		}
	}

	return o.CreateSubcommandOptions.Printer.PrintObj(deploymentConfig, o.CreateSubcommandOptions.Out)
}
