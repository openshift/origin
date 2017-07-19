package groups

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kprinters "k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
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
	GroupClient client.GroupInterface

	Group string
	Users []string

	Out     io.Writer
	Printer kprinters.ResourcePrinterFunc
}

func NewCmdNewGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewGroupOptions{Out: out}

	cmd := &cobra.Command{
		Use:     name + " GROUP [USER ...]",
		Short:   "Create a new group",
		Long:    newLong,
		Example: fmt.Sprintf(newExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, cmd, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.AddGroup())
		},
	}

	kcmdutil.AddPrinterFlags(cmd)

	return cmd
}

func (o *NewGroupOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("You must specify at least one argument: GROUP [USER ...]")
	}

	o.Group = args[0]
	if len(args) > 1 {
		o.Users = append(o.Users, args[1:]...)
	}

	osClient, _, err := f.Clients()
	if err != nil {
		return err
	}

	o.GroupClient = osClient.Groups()

	printer, err := f.PrinterForCommand(cmd, true, nil, kprinters.PrintOptions{})
	if err != nil {
		return err
	}

	if printer != nil {
		o.Printer = printer.PrintObj
	} else {
		o.Printer = func(obj runtime.Object, out io.Writer) error {
			mapper, _ := f.Object()
			return f.PrintObject(cmd, true, mapper, obj, out)
		}
	}

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
	group := &userapi.Group{}
	group.Name = o.Group

	usedNames := sets.String{}
	for _, user := range o.Users {
		if usedNames.Has(user) {
			continue
		}
		usedNames.Insert(user)

		group.Users = append(group.Users, user)
	}

	actualGroup, err := o.GroupClient.Create(group)
	if err != nil {
		return err
	}

	return o.Printer(actualGroup, o.Out)
}
