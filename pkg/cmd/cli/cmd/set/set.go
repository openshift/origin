package set

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	setLong = `
Configure application resources

These commands help you make changes to existing application resources.`
)

// NewCmdSet exposes commands for modifying objects.
func NewCmdSet(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	set := &cobra.Command{
		Use:   "set COMMAND",
		Short: "Commands that help set specific features on objects",
		Long:  setLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	name := fmt.Sprintf("%s set", fullName)

	groups := templates.CommandGroups{
		{
			Message: "Replication controllers, deployments, and daemon sets:",
			Commands: []*cobra.Command{
				NewCmdEnv(name, f, in, out),
				NewCmdVolume(name, f, out, errout),
				NewCmdProbe(name, f, out, errout),
			},
		},
		{
			Message: "Manage application flows:",
			Commands: []*cobra.Command{
				NewCmdTriggers(name, f, out, errout),
			},
		},
	}
	groups.Add(set)
	templates.ActsAsRootCommand(set, []string{"options"}, groups...)
	return set
}
