package new

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"

	userapiv1 "github.com/openshift/api/user/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/origin/pkg/oc/util/ocscheme"
)

const NewGroupRecommendedName = "new"

var (
	newLong = templates.LongDesc(`
		Create a new group.

		This command will create a new group with an optional list of users.`)

	newExample = templates.Examples(`
		# Add a group with no users
	  %[1]s my-group

	  # Add a group with two users
	  %[1]s my-group user1 user2

	  # Add a group with one user and shorter output
	  %[1]s my-group user1 -o name`)
)

type NewGroupOptions struct {
	PrintFlags  *genericclioptions.PrintFlags
	GroupClient userv1client.GroupInterface

	Group string
	Users []string

	DryRun bool

	Printer printers.ResourcePrinter

	genericclioptions.IOStreams
}

func NewNewGroupOptions(streams genericclioptions.IOStreams) *NewGroupOptions {
	return &NewGroupOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(ocscheme.PrintingInternalScheme),
		IOStreams:  streams,
	}
}

func NewCmdNewGroup(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNewGroupOptions(streams)

	cmd := &cobra.Command{
		Use:     name + " GROUP [USER ...]",
		Short:   "Create a new group",
		Long:    newLong,
		Example: fmt.Sprintf(newExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.Complete(f, cmd, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.AddGroup())
		},
	}

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "Display the group that would be created, without actually creating the group.")
	return cmd
}

func (o *NewGroupOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("You must specify at least one argument: GROUP [USER ...]")
	}

	o.Group = args[0]
	if len(args) > 1 {
		o.Users = append(o.Users, args[1:]...)
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	userClient, err := userv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}

	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	o.GroupClient = userClient.Groups()
	return nil
}

func (o *NewGroupOptions) Validate() error {
	if len(o.Group) == 0 {
		return fmt.Errorf("Group is required")
	}
	if o.GroupClient == nil {
		return fmt.Errorf("GroupClient is required")
	}
	if o.Out == nil {
		return fmt.Errorf("Out is required")
	}
	if o.Printer == nil {
		return fmt.Errorf("Printer is required")
	}

	return nil
}

func (o *NewGroupOptions) AddGroup() error {
	group := &userapiv1.Group{}
	group.Name = o.Group

	usedNames := sets.String{}
	for _, user := range o.Users {
		if usedNames.Has(user) {
			continue
		}
		usedNames.Insert(user)

		group.Users = append(group.Users, user)
	}

	if o.DryRun {
		return o.Printer.PrintObj(group, o.Out)
	}

	var err error
	group, err = o.GroupClient.Create(group)
	if err != nil {
		return err
	}

	return o.Printer.PrintObj(group, o.Out)
}
