package create

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/create"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
)

// CreateSubcommandOptions is an options struct to support create subcommands
type CreateSubcommandOptions struct {
	genericclioptions.IOStreams

	// PrintFlags holds options necessary for obtaining a printer
	PrintFlags *create.PrintFlags
	// Name of resource being created
	Name string
	// DryRun is true if the command should be simulated but not run against the server
	DryRun bool

	Namespace        string
	EnforceNamespace bool

	Printer printers.ResourcePrinter
}

func NewCreateSubcommandOptions(ioStreams genericclioptions.IOStreams) *CreateSubcommandOptions {
	return &CreateSubcommandOptions{
		PrintFlags: create.NewPrintFlags("created", legacyscheme.Scheme),
		IOStreams:  ioStreams,
	}
}

func (o *CreateSubcommandOptions) Complete(f genericclioptions.RESTClientGetter, cmd *cobra.Command, args []string) error {
	name, err := NameFromCommandArgs(args)
	if err != nil {
		return err
	}

	o.Name = name
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DryRun = cmdutil.GetDryRunFlag(cmd)
	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	return nil
}

// NameFromCommandArgs is a utility function for commands that assume the first argument is a resource name
func NameFromCommandArgs(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("exactly one NAME is required, got %d", len(args))
	}
	return args[0], nil
}
