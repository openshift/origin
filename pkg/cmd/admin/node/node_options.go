package node

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type NodeOptions struct {
	DefaultNamespace string
	Kclient          *client.Client
	Writer           io.Writer

	Mapper            meta.RESTMapper
	Typer             runtime.ObjectTyper
	RESTClientFactory func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	Printer           func(mapping *meta.RESTMapping, noHeaders bool) (kubectl.ResourcePrinter, error)

	CmdPrinter       kubectl.ResourcePrinter
	CmdPrinterOutput bool

	NodeNames []string

	// Common optional params
	Selector    string
	PodSelector string
}

func (n *NodeOptions) Complete(f *clientcmd.Factory, c *cobra.Command, args []string, out io.Writer) error {
	defaultNamespace, err := f.DefaultNamespace()
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
	mapper, typer := f.Object()

	n.DefaultNamespace = defaultNamespace
	n.Kclient = kc
	n.Writer = out
	n.Mapper = mapper
	n.Typer = typer
	n.RESTClientFactory = f.Factory.RESTClient
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

	r := resource.NewBuilder(n.Mapper, n.Typer, resource.ClientMapperFunc(n.RESTClientFactory)).
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
	_ = r.Visit(func(info *resource.Info) error {
		node, ok := info.Object.(*kapi.Node)
		if !ok {
			err := fmt.Errorf("cannot convert input to Node: ", reflect.TypeOf(info.Object))
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
		givenNodeNames := util.NewStringSet(n.NodeNames...)
		foundNodeNames := util.StringSet{}
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

func (n *NodeOptions) GetPrintersByObject(obj runtime.Object) (kubectl.ResourcePrinter, kubectl.ResourcePrinter, error) {
	version, kind, err := kapi.Scheme.ObjectVersionAndKind(obj)
	if err != nil {
		return nil, nil, err
	}
	return n.GetPrinters(kind, version)
}

func (n *NodeOptions) GetPrintersByResource(resource string) (kubectl.ResourcePrinter, kubectl.ResourcePrinter, error) {
	version, kind, err := n.Mapper.VersionAndKindForResource(resource)
	if err != nil {
		return nil, nil, err
	}
	return n.GetPrinters(kind, version)
}

func (n *NodeOptions) GetPrinters(kind, version string) (kubectl.ResourcePrinter, kubectl.ResourcePrinter, error) {
	mapping, err := n.Mapper.RESTMapping(kind, version)
	if err != nil {
		return nil, nil, err
	}

	printerWithHeaders, err := n.Printer(mapping, false)
	if err != nil {
		return nil, nil, err
	}
	printerNoHeaders, err := n.Printer(mapping, true)
	if err != nil {
		return nil, nil, err
	}
	return printerWithHeaders, printerNoHeaders, nil
}

func GetPodHostFieldLabel(apiVersion string) string {
	switch apiVersion {
	case "v1beta1", "v1beta2":
		return "DesiredState.Host"
	default:
		return "spec.host"
	}
}
