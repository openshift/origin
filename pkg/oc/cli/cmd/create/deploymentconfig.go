package create

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	appsinternalversion "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
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
	DC     *appsapi.DeploymentConfig
	Client appsinternalversion.DeploymentConfigsGetter

	DryRun bool

	OutputFormat string
	Out          io.Writer
	Printer      ObjectPrinter
}

// NewCmdCreateDeploymentConfig is a macro command to create a new deployment config.
func NewCmdCreateDeploymentConfig(name, fullName string, f kcmdutil.Factory, out io.Writer) *cobra.Command {
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

func (o *CreateDeploymentConfigOptions) Complete(cmd *cobra.Command, f kcmdutil.Factory, args []string) error {
	argsLenAtDash := cmd.ArgsLenAtDash()
	switch {
	case (argsLenAtDash == -1 && len(args) != 1),
		(argsLenAtDash == 0),
		(argsLenAtDash > 1):
		return fmt.Errorf("NAME is required: %v", args)

	}

	labels := map[string]string{"deployment-config.name": args[0]}

	o.DryRun = cmdutil.GetFlagBool(cmd, "dry-run")
	o.DC = &appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Name: args[0]},
		Spec: appsapi.DeploymentConfigSpec{
			Selector: labels,
			Replicas: 1,
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
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
	o.DC.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	appsClient, err := appsclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.Client = appsClient.Apps()

	o.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	o.Printer = func(obj runtime.Object, out io.Writer) error {
		return cmdutil.PrintObject(cmd, obj, out)
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
		cmdutil.PrintSuccess(useShortOutput, o.Out, actualObj, o.DryRun, "created")
		return nil
	}

	return o.Printer(actualObj, o.Out)
}
