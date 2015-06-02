package admin

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

const CreateServerCertCommandName = "create-server-cert"

type CreateServerCertOptions struct {
	GetSignerCertOptions *GetSignerCertOptions

	CertFile string
	KeyFile  string

	Hostnames util.StringList
	Overwrite bool
	ClientAuth bool
	Output    cmdutil.Output
}

const create_server_long = `
Create a key and server certificate valid for the specified hostnames,
signed by the specified CA. These are useful for securing infrastructure
components such as the router, authentication server, etc.

Example: Creating a secure router certificate.

    $ CA=openshift.local.config/master
	$ %[1]s --signer-cert=$CA/ca.crt \
	          --signer-key=$CA/ca.key --signer-serial=$CA/ca.serial.txt \
	          --hostnames='*.cloudapps.example.com' \
	          --cert=cloudapps.crt --key=cloudapps.key
    $ cat cloudapps.crt cloudapps.key $CA/ca.crt > cloudapps.router.pem
`

func NewCommandCreateServerCert(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateServerCertOptions{GetSignerCertOptions: &GetSignerCertOptions{}, Output: cmdutil.Output{out}}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a signed server certificate and key",
		Long:  fmt.Sprintf(create_server_long, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if _, err := options.CreateServerCert(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	BindGetSignerCertOptions(options.GetSignerCertOptions, flags, "")

	flags.StringVar(&options.CertFile, "cert", "", "The certificate file. Choose a name that indicates what the service is.")
	flags.StringVar(&options.KeyFile, "key", "", "The key file. Choose a name that indicates what the service is.")

	flags.Var(&options.Hostnames, "hostnames", "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	flags.BoolVar(&options.Overwrite, "overwrite", true, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")
	flags.BoolVar(&options.ClientAuth, "clientauth", true, "Generate a certificate that's valid for both Client and Server uses.")

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

	if o.GetSignerCertOptions == nil {
		return errors.New("signer options are required")
	}
	if err := o.GetSignerCertOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func (o CreateServerCertOptions) CreateServerCert() (*crypto.TLSCertificateConfig, error) {
	glog.V(2).Infof("Creating a server cert with: %#v", o)

	signerCert, err := o.GetSignerCertOptions.GetSignerCert()
	if err != nil {
		return nil, err
	}

	var ca *crypto.TLSCertificateConfig
	written := true
	if o.Overwrite {
		if o.ClientAuth {
			ca, err = signerCert.MakeClientServerCert(o.CertFile, o.KeyFile, util.NewStringSet([]string(o.Hostnames)...))
		} else {
			ca, err = signerCert.MakeServerCert(o.CertFile, o.KeyFile, util.NewStringSet([]string(o.Hostnames)...))
		}
	} else {
		if o.ClientAuth {
			ca, written, err = signerCert.EnsureClientServerCert(o.CertFile, o.KeyFile, util.NewStringSet([]string(o.Hostnames)...))
		} else {
			ca, written, err = signerCert.EnsureServerCert(o.CertFile, o.KeyFile, util.NewStringSet([]string(o.Hostnames)...))
		}
	}
	if written {
		fmt.Fprintf(o.Output.Get(), "Generated new server certificate as %s, key as %s\n", o.CertFile, o.KeyFile)
	} else {
		fmt.Fprintf(o.Output.Get(), "Keeping existing server certificate at %s, key at %s\n", o.CertFile, o.KeyFile)
	}
	return ca, err
}
