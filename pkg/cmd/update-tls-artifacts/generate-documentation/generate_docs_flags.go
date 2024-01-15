package generate_documentation

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// GenerateDocumentationFlags gets bound to cobra commands and arguments.  It is used to validate input and then produce
// the Options struct.  Options struct is intended to be embeddable and re-useable without cobra.
type GenerateDocumentationFlags struct {
	TLSInfoDir string
	Verify     bool

	genericclioptions.IOStreams
}

func NewGenerateDocumentationCommand(streams genericclioptions.IOStreams) *cobra.Command {
	f := NewGenerateDocumentationFlags(streams)

	cmd := &cobra.Command{
		Use:           "generate-documentation",
		Short:         "Generate documentation for TLS artifacts.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := f.Validate()
			if err != nil {
				return err
			}

			o, err := f.ToOptions()
			if err != nil {
				return err
			}
			return o.Run(context.Background())
		},
	}

	f.BindFlags(cmd.Flags())

	return cmd
}

func NewGenerateDocumentationFlags(streams genericclioptions.IOStreams) *GenerateDocumentationFlags {
	return &GenerateDocumentationFlags{
		TLSInfoDir: "tls",
		IOStreams:  streams,
	}
}

func (f *GenerateDocumentationFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.TLSInfoDir, "tls-dir", f.TLSInfoDir, "The directory where the  TLS ownership info is should be written.")
	flags.BoolVar(&f.Verify, "verify", f.Verify, "Verify content, don't mutate.")
}

func (f *GenerateDocumentationFlags) Validate() error {
	if len(f.TLSInfoDir) == 0 {
		return fmt.Errorf("--tls-dir must be specified")
	}

	return nil
}

func (f *GenerateDocumentationFlags) ToOptions() (*GenerateDocumentationOptions, error) {
	return &GenerateDocumentationOptions{
		TLSInfoDir: f.TLSInfoDir,
		Verify:     f.Verify,

		IOStreams: f.IOStreams,
	}, nil
}
