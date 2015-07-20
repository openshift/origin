package secrets

import (
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const SecretsRecommendedName = "secrets"

const (
	secretsLong = `
Manage secrets in your project

Secrets are used to store confidential information that should not be contained inside of an image.
They are commonly used to hold things like keys for authentication to other internal systems like 
Docker registries.`
)

func NewCmdSecrets(name, fullName string, f *clientcmd.Factory, out io.Writer, ocEditFullName string) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage secrets",
		Long:  secretsLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	newSecretFullName := fullName + " " + NewSecretRecommendedCommandName
	cmds.AddCommand(NewCmdCreateSecret(NewSecretRecommendedCommandName, newSecretFullName, f, out))
	cmds.AddCommand(NewCmdCreateDockerConfigSecret(CreateDockerConfigSecretRecommendedName, fullName+" "+CreateDockerConfigSecretRecommendedName, f.Factory, out, newSecretFullName, ocEditFullName))
	cmds.AddCommand(NewCmdAddSecret(AddSecretRecommendedName, fullName+" "+AddSecretRecommendedName, f.Factory, out))

	return cmds
}
