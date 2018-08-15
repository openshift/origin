package serviceaccounts

import (
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

const ServiceAccountsRecommendedName = "serviceaccounts"

var serviceAccountsLong = templates.LongDesc(`Manage service accounts in your project

Service accounts allow system components to access the API.`)

const (
	serviceAccountsShort = `Manage service accounts in your project`
)

func NewCmdServiceAccounts(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmds := &cobra.Command{
		Use:     name,
		Short:   serviceAccountsShort,
		Long:    serviceAccountsLong,
		Aliases: []string{"sa"},
		Run:     kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	cmds.AddCommand(NewCommandCreateKubeconfig(CreateKubeconfigRecommendedName, fullName+" "+CreateKubeconfigRecommendedName, f, streams))
	cmds.AddCommand(NewCommandGetServiceAccountToken(GetServiceAccountTokenRecommendedName, fullName+" "+GetServiceAccountTokenRecommendedName, f, streams))
	cmds.AddCommand(NewCommandNewServiceAccountToken(NewServiceAccountTokenRecommendedName, fullName+" "+NewServiceAccountTokenRecommendedName, f, streams))

	return cmds
}
