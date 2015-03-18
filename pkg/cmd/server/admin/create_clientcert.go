package admin

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type CreateClientCertOptions struct {
	GetSignerCertOptions *GetSignerCertOptions

	CertFile string
	KeyFile  string

	User   string
	Groups util.StringList

	Overwrite bool
}

func NewCommandCreateClientCert() *cobra.Command {
	options := &CreateClientCertOptions{GetSignerCertOptions: &GetSignerCertOptions{}}

	cmd := &cobra.Command{
		Use:   "create-client-cert",
		Short: "Create client certificate",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			if _, err := options.CreateClientCert(); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}
		},
	}

	flags := cmd.Flags()
	BindGetSignerCertOptions(options.GetSignerCertOptions, flags, "")

	flags.StringVar(&options.CertFile, "cert", "openshift.local.certificates/user/cert.crt", "The certificate file.")
	flags.StringVar(&options.KeyFile, "key", "openshift.local.certificates/user/key.key", "The key file.")

	flags.StringVar(&options.User, "user", "", "The scope qualified username.")
	flags.Var(&options.Groups, "groups", "The list of groups this user belongs to. Comma delimited list")
	flags.BoolVar(&options.Overwrite, "overwrite", true, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

	return cmd
}

func (o CreateClientCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.CertFile) == 0 {
		return errors.New("cert must be provided")
	}
	if len(o.KeyFile) == 0 {
		return errors.New("key must be provided")
	}
	if len(o.User) == 0 {
		return errors.New("user must be provided")
	}

	return o.GetSignerCertOptions.Validate()
}

func (o CreateClientCertOptions) CreateClientCert() (*crypto.TLSCertificateConfig, error) {
	glog.V(2).Infof("Creating a client cert with: %#v and %#v", o, o.GetSignerCertOptions)

	signerCert, err := o.GetSignerCertOptions.GetSignerCert()
	if err != nil {
		return nil, err
	}

	userInfo := &user.DefaultInfo{Name: o.User, Groups: o.Groups}
	if o.Overwrite {
		return signerCert.MakeClientCertificate(o.CertFile, o.KeyFile, userInfo)
	} else {
		return signerCert.EnsureClientCertificate(o.CertFile, o.KeyFile, userInfo)
	}
}
