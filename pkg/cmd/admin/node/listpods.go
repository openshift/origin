package node

import (
	"fmt"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
)

type ListPodsOptions struct {
	Options *NodeOptions
}

func (l *ListPodsOptions) AddFlags(cmd *cobra.Command) {
	kcmdutil.AddPrinterFlags(cmd)
}

func (l *ListPodsOptions) Run() error {
	nodes, err := l.Options.GetNodes()
	if err != nil {
		return err
	}

	var printer kubectl.ResourcePrinter
	if l.Options.CmdPrinterOutput {
		printer = l.Options.CmdPrinter
	} else {
		printer, _, err = l.Options.GetPrintersByResource("pod")
		if err != nil {
			return err
		}
	}

	errList := []error{}
	for _, node := range nodes {
		err := l.runListPods(node, printer)
		if err != nil {
			// Don't bail out if one node fails
			errList = append(errList, err)
		}
	}
	return kerrors.NewAggregate(errList)
}

func (l *ListPodsOptions) runListPods(node *kapi.Node, printer kubectl.ResourcePrinter) error {
	labelSelector, err := labels.Parse(l.Options.PodSelector)
	if err != nil {
		return err
	}
	fieldSelector := fields.Set{GetPodHostFieldLabel(node.TypeMeta.APIVersion): node.ObjectMeta.Name}.AsSelector()

	// Filter all pods that satisfies pod label selector and belongs to the given node
	pods, err := l.Options.Kclient.Pods(kapi.NamespaceAll).List(labelSelector, fieldSelector)
	if err != nil {
		return err
	}
	fmt.Fprintln(l.Options.Writer, "\nListing matched pods on node: ", node.ObjectMeta.Name, "\n")
	printer.PrintObj(pods, l.Options.Writer)

	return err
}
