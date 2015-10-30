package admin

import (
	"errors"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	configutil "github.com/openshift/origin/pkg/cmd/server/util"
	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"
)

const CreateClientCommandName = "create-api-client-config"

type CreateClientOptions struct {
	SignerCertOptions *configutil.SignerCertOptions

	ClientDir string
	BaseName  string

	User   string
	Groups util.StringList

	APIServerCAFile    string
	APIServerURL       string
	PublicAPIServerURL string
	Output             io.Writer
}

const createClientLong = `
Create a client configuration for connecting to the server

This command creates a folder containing a client certificate, a client key,
a server certificate authority, and a .kubeconfig file for connecting to the
master as the provided user.
`

func NewCommandCreateClient(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateClientOptions{SignerCertOptions: configutil.NewDefaultSignerCertOptions(), Output: out}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a config file for connecting to the server as a user",
		Long:  createClientLong,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.CreateClientFolder(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()

	configutil.BindSignerCertOptions(options.SignerCertOptions, flags, "")

	flags.StringVar(&options.ClientDir, "client-dir", "", "The client data directory.")
	flags.StringVar(&options.BaseName, "basename", "", "The base filename to use for the .crt, .key, and .kubeconfig files. Defaults to the username.")

	flags.StringVar(&options.User, "user", "", "The scope qualified username.")
	flags.Var(&options.Groups, "groups", "The list of groups this user belongs to. Comma delimited list")

	flags.StringVar(&options.APIServerURL, "master", "https://localhost:8443", "The API server's URL.")
	flags.StringVar(&options.PublicAPIServerURL, "public-master", "", "The API public facing server's URL (if applicable).")
	flags.StringVar(&options.APIServerCAFile, "certificate-authority", "openshift.local.config/master/ca.crt", "Path to the API server's CA file.")

	// autocompletion hints
	cmd.MarkFlagFilename("client-dir")
	cmd.MarkFlagFilename("certificate-authority")

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

	if o.SignerCertOptions == nil {
		return errors.New("signer options are required")
	}
	if err := o.SignerCertOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func (o CreateClientOptions) CreateClientFolder() error {
	glog.V(4).Infof("creating a .kubeconfig with: %#v", o)

	baseName := o.BaseName
	if len(baseName) == 0 {
		baseName = o.User
	}
	clientCertFile := configutil.DefaultCertFilename(o.ClientDir, baseName)
	clientKeyFile := configutil.DefaultKeyFilename(o.ClientDir, baseName)
	clientCopyOfCAFile := configutil.DefaultCAFilename(o.ClientDir, "ca")
	kubeConfigFile := configutil.DefaultKubeConfigFilename(o.ClientDir, baseName)

	createClientCertOptions := CreateClientCertOptions{
		SignerCertOptions: o.SignerCertOptions,
		CertFile:          clientCertFile,
		KeyFile:           clientKeyFile,

		User:      o.User,
		Groups:    o.Groups,
		Overwrite: true,
		Output:    o.Output,
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

		CertFile: clientCertFile,
		KeyFile:  clientKeyFile,

		ContextNamespace: kapi.NamespaceDefault,

		KubeConfigFile: kubeConfigFile,
		Output:         o.Output,
	}
	if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
		return err
	}

	return nil
}
