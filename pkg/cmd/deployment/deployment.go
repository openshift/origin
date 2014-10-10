package deployment

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/cmd/config"
	r "github.com/openshift/origin/pkg/cmd/resource"
	"github.com/spf13/cobra"
)

// Root command

func NewCmdDeployment(resource string) *cobra.Command {
	deploymentCmd := r.NewCmdRoot(resource)

	deploymentCmd.AddCommand(NewCmdDeploymentList(resource, "list"))
	deploymentCmd.AddCommand(NewCmdDeploymentShow(resource, "show"))
	deploymentCmd.AddCommand(NewCmdDeploymentCreate(resource, "create"))
	deploymentCmd.AddCommand(NewCmdDeploymentUpdate(resource, "update"))
	deploymentCmd.AddCommand(NewCmdDeploymentRemove(resource, "remove"))

	return deploymentCmd
}

// Subcommands

func NewCmdDeploymentList(resource string, name string) *cobra.Command {
	return r.NewCmdList(resource, name, ListDeployments)
}

func NewCmdDeploymentShow(resource string, name string) *cobra.Command {
	return r.NewCmdShow(resource, name, ShowDeployment)
}

func NewCmdDeploymentCreate(resource string, name string) *cobra.Command {
	return r.NewCmdCreate(resource, name, CreateDeployment)
}

func NewCmdDeploymentUpdate(resource string, name string) *cobra.Command {
	return r.NewCmdUpdate(resource, name, UpdateDeployment)
}

func NewCmdDeploymentRemove(resource string, name string) *cobra.Command {
	return r.NewCmdRemove(resource, name, RemoveDeployment)
}

// Executors

func ListDeployments() (interface{}, error) {
	cli := config.NewOpenShiftClient()
	ctx := api.NewContext()
	deployments, err := cli.ListDeployments(ctx, labels.Everything())
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
