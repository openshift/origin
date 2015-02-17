package project

import (
	"fmt"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

type newProjectOptions struct {
	projectName  string
	displayName  string
	description  string
	clientConfig clientcmd.ClientConfig

	adminRole             string
	masterPolicyNamespace string
	adminUser             string
}

func NewCmdNewProject(name string) *cobra.Command {
	options := &newProjectOptions{}

	cmd := &cobra.Command{
		Use:   name + " <project-name>",
		Short: "create a new project",
		Long:  `create a new project`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			err := options.run()
			if err != nil {
				fmt.Printf("%v\n", err)
			}
		},
	}

	// Override global default to https and port 8443
	clientcmd.DefaultCluster.Server = "https://localhost:8443"
	clientConfig := cmdutil.DefaultClientConfig(cmd.Flags())
	options.clientConfig = clientConfig

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
	clientConfig, err := o.clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	client, err := client.New(clientConfig)
	if err != nil {
		return err
	}

	_, err = client.Projects().Get(o.projectName)
	projectFound := !kerrors.IsNotFound(err)
	if (err != nil) && (projectFound) {
		return err
	}
	if projectFound {
		return fmt.Errorf("project %v already exists", o.projectName)
	}

	project := &projectapi.Project{}
	project.Name = o.projectName
	project.DisplayName = o.displayName
	project.Annotations = make(map[string]string)
	project.Annotations["description"] = o.description
	project, err = client.Projects().Create(project)
	if err != nil {
		return err
	}

	if len(o.adminUser) != 0 {
		adminRoleBinding := &authorizationapi.RoleBinding{}

		adminRoleBinding.Name = "admins"
		adminRoleBinding.RoleRef.Namespace = o.masterPolicyNamespace
		adminRoleBinding.RoleRef.Name = o.adminRole
		adminRoleBinding.UserNames = []string{o.adminUser}

		_, err := client.RoleBindings(project.Name).Create(adminRoleBinding)
		if err != nil {
			fmt.Printf("The project %v was created, but %v could not be added to the %v role.\n", o.projectName, o.adminUser, o.adminRole)
			fmt.Printf("To add the user to the existing project, run\n\n\topenshift ex policy add-user --namespace=%v --role-namespace=%v %v %v\n", o.projectName, o.masterPolicyNamespace, o.adminRole, o.adminUser)
			return err
		}
	}

	return nil
}
