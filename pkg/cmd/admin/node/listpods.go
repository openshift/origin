package node

import (
	"fmt"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
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
		printer, err = l.Options.GetPrintersByResource(unversioned.GroupVersionResource{Resource: "pod"})
		if err != nil {
			return err
		}
	}

	// determine if printer kind is json or yaml and modify output
	// to combine all pod lists into a single list
	if l.Options.CmdPrinterOutput {
		errs := l.handleRESTOutput(nodes, printer)
		return kerrors.NewAggregate(errs)
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
	pods, err := l.Options.KubeClient.Pods(kapi.NamespaceAll).List(kapi.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector})
	if err != nil {
		return err
	}

	fmt.Fprint(l.Options.ErrWriter, "\nListing matched pods on node: ", node.ObjectMeta.Name, "\n\n")
	printer.PrintObj(pods, l.Options.Writer)

	return err
}

// handleRESTOutput receives a list of nodes, and a REST output type, and combines *kapi.PodList
// objects for every node, into a single list. This allows output containing multiple nodes to be
// printed to a single writer, and be easily parsed as a single data format.
func (l *ListPodsOptions) handleRESTOutput(nodes []*kapi.Node, printer kubectl.ResourcePrinter) []error {
	unifiedPodList := &kapi.PodList{}

	errList := []error{}
	for _, node := range nodes {
		labelSelector, err := labels.Parse(l.Options.PodSelector)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		fieldSelector := fields.Set{GetPodHostFieldLabel(node.TypeMeta.APIVersion): node.ObjectMeta.Name}.AsSelector()

		pods, err := l.Options.KubeClient.Pods(kapi.NamespaceAll).List(kapi.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector})
		if err != nil {
			errList = append(errList, err)
			continue
		}

		unifiedPodList.Items = append(unifiedPodList.Items, pods.Items...)
	}

	printer.PrintObj(unifiedPodList, l.Options.Writer)
	return errList
}
