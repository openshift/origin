package policy

import (
	"io"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	adminpolicy "github.com/openshift/origin/pkg/oc/admin/policy"
)

const PolicyRecommendedName = "policy"

func NewCmdPolicy(name, fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage authorization policy",
		Long:  `Manage authorization policy`,
		Run:   cmdutil.DefaultSubCommandRun(errOut),
	}

	cmds.AddCommand(adminpolicy.NewCmdWhoCan(adminpolicy.WhoCanRecommendedName, fullName+" "+adminpolicy.WhoCanRecommendedName, f, out))
	cmds.AddCommand(adminpolicy.NewCmdCanI(adminpolicy.CanIRecommendedName, fullName+" "+adminpolicy.CanIRecommendedName, f, out))
	cmds.AddCommand(adminpolicy.NewCmdSccSubjectReview(adminpolicy.SubjectReviewRecommendedName, fullName+" "+adminpolicy.SubjectReviewRecommendedName, f, out))
	cmds.AddCommand(adminpolicy.NewCmdSccReview(adminpolicy.ReviewRecommendedName, fullName+" "+adminpolicy.ReviewRecommendedName, f, out))

	cmds.AddCommand(adminpolicy.NewCmdAddRoleToUser(adminpolicy.AddRoleToUserRecommendedName, fullName+" "+adminpolicy.AddRoleToUserRecommendedName, f, out, errOut))
	cmds.AddCommand(adminpolicy.NewCmdRemoveRoleFromUser(adminpolicy.RemoveRoleFromUserRecommendedName, fullName+" "+adminpolicy.RemoveRoleFromUserRecommendedName, f, out, errOut))
	cmds.AddCommand(adminpolicy.NewCmdRemoveUserFromProject(adminpolicy.RemoveUserRecommendedName, fullName+" "+adminpolicy.RemoveUserRecommendedName, f, out))
	cmds.AddCommand(adminpolicy.NewCmdAddRoleToGroup(adminpolicy.AddRoleToGroupRecommendedName, fullName+" "+adminpolicy.AddRoleToGroupRecommendedName, f, out, errOut))
	cmds.AddCommand(adminpolicy.NewCmdRemoveRoleFromGroup(adminpolicy.RemoveRoleFromGroupRecommendedName, fullName+" "+adminpolicy.RemoveRoleFromGroupRecommendedName, f, out, errOut))
	cmds.AddCommand(adminpolicy.NewCmdRemoveGroupFromProject(adminpolicy.RemoveGroupRecommendedName, fullName+" "+adminpolicy.RemoveGroupRecommendedName, f, out))

	return cmds
}
