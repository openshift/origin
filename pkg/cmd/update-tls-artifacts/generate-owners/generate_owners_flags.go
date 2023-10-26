package generate_owners

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// GenerateOwnersFlags gets bound to cobra commands and arguments.  It is used to validate input and then produce
// the Options struct.  Options struct is intended to be embeddable and re-useable without cobra.
type GenerateOwnersFlags struct {
	RawTLSInfoDir       string
	TLSOwnershipInfoDir string
	Verify              bool

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
		RawTLSInfoDir:       "tls/raw-data",
		TLSOwnershipInfoDir: "tls/ownership",
		IOStreams:           streams,
	}
}

func (f *GenerateOwnersFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.RawTLSInfoDir, "raw-tls-info-dir", f.RawTLSInfoDir, "The directory where the raw TLS info is located.")
	flags.StringVar(&f.TLSOwnershipInfoDir, "ownership-info-dir", f.TLSOwnershipInfoDir, "The directory where the  TLS ownership info is should be written.")
	flags.BoolVar(&f.Verify, "verify", f.Verify, "Verify content, don't mutate.")
}

func (f *GenerateOwnersFlags) Validate() error {
	if len(f.RawTLSInfoDir) == 0 {
		return fmt.Errorf("--raw-tls-info-dir must be specified")
	}
	if len(f.TLSOwnershipInfoDir) == 0 {
		return fmt.Errorf("--ownership-info-dir must be specified")
	}
	return nil
}

func (f *GenerateOwnersFlags) ToOptions() (*GenerateOwnersOptions, error) {
	return &GenerateOwnersOptions{
		RawTLSInfoDir:       f.RawTLSInfoDir,
		TLSOwnershipInfoDir: f.TLSOwnershipInfoDir,
		Verify:              f.Verify,
		IOStreams:           f.IOStreams,
	}, nil
}
