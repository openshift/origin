package config

import (
	"io"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const ConfigRecommendedName = "config"

var configLong = templates.LongDesc(`Manage cluster configuration files like master-config.yaml.`)

func NewCmdConfig(name, fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Manage config",
		Long:  configLong,
		Run:   cmdutil.DefaultSubCommandRun(errout),
	}

	cmd.AddCommand(NewCmdPatch(PatchRecommendedName, fullName+" "+PatchRecommendedName, f, out))

	return cmd
}
