package secrets

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	kterm "k8s.io/kubernetes/pkg/kubectl/util/term"

	"github.com/openshift/origin/pkg/cmd/util/term"
)

// CreateBasicAuthSecretRecommendedCommandName represents name of subcommand for `oc secrets` command
const CreateBasicAuthSecretRecommendedCommandName = "new-basicauth"

var (
	createBasicAuthSecretLong = templates.LongDesc(`
    Create a new basic authentication secret

    Basic authentication secrets are used to authenticate against SCM servers.

    When creating applications, you may have a SCM server that requires basic authentication - username, password.
    In order for the nodes to clone source code on your behalf, they have to have the credentials. You can provide
    this information by creating a 'basicauth' secret and attaching it to your service account.`)

	createBasicAuthSecretExample = templates.Examples(`
    # If your basic authentication method requires only username and password or token, add it by using:
    %[1]s SECRET --username=USERNAME --password=PASSWORD

    # If your basic authentication method requires also CA certificate, add it by using:
    %[1]s SECRET --username=USERNAME --password=PASSWORD --ca-cert=FILENAME

    # If you do already have a .gitconfig file needed for authentication, you can create a gitconfig secret by using:
    %[2]s SECRET path/to/.gitconfig`)
)

// CreateBasicAuthSecretOptions holds the credential needed to authenticate against SCM servers.
type CreateBasicAuthSecretOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Printer printers.ResourcePrinter

	SecretName      string
	Username        string
	Password        string
	CertificatePath string
	GitConfigPath   string

	PromptForPassword bool

	SecretsInterface corev1client.SecretInterface

	genericclioptions.IOStreams
}

func NewCreateBasicAuthSecretOptions(streams genericclioptions.IOStreams) *CreateBasicAuthSecretOptions {
	return &CreateBasicAuthSecretOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

// NewCmdCreateBasicAuthSecret implements the OpenShift cli secrets new-basicauth subcommand
func NewCmdCreateBasicAuthSecret(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams, newSecretFullName, ocEditFullName string) *cobra.Command {
	o := NewCreateBasicAuthSecretOptions(streams)

	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s SECRET --username=USERNAME --password=PASSWORD [--ca-cert=FILENAME] [--gitconfig=FILENAME]", name),
		Short:      "Create a new secret for basic authentication",
		Long:       createBasicAuthSecretLong,
		Example:    fmt.Sprintf(createBasicAuthSecretExample, fullName, newSecretFullName, ocEditFullName),
		Deprecated: "use oc create secret",
		Hidden:     true,
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.Username, "username", "", "Username for Git authentication")
	cmd.Flags().StringVar(&o.Password, "password", "", "Password or token for Git authentication")
	cmd.Flags().StringVar(&o.CertificatePath, "ca-cert", "", "Path to a certificate file")
	cmd.MarkFlagFilename("ca-cert")
	cmd.Flags().StringVar(&o.GitConfigPath, "gitconfig", "", "Path to a .gitconfig file")
	cmd.MarkFlagFilename("gitconfig")
	cmd.Flags().BoolVarP(&o.PromptForPassword, "prompt", "", false, "If true, prompt for password or token")

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

// CreateBasicAuthSecret saves created Secret structure and prints the secret name to the output on success.
func (o *CreateBasicAuthSecretOptions) Run() error {
	secret, err := o.NewBasicAuthSecret()
	if err != nil {
		return err
	}
	if _, err := o.SecretsInterface.Create(secret); err != nil {
		return err
	}

	return o.Printer.PrintObj(secret, o.Out)
}

// NewBasicAuthSecret builds up the Secret structure containing secret name, type and data structure
// containing desired credentials.
func (o *CreateBasicAuthSecretOptions) NewBasicAuthSecret() (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secret.Name = o.SecretName
	secret.Type = corev1.SecretTypeBasicAuth
	secret.Data = map[string][]byte{}

	if len(o.Username) != 0 {
		secret.Data[SourceUsername] = []byte(o.Username)
	}

	if len(o.Password) != 0 {
		secret.Data[SourcePassword] = []byte(o.Password)
	}

	if len(o.CertificatePath) != 0 {
		caContent, err := ioutil.ReadFile(o.CertificatePath)
		if err != nil {
			return nil, err
		}
		secret.Data[SourceCertificate] = caContent
	}

	if len(o.GitConfigPath) != 0 {
		gitConfig, err := ioutil.ReadFile(o.GitConfigPath)
		if err != nil {
			return nil, err
		}
		secret.Data[SourceGitConfig] = gitConfig
	}

	return secret, nil
}

// Complete fills CreateBasicAuthSecretOptions fields with data and checks for mutual exclusivity
// between flags from different option groups.
func (o *CreateBasicAuthSecretOptions) Complete(f kcmdutil.Factory, args []string) error {
	if len(args) != 1 {
		return errors.New("must have exactly one argument: secret name")
	}
	o.SecretName = args[0]

	if o.PromptForPassword {
		if len(o.Password) > 0 {
			return errors.New("must provide either --prompt or --password flag")
		}
		if !kterm.IsTerminal(o.In) {
			return errors.New("provided reader is not a terminal")
		}

		o.Password = term.PromptForPasswordString(o.In, o.Out, "Password: ")
		if len(o.Password) == 0 {
			return errors.New("password must be provided")
		}
	}

	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	clientset, err := corev1client.NewForConfig(config)
	if err != nil {
		return err
	}
	o.SecretsInterface = clientset.Secrets(namespace)

	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	return nil
}

// Validate check if all necessary fields from CreateBasicAuthSecretOptions are present.
func (o CreateBasicAuthSecretOptions) Validate() error {
	if len(o.SecretName) == 0 {
		return errors.New("basic authentication secret name must be present")
	}
	if len(o.Username) == 0 && len(o.Password) == 0 {
		return errors.New("must provide basic authentication credentials")
	}

	return nil
}
