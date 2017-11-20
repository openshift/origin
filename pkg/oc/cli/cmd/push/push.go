package push

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

var (
	pushLong = templates.LongDesc(`
		Push content into OpenShift

		These commands send content into OpenShift.`)
)

// NewCmdImport exposes commands for modifying objects.
func NewCmdPush(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push COMMAND",
		Short: "Commands that push content into OpenShift",
		Long:  pushLong,
		Run:   cmdutil.DefaultSubCommandRun(errout),
	}

	name := fmt.Sprintf("%s push", fullName)

	cmd.AddCommand(NewCmdPushBinary(name, f, in, out, errout))
	return cmd
}
