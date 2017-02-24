package create

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// ClusterRoleRecommendedName is the recommended name for oc create clusterrole.
const ClusterRoleRecommendedName = "clusterrole"

var createClusterRoleExample = templates.Examples(`
	# TODO
	%[1]s dev --resources a,b,c --verbs x,y,z`)

type CreateClusterRoleOptions struct {
	ClusterRoleClient client.ClusterRoleInterface

	Name      string
	Resources []string
	Verbs     []string
}

// NewCmdCreateClusterRole is a macro command to create a new cluster role.
func NewCmdCreateClusterRole(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &CreateClusterRoleOptions{}

	cmd := &cobra.Command{
		Use:     name + " <role-name>",
		Short:   "Create a new cluster role",
		Long:    "Create a new cluster role for specified resources and verbs.",
		Example: fmt.Sprintf(createClusterRoleExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}
			if err := options.Validate(); err != nil {
				kcmdutil.CheckErr(err)
			}
			if err := options.CreateRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringSliceVarP(&options.Resources, "resources", "", options.Resources, "list of resources (separated by comma)")
	cmd.Flags().StringSliceVarP(&options.Verbs, "verbs", "", options.Verbs, "list of verbs (separated by comma)")

	return cmd
}

// Complete completes all the required options.
func (o *CreateClusterRoleOptions) Complete(f *clientcmd.Factory, args []string) error {
	if len(args) == 0 {
		return errors.New("you must specify role name")
	}

	o.Name = args[0]

	osClient, _, _, err := f.Clients()
	if err != nil {
		return err
	}

	o.ClusterRoleClient = osClient.ClusterRoles()

	return nil
}

// Validate validates all the required options.
func (o *CreateClusterRoleOptions) Validate() error {
	if len(o.Name) == 0 {
		return errors.New("Name is required")
	}
	if len(o.Resources) == 0 {
		return errors.New("Resources is required")
	}
	if len(o.Verbs) == 0 {
		return errors.New("Verbs is required")
	}
	return nil
}

// CreateRole implements all the necessary functionality for creating cluster role.
func (o *CreateClusterRoleOptions) CreateRole() error {
	rule, err := authapi.NewRule(o.Verbs...).Resources(o.Resources...).Groups("").Rule()
	if err != nil {
		return err
	}

	role := &authapi.ClusterRole{}
	role.Name = o.Name
	role.Rules = []authapi.PolicyRule{rule}

	_, err = o.ClusterRoleClient.Create(role)
	if err != nil {
		return err
	}

	fmt.Printf("clusterrole %q created\n", role.Name)

	return nil
}
