package migrate

import (
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const MigrateRecommendedName = "migrate"

const migrateLong = `Migrate resources on the cluster

These commands assist administrators in performing preventative maintenance on a cluster.`

func NewCommandMigrate(name, fullName string, f *clientcmd.Factory, out io.Writer, cmds ...*cobra.Command) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmd := &cobra.Command{
		Use:   name,
		Short: "Migrate data in the cluster",
		Long:  migrateLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}
	cmd.AddCommand(cmds...)
	return cmd
}
