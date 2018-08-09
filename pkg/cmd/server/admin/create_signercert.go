package admin

import (
	"errors"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/library-go/pkg/crypto"
)

const CreateSignerCertCommandName = "create-signer-cert"

type CreateSignerCertOptions struct {
	CertFile   string
	KeyFile    string
	SerialFile string
	ExpireDays int
	Name       string

	Overwrite bool

	genericclioptions.IOStreams
}

var createSignerLong = templates.LongDesc(`
	Create a self-signed CA key/cert for signing certificates used by server components.`)

func NewCreateSignerCertOptions(streams genericclioptions.IOStreams) *CreateSignerCertOptions {
	return &CreateSignerCertOptions{
		ExpireDays: crypto.DefaultCACertificateLifetimeInDays,
		Overwrite:  true,
		CertFile:   "openshift.local.config/master/ca.crt",
		KeyFile:    "openshift.local.config/master/ca.key",
		SerialFile: "openshift.local.config/master/ca.serial.txt",
		Name:       DefaultSignerName(),
		IOStreams:  streams,
	}
}

func NewCommandCreateSignerCert(commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateSignerCertOptions(streams)
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a signer (certificate authority/CA) certificate and key",
		Long:  createSignerLong,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Validate(args))
			if _, err := o.CreateSignerCert(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	prefix := ""
	cmd.Flags().StringVar(&o.CertFile, prefix+"cert", o.CertFile, "The certificate file.")
	cmd.Flags().StringVar(&o.KeyFile, prefix+"key", o.KeyFile, "The key file.")
	cmd.Flags().StringVar(&o.SerialFile, prefix+"serial", o.SerialFile, "The serial file that keeps track of how many certs have been signed.")
	cmd.Flags().StringVar(&o.Name, prefix+"name", o.Name, "The name of the signer.")
	cmd.Flags().BoolVar(&o.Overwrite, prefix+"overwrite", o.Overwrite, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

	cmd.Flags().IntVar(&o.ExpireDays, "expire-days", o.ExpireDays, "Validity of the certificate in days (defaults to 5 years). WARNING: extending this above default value is highly discouraged.")

	// set dynamic value annotation - allows man pages  to be generated and verified
	cmd.Flags().SetAnnotation(prefix+"name", "manpage-def-value", []string{"openshift-signer@<current_timestamp>"})

	// autocompletion hints
	cobra.MarkFlagFilename(cmd.Flags(), prefix+"cert")
	cobra.MarkFlagFilename(cmd.Flags(), prefix+"key")
	cobra.MarkFlagFilename(cmd.Flags(), prefix+"serial")

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
