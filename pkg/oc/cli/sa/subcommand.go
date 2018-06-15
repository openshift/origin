package sa

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const ServiceAccountsRecommendedName = "serviceaccounts"

var serviceAccountsLong = templates.LongDesc(`Manage service accounts in your project

Service accounts allow system components to access the API.`)

const (
	serviceAccountsShort = `Manage service accounts in your project`
)

func NewCmdServiceAccounts(name, fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	cmds := &cobra.Command{
		Use:     name,
		Short:   serviceAccountsShort,
		Long:    serviceAccountsLong,
		Aliases: []string{"sa"},
		Run:     cmdutil.DefaultSubCommandRun(errOut),
	}

	cmds.AddCommand(NewCommandCreateKubeconfig(CreateKubeconfigRecommendedName, fullName+" "+CreateKubeconfigRecommendedName, f, out))
	cmds.AddCommand(NewCommandGetServiceAccountToken(GetServiceAccountTokenRecommendedName, fullName+" "+GetServiceAccountTokenRecommendedName, f, out))
	cmds.AddCommand(NewCommandNewServiceAccountToken(NewServiceAccountTokenRecommendedName, fullName+" "+NewServiceAccountTokenRecommendedName, f, out))

	return cmds
}
