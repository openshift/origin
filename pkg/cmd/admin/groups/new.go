package groups

import (
	"errors"
	"fmt"
	"io"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	userapi "github.com/openshift/origin/pkg/user/api"
)

const (
	NewGroupRecommendedName = "new"
	newLong                 = `
Create a new group.

This command will create a new group with an optional list of users.`

	newExample = `  # Add a group with no users
  $ %[1]s my-group

  # Add a group with two users
  $ %[1]s my-group user1 user2`
)

type NewGroupOptions struct {
	GroupClient client.GroupInterface

	Group string
	Users []string
}

func NewCmdNewGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewGroupOptions{}

	cmd := &cobra.Command{
		Use:     name + " GROUP [USER ...]",
		Short:   "Create a new group",
		Long:    newLong,
		Example: fmt.Sprintf(newExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			kcmdutil.CheckErr(options.AddGroup())
		},
	}

	return cmd
}

func (o *NewGroupOptions) Complete(f *clientcmd.Factory, args []string) error {
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

	_, err := o.GroupClient.Create(group)
	return err
}
