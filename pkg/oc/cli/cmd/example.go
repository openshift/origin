package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"

	"github.com/openshift/origin/pkg/oc/util/ocscheme"
)

var (
	internalTYPELong = templates.LongDesc(`
		Single line title

		Description body`)

	internalTYPEExample = templates.Examples(`%s`)
)

type TYPEOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Printer printers.ResourcePrinter
	DryRun  bool

	genericclioptions.IOStreams
}

// NewTYPEOptions returns a TYPEOptions with proper defaults.
// This is an example type for templating.
func NewTYPEOptions(streams genericclioptions.IOStreams) *TYPEOptions {
	return &TYPEOptions{
		PrintFlags: genericclioptions.NewPrintFlags("action performed").WithTypeSetter(ocscheme.PrintingInternalScheme),
		IOStreams:  streams,
	}
}

// NewCmdTYPE implements a TYPE command
// This is an example type for templating.
func NewCmdTYPE(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewTYPEOptions(streams)
	cmd := &cobra.Command{
		Use:     "NAME [...]",
		Short:   "A short description",
		Long:    internalTYPELong,
		Example: fmt.Sprintf(internalTYPEExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *TYPEOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
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

func (o *TYPEOptions) Validate() error { return nil }
func (o *TYPEOptions) Run() error      { return nil }
