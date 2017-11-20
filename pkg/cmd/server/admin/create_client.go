package admin

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

const CreateClientCommandName = "create-api-client-config"

type CreateClientOptions struct {
	SignerCertOptions *SignerCertOptions

	ClientDir string
	BaseName  string

	ExpireDays int

	User   string
	Groups []string

	APIServerCAFiles   []string
	APIServerURL       string
	PublicAPIServerURL string
	Output             io.Writer
}

var createClientLong = templates.LongDesc(`
	Create a client configuration for connecting to the server

	This command creates a folder containing a client certificate, a client key,
	a server certificate authority, and a .kubeconfig file for connecting to the
	master as the provided user.`)

func NewCommandCreateClient(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateClientOptions{
		SignerCertOptions: NewDefaultSignerCertOptions(),
		ExpireDays:        crypto.DefaultCertificateLifetimeInDays,
		Output:            out,
	}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a config file for connecting to the server as a user",
		Long:  createClientLong,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.CreateClientFolder(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()

	BindSignerCertOptions(options.SignerCertOptions, flags, "")

	flags.StringVar(&options.ClientDir, "client-dir", "", "The client data directory.")
	flags.StringVar(&options.BaseName, "basename", "", "The base filename to use for the .crt, .key, and .kubeconfig files. Defaults to the username.")

	flags.StringVar(&options.User, "user", "", "The scope qualified username.")
	flags.StringSliceVar(&options.Groups, "groups", options.Groups, "The list of groups this user belongs to. Comma delimited list")

	flags.StringVar(&options.APIServerURL, "master", "https://localhost:8443", "The API server's URL.")
	flags.StringVar(&options.PublicAPIServerURL, "public-master", "", "The API public facing server's URL (if applicable).")
	flags.StringSliceVar(&options.APIServerCAFiles, "certificate-authority", []string{"openshift.local.config/master/ca.crt"}, "Files containing signing authorities to use to verify the API server's serving certificate.")
	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificates in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")

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
	if len(o.APIServerCAFiles) == 0 {
		return errors.New("certificate-authority must be provided")
	} else {
		for _, caFile := range o.APIServerCAFiles {
			if _, err := cert.NewPool(caFile); err != nil {
				return fmt.Errorf("certificate-authority must be a valid certificate file: %v", err)
			}
		}
	}

	if o.SignerCertOptions == nil {
		return errors.New("signer options are required")
	}
	if err := o.SignerCertOptions.Validate(); err != nil {
		return err
	}
	if o.ExpireDays <= 0 {
		return errors.New("expire-days must be valid number of days")
	}
	return nil
}

func (o CreateClientOptions) CreateClientFolder() error {
	glog.V(4).Infof("creating a .kubeconfig with: %#v", o)

	baseName := o.BaseName
	if len(baseName) == 0 {
		baseName = o.User
	}
	clientCertFile := DefaultCertFilename(o.ClientDir, baseName)
	clientKeyFile := DefaultKeyFilename(o.ClientDir, baseName)
	clientCopyOfCAFile := DefaultCAFilename(o.ClientDir, "ca")
	kubeConfigFile := DefaultKubeConfigFilename(o.ClientDir, baseName)

	createClientCertOptions := CreateClientCertOptions{
		SignerCertOptions: o.SignerCertOptions,
		CertFile:          clientCertFile,
		KeyFile:           clientKeyFile,
		ExpireDays:        o.ExpireDays,

		User:      o.User,
		Groups:    o.Groups,
		Overwrite: true,
		Output:    o.Output,
	}
	if err := createClientCertOptions.Validate(nil); err != nil {
		return err
	}
	if _, err := createClientCertOptions.CreateClientCert(); err != nil {
		return err
	}

	// copy the CA file(s) over
	if caBytes, readErr := readFiles(o.APIServerCAFiles, []byte("\n")); readErr != nil {
		return readErr
	} else if writeErr := ioutil.WriteFile(clientCopyOfCAFile, caBytes, 0644); writeErr != nil {
		return writeErr
	}

	createKubeConfigOptions := CreateKubeConfigOptions{
		APIServerURL:       o.APIServerURL,
		PublicAPIServerURL: o.PublicAPIServerURL,
		APIServerCAFiles:   []string{clientCopyOfCAFile},

		CertFile: clientCertFile,
		KeyFile:  clientKeyFile,

		ContextNamespace: metav1.NamespaceDefault,

		KubeConfigFile: kubeConfigFile,
		Output:         o.Output,
	}
	if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
		return err
	}

	return nil
}
