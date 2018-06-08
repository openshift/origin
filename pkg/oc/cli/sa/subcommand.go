package sa

import (
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const ServiceAccountsRecommendedName = "serviceaccounts"

var serviceAccountsLong = templates.LongDesc(`Manage service accounts in your project

Service accounts allow system components to access the API.`)

const (
	serviceAccountsShort = `Manage service accounts in your project`
)

func NewCmdServiceAccounts(name, fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmds := &cobra.Command{
		Use:     name,
		Short:   serviceAccountsShort,
		Long:    serviceAccountsLong,
		Aliases: []string{"sa"},
		Run:     cmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	cmds.AddCommand(NewCommandCreateKubeconfig(CreateKubeconfigRecommendedName, fullName+" "+CreateKubeconfigRecommendedName, f, streams.Out))
	cmds.AddCommand(NewCommandGetServiceAccountToken(GetServiceAccountTokenRecommendedName, fullName+" "+GetServiceAccountTokenRecommendedName, f, streams.Out))
	cmds.AddCommand(NewCommandNewServiceAccountToken(NewServiceAccountTokenRecommendedName, fullName+" "+NewServiceAccountTokenRecommendedName, f, streams.Out))

	return cmds
}
