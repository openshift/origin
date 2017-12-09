package migrate

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const MigrateRecommendedName = "migrate"

var migrateLong = templates.LongDesc(`
	Migrate resources on the cluster

	These commands assist administrators in performing preventative maintenance on a cluster.`)

func NewCommandMigrate(name, fullName string, f *clientcmd.Factory, out, errOut io.Writer, cmds ...*cobra.Command) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmd := &cobra.Command{
		Use:   name,
		Short: "Migrate data in the cluster",
		Long:  migrateLong,
		Run:   cmdutil.DefaultSubCommandRun(errOut),
	}
	cmd.AddCommand(cmds...)
	return cmd
}
