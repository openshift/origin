package admin

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

type CreateClientOptions struct {
	GetSignerCertOptions *GetSignerCertOptions

	ClientDir string

	User   string
	Groups util.StringList

	APIServerCAFile    string
	APIServerURL       string
	PublicAPIServerURL string
}

func NewCommandCreateClient() *cobra.Command {
	options := &CreateClientOptions{GetSignerCertOptions: &GetSignerCertOptions{}}

	cmd := &cobra.Command{
		Use:   "create-api-client-config",
		Short: "Create a portable client folder containing a client certificate, a client key, a server certificate authority, and a .kubeconfig file.",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			if err := options.CreateClientFolder(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()

	BindGetSignerCertOptions(options.GetSignerCertOptions, flags, "")

	flags.StringVar(&options.ClientDir, "client-dir", "", "The client data directory.")

	flags.StringVar(&options.User, "user", "", "The scope qualified username.")
	flags.Var(&options.Groups, "groups", "The list of groups this user belongs to. Comma delimited list")

	flags.StringVar(&options.APIServerURL, "master", "https://localhost:8443", "The API server's URL.")
	flags.StringVar(&options.PublicAPIServerURL, "public-master", "", "The API public facing server's URL (if applicable).")
	flags.StringVar(&options.APIServerCAFile, "certificate-authority", "openshift.local.certificates/ca/cert.crt", "Path to the API server's CA file.")

	return cmd
}

func (o CreateClientOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.ClientDir) == 0 {
		return errors.New("client-dir must be provided")
	}
	if len(o.User) == 0 {
		return errors.New("user must be provided")
	}
	if len(o.APIServerURL) == 0 {
		return errors.New("master must be provided")
	}
	if len(o.APIServerCAFile) == 0 {
		return errors.New("certificate-authority must be provided")
	}

	return nil
}

func (o CreateClientOptions) CreateClientFolder() error {
	glog.V(2).Infof("creating a .kubeconfig with: %#v", o)

	clientCertFile := path.Join(o.ClientDir, "cert.crt")
	clientKeyFile := path.Join(o.ClientDir, "key.key")
	clientCopyOfCAFile := path.Join(o.ClientDir, "ca.crt")
	kubeConfigFile := path.Join(o.ClientDir, ".kubeconfig")

	createClientCertOptions := CreateClientCertOptions{
		GetSignerCertOptions: o.GetSignerCertOptions,
		CertFile:             clientCertFile,
		KeyFile:              clientKeyFile,

		User:      o.User,
		Groups:    o.Groups,
		Overwrite: true,
	}
	if _, err := createClientCertOptions.CreateClientCert(); err != nil {
		return err
	}

	// copy the CA file over
	if caBytes, err := ioutil.ReadFile(o.APIServerCAFile); err != nil {
		return err
	} else if err := ioutil.WriteFile(clientCopyOfCAFile, caBytes, 0644); err != nil {
		return nil
	}

	createKubeConfigOptions := CreateKubeConfigOptions{
		APIServerURL:       o.APIServerURL,
		PublicAPIServerURL: o.PublicAPIServerURL,
		APIServerCAFile:    clientCopyOfCAFile,
		ServerNick:         "master",

		CertFile: clientCertFile,
		KeyFile:  clientKeyFile,
		UserNick: o.User,

		KubeConfigFile: kubeConfigFile,
	}
	if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
		return err
	}

	return nil
}
