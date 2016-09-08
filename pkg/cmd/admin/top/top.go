package top

import (
	"io"

	"github.com/spf13/cobra"

	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	TopRecommendedName = "top"

	topLong = `Show usage statistics of resources on the server

This command analyzes resources managed by the platform and presents current
usage statistics.`
)

func NewCommandTop(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Show usage statistics of resources on the server",
		Long:  topLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	cmds.AddCommand(NewCmdTopImages(f, fullName, TopImagesRecommendedName, out))
	cmds.AddCommand(NewCmdTopImageStreams(f, fullName, TopImageStreamsRecommendedName, out))
	cmds.AddCommand(kcmd.NewCmdTopNode(f.Factory, out))
	cmds.AddCommand(kcmd.NewCmdTopPod(f.Factory, out))
	return cmds
}
