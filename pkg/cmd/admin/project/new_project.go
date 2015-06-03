package project

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

const NewProjectRecommendedName = "new-project"

type NewProjectOptions struct {
	ProjectName  string
	DisplayName  string
	Description  string
	NodeSelector string

	Client client.Interface

	AdminRole string
	AdminUser string
}

const newProjectLong = `Create a new project

Use this command to create a project. You may optionally specify metadata about the project,
an admin user (and role, if you want to use a non-default admin role), and a node selector
to restrict which nodes pods in this project can be scheduled to.
`

func NewCmdNewProject(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewProjectOptions{}

	cmd := &cobra.Command{
		Use:   name + " NAME [--display-name=DISPLAYNAME] [--description=DESCRIPTION]",
		Short: "Create a new project",
		Long:  newProjectLong,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				kcmdutil.CheckErr(err)
			}

			// We can't depend on len(options.NodeSelector) > 0 as node-selector="" is valid
			// and we want to populate node selector as project annotation only if explicitly set by user
			useNodeSelector := cmd.Flag("node-selector").Changed

			if err := options.Run(useNodeSelector); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.AdminRole, "admin-role", bootstrappolicy.AdminRoleName, "project admin role name in the cluster policy")
	cmd.Flags().StringVar(&options.AdminUser, "admin", "", "project admin username")
	cmd.Flags().StringVar(&options.DisplayName, "display-name", "", "project display name")
	cmd.Flags().StringVar(&options.Description, "description", "", "project description")
	cmd.Flags().StringVar(&options.NodeSelector, "node-selector", "", "Restrict pods onto nodes matching given label selector. Format: '<key1>=<value1>, <key2>=<value2>...'. Specifying \"\" means any node, not default. If unspecified, cluster default node selector will be used.")

	return cmd
}

func (o *NewProjectOptions) complete(args []string) error {
	if len(args) != 1 {
		return errors.New("You must specify one argument: project name")
	}

	o.ProjectName = args[0]
	return nil
}

func (o *NewProjectOptions) Run(useNodeSelector bool) error {
	if _, err := o.Client.Projects().Get(o.ProjectName); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
	} else {
		return fmt.Errorf("project %v already exists", o.ProjectName)
	}

	project := &projectapi.Project{}
	project.Name = o.ProjectName
	project.Annotations = make(map[string]string)
	project.Annotations["description"] = o.Description
	project.Annotations["displayName"] = o.DisplayName
	if useNodeSelector {
		project.Annotations[projectapi.ProjectNodeSelectorParam] = o.NodeSelector
	}
	project, err := o.Client.Projects().Create(project)
	if err != nil {
		return err
	}

	if len(o.AdminUser) != 0 {
		adduser := &policy.RoleModificationOptions{
			RoleName:            o.AdminRole,
			RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(project.Name, o.Client),
			Users:               []string{o.AdminUser},
		}

		if err := adduser.AddRole(); err != nil {
			fmt.Printf("The project %v was created, but %v could not be added to the %v role.\n", o.ProjectName, o.AdminUser, o.AdminRole)
			fmt.Printf("To add the user to the existing project, run\n\n\tosadm policy add-role-to-user --namespace=%v %v %v\n", o.ProjectName, o.AdminRole, o.AdminUser)
			return err
		}
	}

	return nil
}
