package node

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type NodeOptions struct {
	DefaultNamespace string
	KubeClient       *client.Client
	Writer           io.Writer
	ErrWriter        io.Writer

	Mapper            meta.RESTMapper
	Typer             runtime.ObjectTyper
	RESTClientFactory func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	Printer           func(mapping *meta.RESTMapping, printOptions kubectl.PrintOptions) (kubectl.ResourcePrinter, error)

	CmdPrinter       kubectl.ResourcePrinter
	CmdPrinterOutput bool

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
	_, kc, err := f.Clients()
	if err != nil {
		return err
	}
	cmdPrinter, output, err := kcmdutil.PrinterForCommand(c)
	if err != nil {
		return err
	}
	mapper, typer := f.Object(false)

	n.DefaultNamespace = defaultNamespace
	n.KubeClient = kc
	n.Writer = out
	n.ErrWriter = errout
	n.Mapper = mapper
	n.Typer = typer
	n.RESTClientFactory = f.Factory.ClientForMapping
	n.Printer = f.Printer
	n.NodeNames = []string{}
	n.CmdPrinter = cmdPrinter
	n.CmdPrinterOutput = false

	if output {
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

	r := resource.NewBuilder(n.Mapper, n.Typer, resource.ClientMapperFunc(n.RESTClientFactory), kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(n.DefaultNamespace).
		SelectorParam(n.Selector).
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

func (n *NodeOptions) GetPrintersByObject(obj runtime.Object) (kubectl.ResourcePrinter, error) {
	gvk, _, err := kapi.Scheme.ObjectKinds(obj)
	if err != nil {
		return nil, err
	}
	return n.GetPrinters(gvk[0])
}

func (n *NodeOptions) GetPrintersByResource(resource unversioned.GroupVersionResource) (kubectl.ResourcePrinter, error) {
	gvks, err := n.Mapper.KindsFor(resource)
	if err != nil {
		return nil, err
	}
	return n.GetPrinters(gvks[0])
}

func (n *NodeOptions) GetPrinters(gvk unversioned.GroupVersionKind) (kubectl.ResourcePrinter, error) {
	mapping, err := n.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	return n.Printer(mapping, kubectl.PrintOptions{})
}

func GetPodHostFieldLabel(apiVersion string) string {
	switch apiVersion {
	default:
		return "spec.host"
	}
}
