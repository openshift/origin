package secrets

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"
)

const (
	// CreateSSHAuthSecretRecommendedCommandName represents name of subcommand for `oc secrets` command
	CreateSSHAuthSecretRecommendedCommandName = "new-sshauth"

	createSSHAuthSecretLong = `
Create a new SSH authentication secret

SSH authentication secrets are used to authenticate against SCM servers.

When creating applications, you may have a SCM server that requires SSH authentication - private SSH key.
In order for the nodes to clone source code on your behalf, they have to have the credentials. You can 
provide this information by creating a 'sshauth' secret and attaching it to your service account.`

	createSSHAuthSecretExample = `  // If your SSH authentication method requires only private SSH key, add it by using:
  $ %[1]s SECRET --ssh-privatekey=FILENAME

  // If your SSH authentication method requires also CA certificate, add it by using:
  $ %[1]s SECRET --ssh-privatekey=FILENAME --ca-cert=FILENAME

  // If you do already have a .gitconfig file needed for authentication, you can create a gitconfig secret by using:
  $ %[2]s SECRET path/to/.gitconfig`
)

// CreateSSHAuthSecretOptions holds the credential needed to authenticate against SCM servers.
type CreateSSHAuthSecretOptions struct {
	SecretName      string
	PrivateKeyPath  string
	CertificatePath string
	GitConfigPath   string

	PromptForPassword bool

	Out io.Writer

	SecretsInterface client.SecretsInterface
}

// NewCmdCreateSSHAuthSecret implements the OpenShift cli secrets new-sshauth subcommand
func NewCmdCreateSSHAuthSecret(name, fullName string, f *kcmdutil.Factory, out io.Writer, newSecretFullName, ocEditFullName string) *cobra.Command {
	o := &CreateSSHAuthSecretOptions{
		Out: out,
	}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s SECRET --ssh-privatekey=FILENAME [--ca-cert=FILENAME] [--gitconfig=FILENAME]", name),
		Short:   "Create a new secret for SSH authentication",
		Long:    createSSHAuthSecretLong,
		Example: fmt.Sprintf(createSSHAuthSecretExample, fullName, newSecretFullName, ocEditFullName),
		Run: func(c *cobra.Command, args []string) {
			if err := o.Complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			if len(kcmdutil.GetFlagString(c, "output")) != 0 {
				secret, err := o.NewSSHAuthSecret()
				kcmdutil.CheckErr(err)

				kcmdutil.CheckErr(f.PrintObject(c, secret, out))
				return
			}

			if err := o.CreateSSHAuthSecret(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&o.PrivateKeyPath, "ssh-privatekey", "", "Path to a SSH private key")
	cmd.Flags().StringVar(&o.CertificatePath, "ca-cert", "", "Path to a certificate file")
	cmd.Flags().StringVar(&o.GitConfigPath, "gitconfig", "", "Path to a .gitconfig file")

	// autocompletion hints
	cmd.MarkFlagFilename("ssh-privatekey")
	cmd.MarkFlagFilename("ca-cert")
	cmd.MarkFlagFilename("gitconfig")

	kcmdutil.AddPrinterFlags(cmd)

	return cmd
}

// CreateSSHAuthSecret saves created Secret structure and prints the secret name to the output on success.
func (o *CreateSSHAuthSecretOptions) CreateSSHAuthSecret() error {
	secret, err := o.NewSSHAuthSecret()
	if err != nil {
		return err
	}

	if _, err := o.SecretsInterface.Create(secret); err != nil {
		return err
	}

	fmt.Fprintf(o.GetOut(), "secret/%s\n", secret.Name)
	return nil
}

// NewSSHAuthSecret builds up the Secret structure containing secret name, type and data structure
// containing desired credentials.
func (o *CreateSSHAuthSecretOptions) NewSSHAuthSecret() (*api.Secret, error) {
	secret := &api.Secret{}
	secret.Name = o.SecretName
	secret.Type = api.SecretTypeOpaque
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
func (o *CreateSSHAuthSecretOptions) Complete(f *kcmdutil.Factory, args []string) error {
	if len(args) != 1 {
		return errors.New("must have exactly one argument: secret name")
	}
	o.SecretName = args[0]

	if f != nil {
		client, err := f.Client()
		if err != nil {
			return err
		}
		namespace, _, err := f.DefaultNamespace()
		if err != nil {
			return err
		}
		o.SecretsInterface = client.Secrets(namespace)
	}

	return nil
}

// Validate check if all necessary fields from CreateSSHAuthSecretOptions are present.
func (o CreateSSHAuthSecretOptions) Validate() error {
	if len(o.SecretName) == 0 {
		return errors.New("basic authentication secret name must be present")
	}

	if len(o.PrivateKeyPath) == 0 {
		return errors.New("must provide SSH private key")
	}

	return nil
}

// GetOut check if the CreateSSHAuthSecretOptions Out Writer is set. Returns it if the Writer
// is present, if not returns Writer on which all Write calls succeed without doing anything.
func (o CreateSSHAuthSecretOptions) GetOut() io.Writer {
	if o.Out == nil {
		return ioutil.Discard
	}

	return o.Out
}
