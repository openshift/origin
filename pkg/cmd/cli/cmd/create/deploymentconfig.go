package create

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
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
	DC     *deployapi.DeploymentConfig
	Client client.DeploymentConfigsNamespacer

	DryRun bool

	Mapper       meta.RESTMapper
	OutputFormat string
	Out          io.Writer
	Printer      ObjectPrinter
}

// NewCmdCreateServiceAccount is a macro command to create a new service account
func NewCmdCreateDeploymentConfig(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &CreateDeploymentConfigOptions{Out: out}

	cmd := &cobra.Command{
		Use:     name + " NAME --image=IMAGE -- [COMMAND] [args...]",
		Short:   "Create deployment config with default options that uses a given image.",
		Long:    deploymentConfigLong,
		Example: fmt.Sprintf(deploymentConfigExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		Aliases: []string{"dc"},
	}

	cmd.Flags().String("image", "", "The image for the container to run.")
	cmd.MarkFlagRequired("image")
	cmdutil.AddDryRunFlag(cmd)
	cmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *CreateDeploymentConfigOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	argsLenAtDash := cmd.ArgsLenAtDash()
	switch {
	case (argsLenAtDash == -1 && len(args) != 1),
		(argsLenAtDash == 0),
		(argsLenAtDash > 1):
		return fmt.Errorf("NAME is required: %v", args)

	}

	labels := map[string]string{"deployment-config.name": args[0]}

	o.DryRun = cmdutil.GetFlagBool(cmd, "dry-run")
	o.DC = &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: args[0]},
		Spec: deployapi.DeploymentConfigSpec{
			Selector: labels,
			Replicas: 1,
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{Labels: labels},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "default-container",
							Image: cmdutil.GetFlagString(cmd, "image"),
							Args:  args[1:],
						},
					},
				},
			},
		},
	}

	var err error
	o.DC.Namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Client, _, err = f.Clients()
	if err != nil {
		return err
	}

	o.Mapper, _ = f.Object(false)
	o.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	o.Printer = func(obj runtime.Object, out io.Writer) error {
		return f.PrintObject(cmd, o.Mapper, obj, out)
	}

	return nil
}

func (o *CreateDeploymentConfigOptions) Validate() error {
	if o.DC == nil {
		return fmt.Errorf("DC is required")
	}
	if o.Client == nil {
		return fmt.Errorf("Client is required")
	}
	if o.Mapper == nil {
		return fmt.Errorf("Mapper is required")
	}
	if o.Out == nil {
		return fmt.Errorf("Out is required")
	}
	if o.Printer == nil {
		return fmt.Errorf("Printer is required")
	}

	return nil
}

func (o *CreateDeploymentConfigOptions) Run() error {
	actualObj := o.DC

	var err error
	if !o.DryRun {

		actualObj, err = o.Client.DeploymentConfigs(o.DC.Namespace).Create(o.DC)
		if err != nil {
			return err
		}
	}

	if useShortOutput := o.OutputFormat == "name"; useShortOutput || len(o.OutputFormat) == 0 {
		cmdutil.PrintSuccess(o.Mapper, useShortOutput, o.Out, "deploymentconfig", actualObj.Name, o.DryRun, "created")
		return nil
	}

	return o.Printer(actualObj, o.Out)
}
