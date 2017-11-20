package admin

import (
	"errors"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

const CreateSignerCertCommandName = "create-signer-cert"

type CreateSignerCertOptions struct {
	CertFile   string
	KeyFile    string
	SerialFile string
	ExpireDays int
	Name       string
	Output     io.Writer

	Overwrite bool
}

func BindCreateSignerCertOptions(options *CreateSignerCertOptions, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&options.CertFile, prefix+"cert", "openshift.local.config/master/ca.crt", "The certificate file.")
	flags.StringVar(&options.KeyFile, prefix+"key", "openshift.local.config/master/ca.key", "The key file.")
	flags.StringVar(&options.SerialFile, prefix+"serial", "openshift.local.config/master/ca.serial.txt", "The serial file that keeps track of how many certs have been signed.")
	flags.StringVar(&options.Name, prefix+"name", DefaultSignerName(), "The name of the signer.")
	flags.BoolVar(&options.Overwrite, prefix+"overwrite", options.Overwrite, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificate in days (defaults to 5 years). WARNING: extending this above default value is highly discouraged.")

	// set dynamic value annotation - allows man pages  to be generated and verified
	flags.SetAnnotation(prefix+"name", "manpage-def-value", []string{"openshift-signer@<current_timestamp>"})

	// autocompletion hints
	cobra.MarkFlagFilename(flags, prefix+"cert")
	cobra.MarkFlagFilename(flags, prefix+"key")
	cobra.MarkFlagFilename(flags, prefix+"serial")
}

var createSignerLong = templates.LongDesc(`
	Create a self-signed CA key/cert for signing certificates used by server components.`)

func NewCommandCreateSignerCert(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateSignerCertOptions{
		ExpireDays: crypto.DefaultCACertificateLifetimeInDays,
		Output:     out,
		Overwrite:  true,
	}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a signer (certificate authority/CA) certificate and key",
		Long:  createSignerLong,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if _, err := options.CreateSignerCert(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	BindCreateSignerCertOptions(options, cmd.Flags(), "")

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
	if o.ExpireDays <= 0 {
		return errors.New("expire-days must be valid number of days")
	}
	if len(o.Name) == 0 {
		return errors.New("name must be provided")
	}

	return nil
}

func (o CreateSignerCertOptions) CreateSignerCert() (*crypto.CA, error) {
	glog.V(4).Infof("Creating a signer cert with: %#v", o)
	var ca *crypto.CA
	var err error
	written := true
	if o.Overwrite {
		ca, err = crypto.MakeCA(o.CertFile, o.KeyFile, o.SerialFile, o.Name, o.ExpireDays)
	} else {
		ca, written, err = crypto.EnsureCA(o.CertFile, o.KeyFile, o.SerialFile, o.Name, o.ExpireDays)
	}
	if written {
		glog.V(3).Infof("Generated new CA for %s: cert in %s and key in %s\n", o.Name, o.CertFile, o.KeyFile)
	} else {
		glog.V(3).Infof("Keeping existing CA cert at %s and key at %s\n", o.CertFile, o.KeyFile)
	}
	return ca, err
}
