package project

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/experimental/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

type NewProjectOptions struct {
	ProjectName string
	DisplayName string
	Description string

	Client client.Interface

	AdminRole             string
	MasterPolicyNamespace string
	AdminUser             string
}

func NewCmdNewProject(f *clientcmd.Factory, parentName, name string) *cobra.Command {
	options := &NewProjectOptions{}

	cmd := &cobra.Command{
		Use:   name + " <project-name>",
		Short: "create a new project",
		Long:  `create a new project`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			var err error
			if options.Client, _, err = f.Clients(cmd); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if err := options.Run(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	// TODO remove once we have global policy objects
	cmd.Flags().StringVar(&options.MasterPolicyNamespace, "master-policy-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "master policy namespace")
	cmd.Flags().StringVar(&options.AdminRole, "admin-role", bootstrappolicy.AdminRoleName, "project admin role name in the master policy namespace")
	cmd.Flags().StringVar(&options.AdminUser, "admin", "", "project admin username")
	cmd.Flags().StringVar(&options.DisplayName, "display-name", "", "project display name")
	cmd.Flags().StringVar(&options.Description, "description", "", "project description")

	return cmd
}

func (o *NewProjectOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) != 1 {
		cmd.Help()
		return false
	}

	o.ProjectName = args[0]

	return true
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
	project.DisplayName = o.DisplayName
	project.Annotations = make(map[string]string)
	project.Annotations["description"] = o.Description
	project, err := o.Client.Projects().Create(project)
	if err != nil {
		return err
	}

	if len(o.AdminUser) != 0 {
		adduser := &policy.AddUserOptions{
			RoleNamespace:    o.MasterPolicyNamespace,
			RoleName:         o.AdminRole,
			BindingNamespace: project.Name,
			Client:           o.Client,
			Users:            []string{o.AdminUser},
		}

		if err := adduser.Run(); err != nil {
			fmt.Printf("The project %v was created, but %v could not be added to the %v role.\n", o.ProjectName, o.AdminUser, o.AdminRole)
			fmt.Printf("To add the user to the existing project, run\n\n\topenshift ex policy add-user --namespace=%v --role-namespace=%v %v %v\n", o.ProjectName, o.MasterPolicyNamespace, o.AdminRole, o.AdminUser)
			return err
		}
	}

	return nil
}
