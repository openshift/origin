package config

import (
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

const ConfigRecommendedName = "config"

var configLong = templates.LongDesc(`Manage cluster configuration files like master-config.yaml.`)

func NewCmdConfig(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Manage config",
		Long:  configLong,
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	cmd.AddCommand(NewCmdPatch(PatchRecommendedName, fullName+" "+PatchRecommendedName, f, streams))

	return cmd
}
