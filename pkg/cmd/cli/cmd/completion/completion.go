package completion

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	completionLong = `This command prints shell code which must be evaluated to provide interactive
completion of %s commands.`

	completionExample = `  # Generate the %s completion code for bash
  %s completion bash > bash_completion.sh
  source bash_completion.sh

  # The above example depends on the bash-completion framework.
  It must be sourced before sourcing the openshift cli completion, i.e. on the Mac:

  brew install bash-completion
  source $(brew --prefix)/etc/bash_completion
  %s completion bash > bash_completion.sh
  source bash_completion.sh

  # In zsh*, the following will load openshift cli zsh completion:
  source <(%s completion zsh)

  * zsh completions are only supported in versions of zsh >= 5.2`
)

func NewCmdCompletion(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmdHelpName := fullName

	if strings.HasSuffix(fullName, "completion") {
		cmdHelpName = "openshift"
	}

	cmd := kcmd.NewCmdCompletion(f.Factory, out)
	cmd.Long = fmt.Sprintf(completionLong, cmdHelpName)
	cmd.Example = fmt.Sprintf(completionExample, cmdHelpName, cmdHelpName, cmdHelpName, cmdHelpName)
	return cmd
}
