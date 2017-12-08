package policy

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

const (
	AddSCCToGroupRecommendedName      = "add-scc-to-group"
	AddSCCToUserRecommendedName       = "add-scc-to-user"
	RemoveSCCFromGroupRecommendedName = "remove-scc-from-group"
	RemoveSCCFromUserRecommendedName  = "remove-scc-from-user"
)

var (
	addSCCToUserExample = templates.Examples(`
		# Add the 'restricted' security context contraint to user1 and user2
	  %[1]s restricted user1 user2

	  # Add the 'privileged' security context contraint to the service account serviceaccount1 in the current namespace
	  %[1]s privileged -z serviceaccount1`)
)

type SCCModificationOptions struct {
	SCCName      string
	SCCInterface securitytypedclient.SecurityContextConstraintsInterface

	DefaultSubjectNamespace string
	Subjects                []kapi.ObjectReference

	IsGroup bool
	DryRun  bool
	Output  string

	PrintObj func(runtime.Object) error
	Out      io.Writer
}

func NewCmdAddSCCToGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &SCCModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " SCC GROUP [GROUP ...]",
		Short: "Add groups to a security context constraint",
		Long:  `Add groups to a security context constraint`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteGroups(f, cmd, args, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.AddSCC(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func NewCmdAddSCCToUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &SCCModificationOptions{}
	saNames := []string{}

	cmd := &cobra.Command{
		Use:     name + " SCC (USER | -z SERVICEACCOUNT) [USER ...]",
		Short:   "Add users or serviceaccount to a security context constraint",
		Long:    `Add users or serviceaccount to a security context constraint`,
		Example: fmt.Sprintf(addSCCToUserExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUsers(f, cmd, args, saNames, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.AddSCC(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringSliceVarP(&saNames, "serviceaccount", "z", saNames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func NewCmdRemoveSCCFromGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &SCCModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " SCC GROUP [GROUP ...]",
		Short: "Remove group from scc",
		Long:  `Remove group from scc`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteGroups(f, cmd, args, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.RemoveSCC(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func NewCmdRemoveSCCFromUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &SCCModificationOptions{}
	saNames := []string{}

	cmd := &cobra.Command{
		Use:   name + " SCC USER [USER ...]",
		Short: "Remove user from scc",
		Long:  `Remove user from scc`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUsers(f, cmd, args, saNames, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.RemoveSCC(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringSliceVarP(&saNames, "serviceaccount", "z", saNames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *SCCModificationOptions) CompleteUsers(f *clientcmd.Factory, cmd *cobra.Command, args []string, saNames []string, out io.Writer) error {
	if len(args) < 1 {
		return errors.New("you must specify a scc")
	}

	o.Out = out
	o.SCCName = args[0]
	o.Subjects = authorizationapi.BuildSubjects(args[1:], []string{})

	if (len(o.Subjects) == 0) && (len(saNames) == 0) {
		return errors.New("you must specify at least one user or service account")
	}

	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")
	o.Output = kcmdutil.GetFlagString(cmd, "output")

	mapper, _ := f.Object()

	o.PrintObj = func(obj runtime.Object) error {
		return f.PrintObject(cmd, false, mapper, obj, out)
	}

	securityClient, err := f.OpenshiftInternalSecurityClient()
	if err != nil {
		return err
	}
	o.SCCInterface = securityClient.Security().SecurityContextConstraints()

	o.DefaultSubjectNamespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	for _, sa := range saNames {
		o.Subjects = append(o.Subjects, kapi.ObjectReference{Namespace: o.DefaultSubjectNamespace, Name: sa, Kind: "ServiceAccount"})
	}

	return nil
}

func (o *SCCModificationOptions) CompleteGroups(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("you must specify at least two arguments: <scc> <group> [group]...")
	}

	o.Out = out
	o.Output = kcmdutil.GetFlagString(cmd, "output")

	mapper, _ := f.Object()

	o.PrintObj = func(obj runtime.Object) error {
		return f.PrintObject(cmd, false, mapper, obj, out)
	}

	o.IsGroup = true
	o.SCCName = args[0]
	o.Subjects = authorizationapi.BuildSubjects([]string{}, args[1:])

	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")

	securityClient, err := f.OpenshiftInternalSecurityClient()
	if err != nil {
		return err
	}
	o.SCCInterface = securityClient.Security().SecurityContextConstraints()

	o.DefaultSubjectNamespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	return nil
}

func (o *SCCModificationOptions) AddSCC() error {
	scc, err := o.SCCInterface.Get(o.SCCName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	users, groups := authorizationapi.StringSubjectsFor(o.DefaultSubjectNamespace, o.Subjects)
	usersToAdd, _ := diff(users, scc.Users)
	groupsToAdd, _ := diff(groups, scc.Groups)

	scc.Users = append(scc.Users, usersToAdd...)
	scc.Groups = append(scc.Groups, groupsToAdd...)

	if len(o.Output) > 0 && o.PrintObj != nil {
		return o.PrintObj(scc)
	}

	if o.DryRun {
		printSuccess(o.SCCName, true, o.IsGroup, users, groups, o.DryRun, o.Out)
		return nil
	}

	_, err = o.SCCInterface.Update(scc)
	if err != nil {
		return err
	}

	printSuccess(o.SCCName, true, o.IsGroup, users, groups, o.DryRun, o.Out)
	return nil
}

func (o *SCCModificationOptions) RemoveSCC() error {
	scc, err := o.SCCInterface.Get(o.SCCName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	users, groups := authorizationapi.StringSubjectsFor(o.DefaultSubjectNamespace, o.Subjects)
	_, remainingUsers := diff(users, scc.Users)
	_, remainingGroups := diff(groups, scc.Groups)

	scc.Users = remainingUsers
	scc.Groups = remainingGroups

	if len(o.Output) > 0 && o.PrintObj != nil {
		return o.PrintObj(scc)
	}

	if o.DryRun {
		printSuccess(o.SCCName, false, o.IsGroup, users, groups, o.DryRun, o.Out)
		return nil
	}

	_, err = o.SCCInterface.Update(scc)
	if err != nil {
		return err
	}

	printSuccess(o.SCCName, false, o.IsGroup, users, groups, o.DryRun, o.Out)
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

// prints affirmative output
func printSuccess(scc string, didAdd bool, isGroup bool, usersToAdd, groupsToAdd []string, dryRun bool, out io.Writer) {
	verb := "removed from"
	allTargets := fmt.Sprintf("%q", usersToAdd)
	dryRunText := ""

	if isGroup {
		allTargets = fmt.Sprintf("%q", groupsToAdd)
	}
	if didAdd {
		verb = "added to"
	}
	if isGroup {
		verb += " groups"
	}

	msg := "scc %q %s: %s%s"
	if dryRun {
		dryRunText = " (dry run)"
	}

	fmt.Fprintf(out, msg+"\n", scc, verb, allTargets, dryRunText)
}
