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
)

// CreateSSHAuthSecretRecommendedCommandName represents name of subcommand for `oc secrets` command
const CreateSSHAuthSecretRecommendedCommandName = "new-sshauth"

var (
	createSSHAuthSecretLong = templates.LongDesc(`
    Create a new SSH authentication secret

    SSH authentication secrets are used to authenticate against SCM servers.

    When creating applications, you may have a SCM server that requires SSH authentication - private SSH key.
    In order for the nodes to clone source code on your behalf, they have to have the credentials. You can
    provide this information by creating a 'sshauth' secret and attaching it to your service account.`)

	createSSHAuthSecretExample = templates.Examples(`
    # If your SSH authentication method requires only private SSH key, add it by using:
    %[1]s SECRET --ssh-privatekey=FILENAME

    # If your SSH authentication method requires also CA certificate, add it by using:
    %[1]s SECRET --ssh-privatekey=FILENAME --ca-cert=FILENAME

    # If you do already have a .gitconfig file needed for authentication, you can create a gitconfig secret by using:
    %[2]s SECRET path/to/.gitconfig`)
)

// CreateSSHAuthSecretOptions holds the credential needed to authenticate against SCM servers.
type CreateSSHAuthSecretOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Printer printers.ResourcePrinter

	SecretName      string
	PrivateKeyPath  string
	CertificatePath string
	GitConfigPath   string

	PromptForPassword bool

	SecretsInterface corev1client.SecretInterface

	genericclioptions.IOStreams
}

func NewCreateSSHAuthSecretOptions(streams genericclioptions.IOStreams) *CreateSSHAuthSecretOptions {
	return &CreateSSHAuthSecretOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

// NewCmdCreateSSHAuthSecret implements the OpenShift cli secrets new-sshauth subcommand
func NewCmdCreateSSHAuthSecret(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams, newSecretFullName, ocEditFullName string) *cobra.Command {
	o := NewCreateSSHAuthSecretOptions(streams)

	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s SECRET --ssh-privatekey=FILENAME [--ca-cert=FILENAME] [--gitconfig=FILENAME]", name),
		Short:      "Create a new secret for SSH authentication",
		Long:       createSSHAuthSecretLong,
		Example:    fmt.Sprintf(createSSHAuthSecretExample, fullName, newSecretFullName, ocEditFullName),
		Deprecated: "use oc create secret",
		Hidden:     true,
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, args))
			kcmdutil.CheckErr(o.Validate(args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.PrivateKeyPath, "ssh-privatekey", "", "Path to a SSH private key file")
	cmd.MarkFlagFilename("ssh-privatekey")
	cmd.Flags().StringVar(&o.CertificatePath, "ca-cert", "", "Path to a certificate file")
	cmd.MarkFlagFilename("ca-cert")
	cmd.Flags().StringVar(&o.GitConfigPath, "gitconfig", "", "Path to a .gitconfig file")
	cmd.MarkFlagFilename("gitconfig")

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

// CreateSSHAuthSecret saves created Secret structure and prints the secret name to the output on success.
func (o *CreateSSHAuthSecretOptions) Run() error {
	secret, err := o.NewSSHAuthSecret()
	if err != nil {
		return err
	}

	if _, err := o.SecretsInterface.Create(secret); err != nil {
		return err
	}

	return o.Printer.PrintObj(secret, o.Out)
}

// NewSSHAuthSecret builds up the Secret structure containing secret name, type and data structure
// containing desired credentials.
func (o *CreateSSHAuthSecretOptions) NewSSHAuthSecret() (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secret.Name = o.SecretName
	secret.Type = corev1.SecretTypeSSHAuth
	secret.Data = map[string][]byte{}

	if len(o.PrivateKeyPath) != 0 {
		privateKeyContent, err := ioutil.ReadFile(o.PrivateKeyPath)
		if err != nil {
			return nil, err
		}
		secret.Data[SourcePrivateKey] = privateKeyContent
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

// Complete fills CreateSSHAuthSecretOptions fields with data and checks whether necessary
// arguments were provided.
func (o *CreateSSHAuthSecretOptions) Complete(f kcmdutil.Factory, args []string) error {
	o.SecretName = args[0]

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	client, err := corev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.SecretsInterface = client.Secrets(namespace)

	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	return nil
}

// Validate check if all necessary fields from CreateSSHAuthSecretOptions are present.
func (o CreateSSHAuthSecretOptions) Validate(args []string) error {
	if len(args) != 1 {
		return errors.New("must have exactly one argument: secret name")
	}
	if len(o.SecretName) == 0 {
		return errors.New("basic authentication secret name must be present")
	}

	if len(o.PrivateKeyPath) == 0 {
		return errors.New("must provide SSH private key")
	}

	return nil
}
