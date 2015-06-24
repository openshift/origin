package admin

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type GetSignerCertOptions struct {
	CertFile   string
	KeyFile    string
	SerialFile string
}

func BindGetSignerCertOptions(options *GetSignerCertOptions, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&options.CertFile, prefix+"signer-cert", "openshift.local.config/master/ca.crt", "The certificate file.")
	flags.StringVar(&options.KeyFile, prefix+"signer-key", "openshift.local.config/master/ca.key", "The key file.")
	flags.StringVar(&options.SerialFile, prefix+"signer-serial", "openshift.local.config/master/ca.serial.txt", "The serial file that keeps track of how many certs have been signed.")

	// autocompletion hints
	cobra.MarkFlagFilename(flags, prefix+"signer-cert")
	cobra.MarkFlagFilename(flags, prefix+"signer-key")
	cobra.MarkFlagFilename(flags, prefix+"signer-serial")
}

func (o GetSignerCertOptions) Validate() error {
	if len(o.CertFile) == 0 {
		return errors.New("signer-cert must be provided")
	}
	if len(o.KeyFile) == 0 {
		return errors.New("signer-key must be provided")
	}
	if len(o.SerialFile) == 0 {
		return errors.New("signer-serial must be provided")
	}

	return nil
}

func (o GetSignerCertOptions) GetSignerCert() (*crypto.CA, error) {
	return crypto.GetCA(o.CertFile, o.KeyFile, o.SerialFile)
}
