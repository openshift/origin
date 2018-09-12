package node

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
)

const (
	tabWriterMinWidth = 0
	tabWriterWidth    = 7
	tabWriterPadding  = 3
	tabWriterPadChar  = ' '
	tabWriterFlags    = 0
)

type NodeOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	DefaultNamespace string
	KubeClient       kubernetes.Interface

	ToPrinter func(string) (printers.ResourcePrinter, error)

	Builder *resource.Builder

	NoHeaders bool
	NodeNames []string

	// Common optional params
	Selector    string
	PodSelector string

	CheckNodeSelector bool

	genericclioptions.IOStreams
}

func NewNodeOptions(streams genericclioptions.IOStreams) *NodeOptions {
	return &NodeOptions{
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

func (o *NodeOptions) Complete(f kcmdutil.Factory, c *cobra.Command, args []string) error {
	defaultNamespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.KubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	o.Builder = f.NewBuilder()
	o.DefaultNamespace = defaultNamespace
	o.NodeNames = []string{}

	o.CheckNodeSelector = c.Flag("selector").Changed

	o.NoHeaders = kcmdutil.GetFlagBool(c, "no-headers")
	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}

	if len(args) > 0 {
		o.NodeNames = append(o.NodeNames, args...)
	}
	return nil
}

func (o *NodeOptions) Validate() error {
	errList := []error{}
	if o.CheckNodeSelector {
		if len(o.Selector) > 0 {
			if _, err := labels.Parse(o.Selector); err != nil {
				errList = append(errList, errors.New("--selector=<node_selector> must be a valid label selector"))
			}
		}
		if len(o.NodeNames) != 0 {
			errList = append(errList, errors.New("either specify --selector=<node_selector> or nodes but not both"))
		}
	} else if len(o.NodeNames) == 0 {
		errList = append(errList, errors.New("must provide --selector=<node_selector> or nodes"))
	}

	if len(o.PodSelector) > 0 {
		if _, err := labels.Parse(o.PodSelector); err != nil {
			errList = append(errList, errors.New("--pod-selector=<pod_selector> must be a valid label selector"))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (o *NodeOptions) GetNodes() ([]*corev1.Node, error) {
	nameArgs := []string{"nodes"}
	if len(o.NodeNames) != 0 {
		nameArgs = append(nameArgs, o.NodeNames...)
	}

	r := o.Builder.
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ContinueOnError().
		NamespaceParam(o.DefaultNamespace).
		LabelSelectorParam(o.Selector).
		ResourceTypeOrNameArgs(true, nameArgs...).
		Flatten().
		Do()
	if r.Err() != nil {
		return nil, r.Err()
	}

	errList := []error{}
	nodeList := []*corev1.Node{}
	_ = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		node, ok := info.Object.(*corev1.Node)
		if !ok {
			err = fmt.Errorf("cannot convert input to Node: %v", reflect.TypeOf(info.Object))
			errList = append(errList, err)
			// Don't bail out if one node fails
			return nil
		}
		nodeList = append(nodeList, node)
		return nil
	})
	if len(errList) != 0 {
		return nodeList, kerrors.NewAggregate(errList)
	}

	if len(nodeList) == 0 {
		return nodeList, fmt.Errorf("No nodes found")
	} else {
		givenNodeNames := sets.NewString(o.NodeNames...)
		foundNodeNames := sets.String{}
		for _, node := range nodeList {
			foundNodeNames.Insert(node.ObjectMeta.Name)
		}
		skippedNodeNames := givenNodeNames.Difference(foundNodeNames)
		if skippedNodeNames.Len() > 0 {
			return nodeList, fmt.Errorf("Nodes %v not found", strings.Join(skippedNodeNames.List(), ", "))
		}
	}
	return nodeList, nil
}

func GetPodHostFieldLabel(apiVersion string) string {
	switch apiVersion {
	default:
		return "spec.host"
	}
}
