package deprecatedcerts

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/library-go/pkg/crypto"
)

type SignerCertOptions struct {
	CertFile   string
	KeyFile    string
	SerialFile string

	lock sync.Mutex
	ca   *crypto.CA
}

func BindSignerCertOptions(options *SignerCertOptions, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&options.CertFile, prefix+"signer-cert", options.CertFile, "The certificate file.")
	flags.StringVar(&options.KeyFile, prefix+"signer-key", options.KeyFile, "The key file.")
	flags.StringVar(&options.SerialFile, prefix+"signer-serial", options.SerialFile, "The serial file that keeps track of how many certs have been signed.")

	// autocompletion hints
	cobra.MarkFlagFilename(flags, prefix+"signer-cert")
	cobra.MarkFlagFilename(flags, prefix+"signer-key")
	cobra.MarkFlagFilename(flags, prefix+"signer-serial")
}

func NewDefaultSignerCertOptions() *SignerCertOptions {
	options := &SignerCertOptions{}
	options.CertFile = "openshift.local.config/master/ca.crt"
	options.KeyFile = "openshift.local.config/master/ca.key"
	options.SerialFile = "openshift.local.config/master/ca.serial.txt"

	return options
}

func (o *SignerCertOptions) Validate() error {
	if _, err := os.Stat(o.CertFile); len(o.CertFile) == 0 || err != nil {
		return fmt.Errorf("--signer-cert, %q must be a valid certificate file", getDisplayFilename(o.CertFile))
	}
	if _, err := os.Stat(o.KeyFile); len(o.KeyFile) == 0 || err != nil {
		return fmt.Errorf("--signer-key, %q must be a valid key file", getDisplayFilename(o.KeyFile))
	}
	if len(o.SerialFile) > 0 {
		if _, err := os.Stat(o.SerialFile); err != nil {
			return fmt.Errorf("--signer-serial, %q must be a valid file", getDisplayFilename(o.SerialFile))
		}
	}

	return nil
}

func (o *SignerCertOptions) CA() (*crypto.CA, error) {
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

// getDisplayFilename returns the absolute path of the filename as long as there was no error, otherwise it returns the filename as-is
func getDisplayFilename(filename string) string {
	if absName, err := filepath.Abs(filename); err == nil {
		return absName
	}

	return filename
}
