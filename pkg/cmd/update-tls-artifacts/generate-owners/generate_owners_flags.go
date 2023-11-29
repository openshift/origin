package generate_owners

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatadefaults"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// GenerateOwnersFlags gets bound to cobra commands and arguments.  It is used to validate input and then produce
// the Options struct.  Options struct is intended to be embeddable and re-useable without cobra.
type GenerateOwnersFlags struct {
	TLSInfoDir string
	Verify     bool

	genericclioptions.IOStreams
}

func NewGenerateOwnershipCommand(streams genericclioptions.IOStreams) *cobra.Command {
	f := NewGenerateOwnersFlags(streams)

	cmd := &cobra.Command{
		Use:           "generate-ownership",
		Short:         "Generate ownership json and markdown files.",
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
			return o.Run()
		},
	}

	f.BindFlags(cmd.Flags())

	return cmd
}

func NewGenerateOwnersFlags(streams genericclioptions.IOStreams) *GenerateOwnersFlags {
	return &GenerateOwnersFlags{
		TLSInfoDir: "tls",
		IOStreams:  streams,
	}
}

func (f *GenerateOwnersFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.TLSInfoDir, "ownership-dir", f.TLSInfoDir, "The directory where the  TLS ownership info is should be written.")
	flags.BoolVar(&f.Verify, "verify", f.Verify, "Verify content, don't mutate.")
}

func (f *GenerateOwnersFlags) Validate() error {
	if len(f.TLSInfoDir) == 0 {
		return fmt.Errorf("--ownership-dir must be specified")
	}
	return nil
}

func (f *GenerateOwnersFlags) ToOptions() (*GenerateOwnersOptions, error) {
	return &GenerateOwnersOptions{
		TLSInfoDir:   f.TLSInfoDir,
		Verify:       f.Verify,
		Requirements: tlsmetadatadefaults.GetDefaultTLSRequirements(),

		IOStreams: f.IOStreams,
	}, nil
}
