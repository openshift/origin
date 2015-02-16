package policy

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

func NewCommandPolicy(name string) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "manage authorization policy",
		Long:  `manage authorization policy`,
		Run:   runHelp,
	}

	// Override global default to https and port 8443
	clientcmd.DefaultCluster.Server = "https://localhost:8443"
	clientConfig := cmdutil.DefaultClientConfig(cmds.PersistentFlags())

	cmds.AddCommand(NewCmdAddUser(clientConfig))
	cmds.AddCommand(NewCmdRemoveUser(clientConfig))
	cmds.AddCommand(NewCmdRemoveUserFromProject(clientConfig))
	cmds.AddCommand(NewCmdAddGroup(clientConfig))
	cmds.AddCommand(NewCmdRemoveGroup(clientConfig))
	cmds.AddCommand(NewCmdRemoveGroupFromProject(clientConfig))
	cmds.AddCommand(NewCmdWhoCan(clientConfig))

	return cmds
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func getFlagString(cmd *cobra.Command, flag string) string {
	f := cmd.Flags().Lookup(flag)
	if f == nil {
		glog.Fatalf("Flag accessed but not defined for command %s: %s", cmd.Name(), flag)
	}
	return f.Value.String()
}

func getUniqueName(basename string, existingNames *util.StringSet) string {
	if !existingNames.Has(basename) {
		return basename
	}

	for i := 0; i < 100; i++ {
		trialName := fmt.Sprintf("%v-%d", basename, i)
		if !existingNames.Has(trialName) {
			return trialName
		}
	}

	return string(util.NewUUID())
}

func getExistingRoleBindingsForRole(roleNamespace, role, bindingNamespace string, client *client.Client) ([]*authorizationapi.RoleBinding, *util.StringSet, error) {
	existingBindings, err := client.PolicyBindings(bindingNamespace).Get(roleNamespace)
	if err != nil && !strings.Contains(err.Error(), " not found") {
		return nil, &util.StringSet{}, err
	}

	ret := make([]*authorizationapi.RoleBinding, 0)
	roleBindingNames := &util.StringSet{}
	// see if we can find an existing binding that points to the role in question.
	for _, currBinding := range existingBindings.RoleBindings {
		roleBindingNames.Insert(currBinding.Name)

		if currBinding.RoleRef.Name == role {
			t := currBinding
			ret = append(ret, &t)
		}
	}

	return ret, roleBindingNames, nil
}
