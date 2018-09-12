package node

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
)

type ListPodsOptions struct {
	Options *NodeOptions

	printPodHeaders bool
}

func (o *ListPodsOptions) AddFlags(cmd *cobra.Command) {
	kcmdutil.AddNoHeadersFlags(cmd)
	o.Options.PrintFlags.AddFlags(cmd)
}

func (o *ListPodsOptions) Run() error {
	nodes, err := o.Options.GetNodes()
	if err != nil {
		return err
	}

	// define a ToPrinter func that can handle human output
	o.Options.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.Options.PrintFlags.NamePrintFlags.Operation = operation
		return &listPodsPrinter{
			printFlags:     o.Options.PrintFlags,
			noHeaders:      o.Options.NoHeaders,
			humanPrintFunc: nodeOptsHumanPrintFunc,
			humanPrinting:  o.Options.PrintFlags.OutputFormat == nil || len(*o.Options.PrintFlags.OutputFormat) == 0,
			namePrinting:   o.Options.PrintFlags.OutputFormat != nil && *o.Options.PrintFlags.OutputFormat == "name",
		}, nil
	}

	unifiedPodList := &corev1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "List",
			APIVersion: "v1",
		},
	}
	successMessage := bytes.NewBuffer([]byte{})

	humanOutput := o.Options.PrintFlags.OutputFormat == nil || len(*o.Options.PrintFlags.OutputFormat) == 0
	humanPrinter, err := o.Options.ToPrinter("")
	if err != nil {
		return err
	}

	firstNode := true

	errList := []error{}
	for _, node := range nodes {
		podList, err := o.podsForNode(node)
		if err != nil {
			errList = append(errList, err)
			continue
		}

		nodeMessage := fmt.Sprintf("Listing matched pods on node %q:", node.ObjectMeta.Name)

		if humanOutput {
			if !firstNode {
				fmt.Fprintln(o.Options.ErrOut)
			}
			firstNode = false

			fmt.Fprintf(o.Options.ErrOut, "%s\n", nodeMessage)
			if err := humanPrinter.PrintObj(podList, o.Options.Out); err != nil {
				errList = append(errList, err)
				continue
			}
			continue
		}

		// add new set of pods to our successful printer message
		fmt.Fprintf(successMessage, nodeMessage)
		for _, pod := range podList.Items {
			fmt.Fprintf(successMessage, " - %s\n", pod.Name)
		}

		// aggregate pods for this node, with pods from other nodes
		unifiedPodList.Items = append(unifiedPodList.Items, podList.Items...)
	}

	// No need to print aggregated list or success message if printing output as tabular message
	if humanOutput {
		return kerrors.NewAggregate(errList)
	}

	p, err := o.Options.ToPrinter(successMessage.String())
	if err != nil {
		errList = append(errList, err)
		return kerrors.NewAggregate(errList)
	}

	if err := p.PrintObj(unifiedPodList, o.Options.Out); err != nil {
		errList = append(errList, err)
	}

	return kerrors.NewAggregate(errList)
}

func (o *ListPodsOptions) podsForNode(node *corev1.Node) (*corev1.PodList, error) {
	labelSelector, err := labels.Parse(o.Options.PodSelector)
	if err != nil {
		return nil, err
	}
	fieldSelector := fields.Set{GetPodHostFieldLabel(node.TypeMeta.APIVersion): node.ObjectMeta.Name}.AsSelector()

	// Filter all pods that satisfies pod label selector and belongs to the given node
	pods, err := o.Options.KubeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{LabelSelector: labelSelector.String(), FieldSelector: fieldSelector.String()})
	if err != nil {
		return nil, err
	}

	return pods, err
}

type listPodsPrinter struct {
	namePrinting   bool
	humanPrinting  bool
	humanPrintFunc func(*genericclioptions.PrintFlags, runtime.Object, *bool, bool, io.Writer) error
	noHeaders      bool
	printFlags     *genericclioptions.PrintFlags
}

func (p *listPodsPrinter) PrintObj(obj runtime.Object, out io.Writer) error {
	if p.humanPrinting || p.namePrinting {
		return p.humanPrintFunc(p.printFlags, obj, &p.noHeaders, p.namePrinting, out)
	}

	printer, err := p.printFlags.ToPrinter()
	if err != nil {
		return err
	}

	return printer.PrintObj(obj, out)
}

func nodeOptsHumanPrintFunc(printFlags *genericclioptions.PrintFlags, obj runtime.Object, noHeaders *bool, namePrinting bool, out io.Writer) error {
	w := tabwriter.NewWriter(out, tabWriterMinWidth, tabWriterWidth, tabWriterPadding, tabWriterPadChar, tabWriterFlags)
	defer w.Flush()

	noHeadersVal := noHeaders != nil && *noHeaders
	if !noHeadersVal && !namePrinting {
		columns := []string{"NAMESPACE", "NAME", "AGE"}
		fmt.Fprintf(w, "%s\t\n", strings.Join(columns, "\t"))

		// printed only the first time if requested
		*noHeaders = true
	}

	items := []corev1.Pod{}

	switch t := obj.(type) {
	case *corev1.Pod:
		items = append(items, *t)
	case *corev1.PodList:
		items = append(items, t.Items...)
	default:
		return fmt.Errorf("unexpected object %T", obj)
	}

	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}

	for _, pod := range items {
		if namePrinting {
			if err := p.PrintObj(&pod, out); err != nil {
				return err
			}
			continue
		}

		_, err := fmt.Fprintf(w, "%s\t%s/%s\t%s\t\n", pod.Namespace, "pod", pod.Name, translateTimestampSince(pod.CreationTimestamp))
		if err != nil {
			return err
		}
	}

	return nil
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.ShortHumanDuration(time.Since(timestamp.Time))
}
