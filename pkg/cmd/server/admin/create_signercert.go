package admin

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type CreateSignerCertOptions struct {
	CertFile   string
	KeyFile    string
	SerialFile string
	Name       string

	Overwrite bool
}

func BindSignerCertOptions(options *CreateSignerCertOptions, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&options.CertFile, prefix+"cert", "openshift.local.certificates/ca/cert.crt", "The certificate file.")
	flags.StringVar(&options.KeyFile, prefix+"key", "openshift.local.certificates/ca/key.key", "The key file.")
	flags.StringVar(&options.SerialFile, prefix+"serial", "openshift.local.certificates/ca/serial.txt", "The serial file that keeps track of how many certs have been signed.")
	flags.StringVar(&options.Name, prefix+"name", DefaultSignerName(), "The name of the signer.")
	flags.BoolVar(&options.Overwrite, prefix+"overwrite", options.Overwrite, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")
}

func NewCommandCreateSignerCert() *cobra.Command {
	options := &CreateSignerCertOptions{Overwrite: true}

	cmd := &cobra.Command{
		Use:   "create-signer-cert",
		Short: "Create signer certificate",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			if _, err := options.CreateSignerCert(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	BindSignerCertOptions(options, cmd.Flags(), "")

	return cmd
}

func (o CreateSignerCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.CertFile) == 0 {
		return errors.New("cert must be provided")
	}
	if len(o.KeyFile) == 0 {
		return errors.New("key must be provided")
	}
	if len(o.SerialFile) == 0 {
		return errors.New("serial must be provided")
	}
	if len(o.Name) == 0 {
		return errors.New("name must be provided")
	}

	return nil
}

func (o CreateSignerCertOptions) CreateSignerCert() (*crypto.CA, error) {
	glog.V(2).Infof("Creating a signer cert with: %#v", o)

	if o.Overwrite {
		return crypto.MakeCA(o.CertFile, o.KeyFile, o.SerialFile, o.Name)
	} else {
		return crypto.EnsureCA(o.CertFile, o.KeyFile, o.SerialFile, o.Name)
	}
}
