package admin

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type CreateNodeClientCertOptions struct {
	GetSignerCertOptions *GetSignerCertOptions

	CertFile string
	KeyFile  string

	NodeName string

	Overwrite bool
}

func NewCommandCreateNodeClientCert() *cobra.Command {
	options := &CreateNodeClientCertOptions{GetSignerCertOptions: &GetSignerCertOptions{}}

	cmd := &cobra.Command{
		Use:   "create-node-cert",
		Short: "Create node certificate",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			if _, err := options.CreateNodeClientCert(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()
	BindGetSignerCertOptions(options.GetSignerCertOptions, flags, "")

	flags.StringVar(&options.CertFile, "cert", "openshift.local.certificates/user/cert.crt", "The certificate file.")
	flags.StringVar(&options.KeyFile, "key", "openshift.local.certificates/user/key.key", "The key file.")

	flags.StringVar(&options.NodeName, "node-name", "", "The name of the node.")
	flags.BoolVar(&options.Overwrite, "overwrite", true, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

	return cmd
}

func (o CreateNodeClientCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.CertFile) == 0 {
		return errors.New("cert must be provided")
	}
	if len(o.KeyFile) == 0 {
		return errors.New("key must be provided")
	}
	if len(o.NodeName) == 0 {
		return errors.New("node-name must be provided")
	}

	return o.GetSignerCertOptions.Validate()
}

func (o CreateNodeClientCertOptions) CreateNodeClientCert() (*crypto.TLSCertificateConfig, error) {
	glog.V(2).Infof("Creating a node client cert with: %#v and %#v", o, o.GetSignerCertOptions)

	nodeCertOptions := CreateClientCertOptions{
		GetSignerCertOptions: o.GetSignerCertOptions,

		CertFile: o.CertFile,
		KeyFile:  o.KeyFile,

		User:      "system:node-" + o.NodeName,
		Groups:    util.StringList([]string{bootstrappolicy.NodesGroup}),
		Overwrite: o.Overwrite,
	}

	return nodeCertOptions.CreateClientCert()
}
