package deployment

import (
	"github.com/openshift/origin/pkg/cmd/base"
	api "github.com/openshift/origin/pkg/deploy/api"
	"github.com/spf13/cobra"
)

// Root command

func NewCmdDeployment(resource string) *cobra.Command {
	deploymentCmd := base.CreateCmdRoot(resource)

	deploymentListCmd := NewCmdDeploymentList(resource, "list")
	deploymentShowCmd := NewCmdDeploymentShow(resource, "show")
	deploymentCreateCmd := NewCmdDeploymentCreate(resource, "create")
	deploymentUpdateCmd := NewCmdDeploymentUpdate(resource, "update")
	deploymentRemoveCmd := NewCmdDeploymentRemove(resource, "remove")

	deploymentCmd.AddCommand(deploymentListCmd)
	deploymentCmd.AddCommand(deploymentShowCmd)
	deploymentCmd.AddCommand(deploymentCreateCmd)
	deploymentCmd.AddCommand(deploymentUpdateCmd)
	deploymentCmd.AddCommand(deploymentRemoveCmd)

	return deploymentCmd
}

// Subcommands

func NewCmdDeploymentList(resource string, name string) *cobra.Command {
	return base.CreateCmdList(resource, name, ListDeployments)
}

func NewCmdDeploymentShow(resource string, name string) *cobra.Command {
	return base.CreateCmdShow(resource, name, ShowDeployment)
}

func NewCmdDeploymentCreate(resource string, name string) *cobra.Command {
	return base.CreateCmdCreate(resource, name, CreateDeployment)
}

func NewCmdDeploymentUpdate(resource string, name string) *cobra.Command {
	return base.CreateCmdUpdate(resource, name, UpdateDeployment)
}

func NewCmdDeploymentRemove(resource string, name string) *cobra.Command {
	return base.CreateCmdRemove(resource, name, RemoveDeployment)
}

// Executors

func ListDeployments() (interface{}, error) {
	return api.DeploymentList{}.Items, nil
}

func ShowDeployment(id string) (interface{}, error) {
	return nil, nil
}

func CreateDeployment(payload interface{}) (string, error) {
	return "", nil
}

func UpdateDeployment(id string, payload interface{}) (string, error) {
	return id, nil
}

func RemoveDeployment(id string) (string, error) {
	return id, nil
}
