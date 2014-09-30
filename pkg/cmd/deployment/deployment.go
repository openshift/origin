package deployment

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/cmd/base"
	"github.com/openshift/origin/pkg/cmd/config"
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
	cli := config.NewClient()
	deployments, err := cli.ListDeployments(labels.Everything())
	return deployments.Items, err
}

func ShowDeployment(id string) (interface{}, error) {
	return nil, nil
}

func CreateDeployment(payload interface{}) (string, error) {
	return "", nil
}

func UpdateDeployment(id string, payload interface{}) error {
	return nil
}

func RemoveDeployment(id string) error {
	return nil
}
