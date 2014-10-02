package pod

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/cmd/config"
	r "github.com/openshift/origin/pkg/cmd/resource"
	"github.com/spf13/cobra"
)

// Root command

func NewCmdPod(resource string) *cobra.Command {
	podCmd := r.NewCmdRoot(resource)
	podCmd.AddCommand(NewCmdPodList(resource, "list"))
	return podCmd
}

// Subcommands

func NewCmdPodList(resource string, name string) *cobra.Command {
	return r.NewCmdList(resource, name, ListPods)
}

// Executors

func ListPods() (interface{}, error) {
	cli := config.NewKubeClient()
	pods, err := cli.ListPods(labels.Everything())
	return pods.Items, err
}
