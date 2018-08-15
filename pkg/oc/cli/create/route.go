package create

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"

	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/origin/pkg/oc/util/ocscheme"
)

var (
	routeLong = templates.LongDesc(`
		Expose containers externally via secured routes

		Three types of secured routes are supported: edge, passthrough, and reencrypt.
		If you wish to create unsecured routes, see "%[1]s expose -h"`)
)

// NewCmdCreateRoute is a macro command to create a secured route.
func NewCmdCreateRoute(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Expose containers externally via secured routes",
		Long:  fmt.Sprintf(routeLong, fullName),
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	cmd.AddCommand(NewCmdCreateEdgeRoute(fullName, f, streams))
	cmd.AddCommand(NewCmdCreatePassthroughRoute(fullName, f, streams))
	cmd.AddCommand(NewCmdCreateReencryptRoute(fullName, f, streams))

	return cmd
}

// CreateRouteSubcommandOptions is an options struct to support create subcommands
type CreateRouteSubcommandOptions struct {
	// PrintFlags holds options necessary for obtaining a printer
	PrintFlags *genericclioptions.PrintFlags
	// Name of resource being created
	Name        string
	ServiceName string
	// DryRun is true if the command should be simulated but not run against the server
	DryRun bool

	Namespace        string
	EnforceNamespace bool

	Mapper meta.RESTMapper

	Printer printers.ResourcePrinter

	Client     routev1client.RoutesGetter
	CoreClient corev1client.CoreV1Interface

	genericclioptions.IOStreams
}

func NewCreateRouteSubcommandOptions(ioStreams genericclioptions.IOStreams) *CreateRouteSubcommandOptions {
	return &CreateRouteSubcommandOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(ocscheme.PrintingInternalScheme),
		IOStreams:  ioStreams,
	}
}

func (o *CreateRouteSubcommandOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.Name, err = resolveRouteName(args)
	if err != nil {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.CoreClient, err = corev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.Client, err = routev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	return nil
}

func resolveRouteName(args []string) (string, error) {
	switch len(args) {
	case 0:
	case 1:
		return args[0], nil
	default:
		return "", fmt.Errorf("multiple names provided. Please specify at most one")
	}
	return "", nil
}
