package users

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	userv1 "github.com/openshift/api/user/v1"
	userv1typedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
)

const AddRecommendedName = "add-users"

var (
	addLong = templates.LongDesc(`
		Add users to a group.

		This command will append unique users to the list of members for a group.`)

	addExample = templates.Examples(`
		# Add user1 and user2 to my-group
	%[1]s my-group user1 user2`)
)

type AddUsersOptions struct {
	GroupModificationOptions *GroupModificationOptions
}

func NewCmdAddUsers(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &AddUsersOptions{
		GroupModificationOptions: NewGroupModificationOptions(streams),
	}
	cmd := &cobra.Command{
		Use:     name + " GROUP USER [USER ...]",
		Short:   "Add users to a group",
		Long:    addLong,
		Example: fmt.Sprintf(addExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	o.GroupModificationOptions.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *AddUsersOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	return o.GroupModificationOptions.Complete(f, cmd, args)
}

func (o *AddUsersOptions) Run() error {
	group, err := o.GroupModificationOptions.GroupClient.Groups().Get(o.GroupModificationOptions.Group, metav1.GetOptions{})
	if err != nil {
		return err
	}

	existingUsers := sets.NewString(group.Users...)
	for _, user := range o.GroupModificationOptions.Users {
		if existingUsers.Has(user) {
			continue
		}

		group.Users = append(group.Users, user)
	}

	if !o.GroupModificationOptions.DryRun {
		group, err = o.GroupModificationOptions.GroupClient.Groups().Update(group)
		if err != nil {
			return err
		}
	}

	return o.GroupModificationOptions.PrintObj("added", group)
}

type GroupModificationOptions struct {
	PrintFlags *genericclioptions.PrintFlags
	ToPrinter  func(string) (printers.ResourcePrinter, error)

	GroupClient userv1typedclient.GroupsGetter

	Group  string
	Users  []string
	DryRun bool

	genericclioptions.IOStreams
}

func NewGroupModificationOptions(streams genericclioptions.IOStreams) *GroupModificationOptions {
	return &GroupModificationOptions{
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

func (o *GroupModificationOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return errors.New("you must specify at least two arguments: GROUP USER [USER ...]")
	}

	o.Group = args[0]
	o.Users = append(o.Users, args[1:]...)

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.GroupClient, err = userv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		if o.DryRun {
			o.PrintFlags.Complete("%s (dry run)")
		}
		return o.PrintFlags.ToPrinter()
	}

	return nil
}

func (o *GroupModificationOptions) PrintObj(operation string, group *userv1.Group) error {
	allTargets := fmt.Sprintf("%q", o.Users)
	if len(o.Users) == 1 {
		allTargets = fmt.Sprintf("%q", o.Users[0])
	}
	printer, err := o.ToPrinter(fmt.Sprintf("%s: %s", operation, allTargets))
	if err != nil {
		return err
	}
	return printer.PrintObj(group, o.Out)
}
