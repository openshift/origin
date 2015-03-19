package admin

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type CreateServerCertOptions struct {
	GetSignerCertOptions *GetSignerCertOptions

	CertFile string
	KeyFile  string

	Hostnames util.StringList
	Overwrite bool
}

func NewCommandCreateServerCert() *cobra.Command {
	options := &CreateServerCertOptions{GetSignerCertOptions: &GetSignerCertOptions{}}

	cmd := &cobra.Command{
		Use:   "create-server-cert",
		Short: "Create server certificate",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			if _, err := options.CreateServerCert(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()
	BindGetSignerCertOptions(options.GetSignerCertOptions, flags, "")

	flags.StringVar(&options.CertFile, "cert", "openshift.local.certificates/user/cert.crt", "The certificate file.")
	flags.StringVar(&options.KeyFile, "key", "openshift.local.certificates/user/key.key", "The key file.")

	flags.Var(&options.Hostnames, "hostnames", "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	flags.BoolVar(&options.Overwrite, "overwrite", true, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

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

	return o.GetSignerCertOptions.Validate()
}

func (o CreateServerCertOptions) CreateServerCert() (*crypto.TLSCertificateConfig, error) {
	glog.V(2).Infof("Creating a server cert with: %#v", o)

	signerCert, err := o.GetSignerCertOptions.GetSignerCert()
	if err != nil {
		return nil, err
	}

	if o.Overwrite {
		return signerCert.MakeServerCert(o.CertFile, o.KeyFile, util.NewStringSet([]string(o.Hostnames)...))
	} else {
		return signerCert.EnsureServerCert(o.CertFile, o.KeyFile, util.NewStringSet([]string(o.Hostnames)...))
	}
}
