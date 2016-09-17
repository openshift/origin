package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

var (
	internalTYPELong = templates.LongDesc(`
		Single line title

		Description body`)

	internalTYPEExample = templates.Examples(`%s`)
)

type TYPEOptions struct {
	In          io.Reader
	Out, ErrOut io.Writer
}

// NewCmdTYPE implements a TYPE command
// This is an example type for templating.
func NewCmdTYPE(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &TYPEOptions{
		In:     in,
		Out:    out,
		ErrOut: errout,
	}
	cmd := &cobra.Command{
		Use:     "NAME [...]",
		Short:   "A short description",
		Long:    internalTYPELong,
		Example: fmt.Sprintf(internalTYPEExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			if err := options.Run(); err != nil {
				// TODO: move met to kcmdutil
				if err == cmdutil.ErrExit {
					os.Exit(1)
				}
				kcmdutil.CheckErr(err)
			}
		},
	}
	return cmd
}

func (o *TYPEOptions) Complete(f *clientcmd.Factory, c *cobra.Command, args []string) error {
	return nil
}

func (o *TYPEOptions) Validate() error { return nil }
func (o *TYPEOptions) Run() error      { return nil }
