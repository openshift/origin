package project

import (
	"fmt"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

type newProjectOptions struct {
	projectName string
	displayName string
	description string

	client client.Interface

	adminRole             string
	masterPolicyNamespace string
	adminUser             string
}

func NewCmdNewProject(f *clientcmd.Factory, parentName, name string) *cobra.Command {
	options := &newProjectOptions{}

	cmd := &cobra.Command{
		Use:   name + " <project-name>",
		Short: "create a new project",
		Long:  `create a new project`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			var err error
			if options.client, _, err = f.Clients(cmd); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if err := options.run(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	// TODO remove once we have global policy objects
	cmd.Flags().StringVar(&options.masterPolicyNamespace, "master-policy-namespace", "master", "master policy namespace")
	cmd.Flags().StringVar(&options.adminRole, "admin-role", "admin", "project admin role name in the master policy namespace")
	cmd.Flags().StringVar(&options.adminUser, "admin", "", "project admin username")
	cmd.Flags().StringVar(&options.displayName, "display-name", "", "project display name")
	cmd.Flags().StringVar(&options.description, "description", "", "project description")

	return cmd
}

func (o *newProjectOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) != 1 {
		cmd.Help()
		return false
	}

	o.projectName = args[0]

	return true
}

func (o *newProjectOptions) run() error {
	if _, err := o.client.Projects().Get(o.projectName); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
	} else {
		return fmt.Errorf("project %v already exists", o.projectName)
	}

	project := &projectapi.Project{}
	project.Name = o.projectName
	project.DisplayName = o.displayName
	project.Annotations = make(map[string]string)
	project.Annotations["description"] = o.description
	project, err := o.client.Projects().Create(project)
	if err != nil {
		return err
	}

	if len(o.adminUser) != 0 {
		adminRoleBinding := &authorizationapi.RoleBinding{}

		adminRoleBinding.Name = "admins"
		adminRoleBinding.RoleRef.Namespace = o.masterPolicyNamespace
		adminRoleBinding.RoleRef.Name = o.adminRole
		adminRoleBinding.UserNames = []string{o.adminUser}

		_, err := o.client.RoleBindings(project.Name).Create(adminRoleBinding)
		if err != nil {
			fmt.Printf("The project %v was created, but %v could not be added to the %v role.\n", o.projectName, o.adminUser, o.adminRole)
			fmt.Printf("To add the user to the existing project, run\n\n\topenshift ex policy add-user --namespace=%v --role-namespace=%v %v %v\n", o.projectName, o.masterPolicyNamespace, o.adminRole, o.adminUser)
			return err
		}
	}

	return nil
}
