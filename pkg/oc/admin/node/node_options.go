package node

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/categories"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

type NodeOptions struct {
	DefaultNamespace   string
	ExternalKubeClient kubernetes.Interface
	KubeClient         kclientset.Interface
	Writer             io.Writer
	ErrWriter          io.Writer

	Mapper            meta.RESTMapper
	Typer             runtime.ObjectTyper
	CategoryExpander  categories.CategoryExpander
	RESTClientFactory func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	Printer           func(mapping *meta.RESTMapping, withNamespace bool) (kprinters.ResourcePrinter, error)

	CmdPrinter       kprinters.ResourcePrinter
	CmdPrinterOutput bool

	Builder *resource.Builder

	NodeNames []string

	// Common optional params
	Selector    string
	PodSelector string
}

func (n *NodeOptions) Complete(f *clientcmd.Factory, c *cobra.Command, args []string, out, errout io.Writer) error {
	defaultNamespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	kc, err := f.ClientSet()
	if err != nil {
		return err
	}

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}
	externalkc, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	cmdPrinter, err := f.PrinterForOptions(kcmdutil.ExtractCmdPrintOptions(c, false))
	if err != nil {
		return err
	}
	mapper, typer := f.Object()

	n.Builder = f.NewBuilder()
	n.DefaultNamespace = defaultNamespace
	n.ExternalKubeClient = externalkc
	n.KubeClient = kc
	n.Writer = out
	n.ErrWriter = errout
	n.Mapper = mapper
	n.Typer = typer
	n.CategoryExpander = f.CategoryExpander()
	n.RESTClientFactory = f.ClientForMapping
	n.NodeNames = []string{}
	n.CmdPrinter = cmdPrinter
	n.CmdPrinterOutput = false

	n.Printer = func(mapping *meta.RESTMapping, withNamespace bool) (kprinters.ResourcePrinter, error) {
		return f.PrinterForMapping(kcmdutil.ExtractCmdPrintOptions(c, withNamespace), mapping)
	}

	if cmdPrinter.IsGeneric() {
		n.CmdPrinterOutput = true
	}
	if len(args) != 0 {
		n.NodeNames = append(n.NodeNames, args...)
	}
	return nil
}

func (n *NodeOptions) Validate(checkNodeSelector bool) error {
	errList := []error{}
	if checkNodeSelector {
		if len(n.Selector) > 0 {
			if _, err := labels.Parse(n.Selector); err != nil {
				errList = append(errList, errors.New("--selector=<node_selector> must be a valid label selector"))
			}
		}
		if len(n.NodeNames) != 0 {
			errList = append(errList, errors.New("either specify --selector=<node_selector> or nodes but not both"))
		}
	} else if len(n.NodeNames) == 0 {
		errList = append(errList, errors.New("must provide --selector=<node_selector> or nodes"))
	}

	if len(n.PodSelector) > 0 {
		if _, err := labels.Parse(n.PodSelector); err != nil {
			errList = append(errList, errors.New("--pod-selector=<pod_selector> must be a valid label selector"))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (n *NodeOptions) GetNodes() ([]*kapi.Node, error) {
	nameArgs := []string{"nodes"}
	if len(n.NodeNames) != 0 {
		nameArgs = append(nameArgs, n.NodeNames...)
	}

	r := n.Builder.
		Internal().
		ContinueOnError().
		NamespaceParam(n.DefaultNamespace).
		LabelSelectorParam(n.Selector).
		ResourceTypeOrNameArgs(true, nameArgs...).
		Flatten().
		Do()
	if r.Err() != nil {
		return nil, r.Err()
	}

	errList := []error{}
	nodeList := []*kapi.Node{}
	_ = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		node, ok := info.Object.(*kapi.Node)
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
		givenNodeNames := sets.NewString(n.NodeNames...)
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

func (n *NodeOptions) GetPrintersByObject(obj runtime.Object) (kprinters.ResourcePrinter, error) {
	gvk, _, err := legacyscheme.Scheme.ObjectKinds(obj)
	if err != nil {
		return nil, err
	}
	return n.GetPrinters(gvk[0], false)
}

func (n *NodeOptions) GetPrintersByResource(resource schema.GroupVersionResource, withNamespace bool) (kprinters.ResourcePrinter, error) {
	gvks, err := n.Mapper.KindsFor(resource)
	if err != nil {
		return nil, err
	}
	return n.GetPrinters(gvks[0], withNamespace)
}

func (n *NodeOptions) GetPrinters(gvk schema.GroupVersionKind, withNamespace bool) (kprinters.ResourcePrinter, error) {
	mapping, err := n.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	return n.Printer(mapping, withNamespace)
}

func GetPodHostFieldLabel(apiVersion string) string {
	switch apiVersion {
	default:
		return "spec.host"
	}
}
