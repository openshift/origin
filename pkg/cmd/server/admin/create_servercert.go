package admin

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

const CreateServerCertCommandName = "create-server-cert"

type CreateServerCertOptions struct {
	SignerCertOptions *SignerCertOptions

	CertFile string
	KeyFile  string

	ExpireDays int

	Hostnames []string
	Overwrite bool
	Output    io.Writer
}

var createServerLong = templates.LongDesc(`
	Create a key and server certificate

	Create a key and server certificate valid for the specified hostnames,
	signed by the specified CA. These are useful for securing infrastructure
	components such as the router, authentication server, etc.

	Example: Creating a secure router certificate.

	    CA=openshift.local.config/master
			%[1]s --signer-cert=$CA/ca.crt \
		          --signer-key=$CA/ca.key --signer-serial=$CA/ca.serial.txt \
		          --hostnames='*.cloudapps.example.com' \
		          --cert=cloudapps.crt --key=cloudapps.key
	    cat cloudapps.crt cloudapps.key $CA/ca.crt > cloudapps.router.pem
	`)

func NewCommandCreateServerCert(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateServerCertOptions{
		SignerCertOptions: NewDefaultSignerCertOptions(),
		ExpireDays:        crypto.DefaultCertificateLifetimeInDays,
		Output:            out,
	}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a signed server certificate and key",
		Long:  fmt.Sprintf(createServerLong, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if _, err := options.CreateServerCert(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()
	BindSignerCertOptions(options.SignerCertOptions, flags, "")

	flags.StringVar(&options.CertFile, "cert", "", "The certificate file. Choose a name that indicates what the service is.")
	flags.StringVar(&options.KeyFile, "key", "", "The key file. Choose a name that indicates what the service is.")

	flags.StringSliceVar(&options.Hostnames, "hostnames", options.Hostnames, "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	flags.BoolVar(&options.Overwrite, "overwrite", true, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificate in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")

	// autocompletion hints
	cmd.MarkFlagFilename("cert")
	cmd.MarkFlagFilename("key")

	return cmd
}

func (o CreateServerCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.Hostnames) == 0 {
		return errors.New("at least one hostname must be provided")
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

	if o.SignerCertOptions == nil {
		return errors.New("signer options are required")
	}
	if err := o.SignerCertOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func (o CreateServerCertOptions) CreateServerCert() (*crypto.TLSCertificateConfig, error) {
	glog.V(4).Infof("Creating a server cert with: %#v", o)

	signerCert, err := o.SignerCertOptions.CA()
	if err != nil {
		return nil, err
	}

	var ca *crypto.TLSCertificateConfig
	written := true
	if o.Overwrite {
		ca, err = signerCert.MakeAndWriteServerCert(o.CertFile, o.KeyFile, sets.NewString([]string(o.Hostnames)...), o.ExpireDays)
	} else {
		ca, written, err = signerCert.EnsureServerCert(o.CertFile, o.KeyFile, sets.NewString([]string(o.Hostnames)...), o.ExpireDays)
	}
	if written {
		glog.V(3).Infof("Generated new server certificate as %s, key as %s\n", o.CertFile, o.KeyFile)
	} else {
		glog.V(3).Infof("Keeping existing server certificate at %s, key at %s\n", o.CertFile, o.KeyFile)
	}
	return ca, err
}
