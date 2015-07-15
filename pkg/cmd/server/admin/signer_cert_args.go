package admin

import (
	"errors"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type SignerCertOptions struct {
	CertFile   string
	KeyFile    string
	SerialFile string

	lock sync.Mutex
	ca   *crypto.CA
}

func BindSignerCertOptions(options *SignerCertOptions, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&options.CertFile, prefix+"signer-cert", "openshift.local.config/master/ca.crt", "The certificate file.")
	flags.StringVar(&options.KeyFile, prefix+"signer-key", "openshift.local.config/master/ca.key", "The key file.")
	flags.StringVar(&options.SerialFile, prefix+"signer-serial", "openshift.local.config/master/ca.serial.txt", "The serial file that keeps track of how many certs have been signed.")

	// autocompletion hints
	cobra.MarkFlagFilename(flags, prefix+"signer-cert")
	cobra.MarkFlagFilename(flags, prefix+"signer-key")
	cobra.MarkFlagFilename(flags, prefix+"signer-serial")
}

func (o SignerCertOptions) Validate() error {
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

func (o SignerCertOptions) CA() (*crypto.CA, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	if o.ca != nil {
		return o.ca, nil
	}
	ca, err := crypto.GetCA(o.CertFile, o.KeyFile, o.SerialFile)
	if err != nil {
		return nil, err
	}
	o.ca = ca
	return ca, nil
}
