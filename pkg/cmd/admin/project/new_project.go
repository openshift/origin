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

	AdminRole             string
	MasterPolicyNamespace string
	AdminUser             string
}

func NewCmdNewProject(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewProjectOptions{}

	cmd := &cobra.Command{
		Use:   name + " NAME [--display-name=DISPLAYNAME] [--description=DESCRIPTION]",
		Short: "create a new project",
		Long:  `create a new project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				kcmdutil.CheckErr(err)
			}
			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	// TODO remove once we have global policy objects
	cmd.Flags().StringVar(&options.MasterPolicyNamespace, "master-policy-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "master policy namespace")
	cmd.Flags().StringVar(&options.AdminRole, "admin-role", bootstrappolicy.AdminRoleName, "project admin role name in the master policy namespace")
	cmd.Flags().StringVar(&options.AdminUser, "admin", "", "project admin username")
	cmd.Flags().StringVar(&options.DisplayName, "display-name", "", "project display name")
	cmd.Flags().StringVar(&options.Description, "description", "", "project description")
	cmd.Flags().StringVar(&options.NodeSelector, "node-selector", "", "Restrict pods onto nodes matching given label selector")

	return cmd
}

func (o *NewProjectOptions) complete(args []string) error {
	if len(args) != 1 {
		return errors.New("You must specify one argument: project name")
	}

	o.ProjectName = args[0]
	return nil
}

func (o *NewProjectOptions) Run() error {
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
	project.Annotations["nodeSelector"] = o.NodeSelector
	project, err := o.Client.Projects().Create(project)
	if err != nil {
		return err
	}

	if len(o.AdminUser) != 0 {
		adduser := &policy.RoleModificationOptions{
			RoleNamespace:    o.MasterPolicyNamespace,
			RoleName:         o.AdminRole,
			BindingNamespace: project.Name,
			Client:           o.Client,
			Users:            []string{o.AdminUser},
		}

		if err := adduser.AddRole(); err != nil {
			fmt.Printf("The project %v was created, but %v could not be added to the %v role.\n", o.ProjectName, o.AdminUser, o.AdminRole)
			fmt.Printf("To add the user to the existing project, run\n\n\topenshift ex policy add-role-to-user --namespace=%v --role-namespace=%v %v %v\n", o.ProjectName, o.MasterPolicyNamespace, o.AdminRole, o.AdminUser)
			return err
		}
	}

	return nil
}
