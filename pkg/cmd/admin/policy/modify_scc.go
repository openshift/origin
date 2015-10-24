package policy

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
)

const (
	AddSCCToGroupRecommendedName      = "add-scc-to-group"
	AddSCCToUserRecommendedName       = "add-scc-to-user"
	RemoveSCCFromGroupRecommendedName = "remove-scc-from-group"
	RemoveSCCFromUserRecommendedName  = "remove-scc-from-user"
)

type SCCModificationOptions struct {
	SCCName      string
	SCCInterface kclient.SecurityContextConstraintsInterface

	DefaultSubjectNamespace string
	Subjects                []kapi.ObjectReference
}

func NewCmdAddSCCToGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &SCCModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " SCC GROUP [GROUP ...]",
		Short: "Add groups to a security context constraint",
		Long:  `Add groups to a security context constraint`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteGroups(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.AddSCC(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func NewCmdAddSCCToUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &SCCModificationOptions{}
	saNames := util.StringList{}

	cmd := &cobra.Command{
		Use:   name + " SCC USER [USER ...]",
		Short: "Add users to a security context constraint",
		Long:  `Add users to a security context constraint`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUsers(f, args, saNames); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.AddSCC(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().VarP(&saNames, "serviceaccount", "z", "service account in the current namespace to use as a user")

	return cmd
}

func NewCmdRemoveSCCFromGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &SCCModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " SCC GROUP [GROUP ...]",
		Short: "Remove group from scc",
		Long:  `Remove group from scc`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteGroups(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RemoveSCC(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func NewCmdRemoveSCCFromUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &SCCModificationOptions{}
	saNames := util.StringList{}

	cmd := &cobra.Command{
		Use:   name + " SCC USER [USER ...]",
		Short: "Remove user from scc",
		Long:  `Remove user from scc`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUsers(f, args, saNames); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RemoveSCC(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().VarP(&saNames, "serviceaccount", "z", "service account in the current namespace to use as a user")

	return cmd
}

func (o *SCCModificationOptions) CompleteUsers(f *clientcmd.Factory, args []string, saNames util.StringList) error {
	if (len(args) < 2) && (len(saNames) == 0) {
		return errors.New("you must specify at least two arguments (<scc> <user> [user]...) or a service account (<scc> -z <service account name>) ")
	}

	o.SCCName = args[0]
	o.Subjects = authorizationapi.BuildSubjects(args[1:], []string{}, uservalidation.ValidateUserName, uservalidation.ValidateGroupName)

	var err error
	_, o.SCCInterface, err = f.Clients()
	if err != nil {
		return err
	}

	o.DefaultSubjectNamespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	for _, sa := range saNames {
		o.Subjects = append(o.Subjects, kapi.ObjectReference{Namespace: o.DefaultSubjectNamespace, Name: sa, Kind: "ServiceAccount"})
	}

	return nil
}

func (o *SCCModificationOptions) CompleteGroups(f *clientcmd.Factory, args []string) error {
	if len(args) < 2 {
		return errors.New("you must specify at least two arguments: <scc> <group> [group]...")
	}

	o.SCCName = args[0]
	o.Subjects = authorizationapi.BuildSubjects([]string{}, args[1:], uservalidation.ValidateUserName, uservalidation.ValidateGroupName)

	var err error
	_, o.SCCInterface, err = f.Clients()
	if err != nil {
		return err
	}

	o.DefaultSubjectNamespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	return nil
}

func (o *SCCModificationOptions) AddSCC() error {
	scc, err := o.SCCInterface.SecurityContextConstraints().Get(o.SCCName)
	if err != nil {
		return err
	}

	users, groups := authorizationapi.StringSubjectsFor(o.DefaultSubjectNamespace, o.Subjects)
	usersToAdd, _ := diff(users, scc.Users)
	groupsToAdd, _ := diff(groups, scc.Groups)

	scc.Users = append(scc.Users, usersToAdd...)
	scc.Groups = append(scc.Groups, groupsToAdd...)

	_, err = o.SCCInterface.SecurityContextConstraints().Update(scc)
	if err != nil {
		return err
	}

	return nil
}

func (o *SCCModificationOptions) RemoveSCC() error {
	scc, err := o.SCCInterface.SecurityContextConstraints().Get(o.SCCName)
	if err != nil {
		return err
	}

	users, groups := authorizationapi.StringSubjectsFor(o.DefaultSubjectNamespace, o.Subjects)
	_, remainingUsers := diff(users, scc.Users)
	_, remainingGroups := diff(groups, scc.Groups)

	scc.Users = remainingUsers
	scc.Groups = remainingGroups

	_, err = o.SCCInterface.SecurityContextConstraints().Update(scc)
	if err != nil {
		return err
	}

	return nil
}

func diff(lhsSlice, rhsSlice []string) (lhsOnly []string, rhsOnly []string) {
	return singleDiff(lhsSlice, rhsSlice), singleDiff(rhsSlice, lhsSlice)
}

func singleDiff(lhsSlice, rhsSlice []string) (lhsOnly []string) {
	for _, lhs := range lhsSlice {
		found := false
		for _, rhs := range rhsSlice {
			if lhs == rhs {
				found = true
				break
			}
		}

		if !found {
			lhsOnly = append(lhsOnly, lhs)
		}
	}

	return lhsOnly
}
