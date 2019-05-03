package policy

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericclioptions/printers"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

const (
	AddSCCToGroupRecommendedName      = "add-scc-to-group"
	AddSCCToUserRecommendedName       = "add-scc-to-user"
	RemoveSCCFromGroupRecommendedName = "remove-scc-from-group"
	RemoveSCCFromUserRecommendedName  = "remove-scc-from-user"
	RBACNamesFmt                      = "system:openshift:scc:%s"
)

var (
	addSCCToUserExample = templates.Examples(`
		# Add the 'restricted' security context contraint to user1 and user2
		%[1]s restricted user1 user2

		# Add the 'privileged' security context contraint to the service account serviceaccount1 in the current namespace
		%[1]s privileged -z serviceaccount1`)

	addSCCToGroupExample = templates.Examples(`
		# Add the 'restricted' security context contraint to group1 and group2
		%[1]s restricted group1 group2`)
)

type SCCModificationOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	ToPrinter func(string) (printers.ResourcePrinter, error)

	SCCName    string
	RbacClient rbacv1client.RbacV1Interface
	SANames    []string

	DefaultSubjectNamespace string
	Subjects                []rbacv1.Subject

	IsGroup bool
	DryRun  bool
	Output  string

	genericclioptions.IOStreams
}

func NewSCCModificationOptions(streams genericclioptions.IOStreams) *SCCModificationOptions {
	return &SCCModificationOptions{
		PrintFlags: genericclioptions.NewPrintFlags("added to").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

func NewCmdAddSCCToGroup(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSCCModificationOptions(streams)
	cmd := &cobra.Command{
		Use:     name + " SCC GROUP [GROUP ...]",
		Short:   "Add security context constraint to groups",
		Long:    `Add security context constraint to groups`,
		Example: fmt.Sprintf(addSCCToGroupExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.CompleteGroups(f, cmd, args))
			kcmdutil.CheckErr(o.AddSCC())
		},
	}

	kcmdutil.AddDryRunFlag(cmd)
	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func NewCmdAddSCCToUser(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSCCModificationOptions(streams)
	o.SANames = []string{}
	cmd := &cobra.Command{
		Use:     name + " SCC (USER | -z SERVICEACCOUNT) [USER ...]",
		Short:   "Add security context constraint to users or a service account",
		Long:    `Add security context constraint to users or a service account`,
		Example: fmt.Sprintf(addSCCToUserExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.CompleteUsers(f, cmd, args))
			kcmdutil.CheckErr(o.AddSCC())
		},
	}

	cmd.Flags().StringSliceVarP(&o.SANames, "serviceaccount", "z", o.SANames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func NewCmdRemoveSCCFromGroup(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSCCModificationOptions(streams)
	cmd := &cobra.Command{
		Use:   name + " SCC GROUP [GROUP ...]",
		Short: "Remove group from scc",
		Long:  `Remove group from scc`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.CompleteGroups(f, cmd, args))
			kcmdutil.CheckErr(o.RemoveSCC())
		},
	}

	kcmdutil.AddDryRunFlag(cmd)
	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func NewCmdRemoveSCCFromUser(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSCCModificationOptions(streams)
	o.SANames = []string{}
	cmd := &cobra.Command{
		Use:   name + " SCC USER [USER ...]",
		Short: "Remove user from scc",
		Long:  `Remove user from scc`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.CompleteUsers(f, cmd, args))
			kcmdutil.CheckErr(o.RemoveSCC())
		},
	}

	cmd.Flags().StringSliceVarP(&o.SANames, "serviceaccount", "z", o.SANames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func (o *SCCModificationOptions) CompleteUsers(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("you must specify a scc")
	}

	o.SCCName = args[0]
	o.Subjects = buildSubjects(args[1:], []string{})

	if (len(o.Subjects) == 0) && (len(o.SANames) == 0) {
		return errors.New("you must specify at least one user or service account")
	}

	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")
	o.Output = kcmdutil.GetFlagString(cmd, "output")

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = getRolesSuccessMessage(o.DryRun, operation, o.getSubjectNames())
		return o.PrintFlags.ToPrinter()
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.RbacClient, err = rbacv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.DefaultSubjectNamespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	for _, sa := range o.SANames {
		o.Subjects = append(o.Subjects, rbacv1.Subject{Namespace: o.DefaultSubjectNamespace, Name: sa, Kind: "ServiceAccount"})
	}

	return nil
}

func (o *SCCModificationOptions) CompleteGroups(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return errors.New("you must specify at least two arguments: <scc> <group> [group]...")
	}

	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")

	o.IsGroup = true
	o.SCCName = args[0]
	o.Subjects = buildSubjects([]string{}, args[1:])

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = getRolesSuccessMessage(o.DryRun, operation, o.getSubjectNames())
		return o.PrintFlags.ToPrinter()
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.RbacClient, err = rbacv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.DefaultSubjectNamespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	return nil
}

func (o *SCCModificationOptions) AddSCC() error {
	addSubjects := RoleModificationOptions{
		RoleKind:        "ClusterRole",
		RoleName:        fmt.Sprintf(RBACNamesFmt, o.SCCName),
		RoleBindingName: fmt.Sprintf(RBACNamesFmt, o.SCCName),

		RbacClient: o.RbacClient,

		Subjects: o.Subjects,
		Targets:  o.getSubjectNames(),

		PrintFlags: o.PrintFlags,
		ToPrinter:  o.ToPrinter,

		DryRun: o.DryRun,
	}
	addSubjects.IOStreams = o.IOStreams

	return addSubjects.AddRole()
}

func (o *SCCModificationOptions) RemoveSCC() error {
	removeSubjects := RoleModificationOptions{
		RoleKind:        "ClusterRole",
		RoleName:        fmt.Sprintf(RBACNamesFmt, o.SCCName),
		RoleBindingName: fmt.Sprintf(RBACNamesFmt, o.SCCName),

		RbacClient: o.RbacClient,

		Subjects: o.Subjects,
		Targets:  o.getSubjectNames(),

		PrintFlags: o.PrintFlags,
		ToPrinter:  o.ToPrinter,

		DryRun: o.DryRun,
	}
	removeSubjects.IOStreams = o.IOStreams

	return removeSubjects.RemoveRole()
}

func (o *SCCModificationOptions) getSubjectNames() []string {
	targets := make([]string, 0, len(o.Subjects))
	for _, s := range o.Subjects {
		targets = append(targets, s.Name)
	}
	return targets
}
