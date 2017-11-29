package node

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const ManageNodeCommandName = "manage-node"

var (
	manageNodeLong = templates.LongDesc(`
		Manage nodes

		This command provides common operations on nodes for administrators.

		schedulable: Marking node schedulable will allow pods to be schedulable on the node and
				 marking node unschedulable will block pods to be scheduled on the node.

		evacuate: Migrate all/selected pod on the provided nodes.

		list-pods: List all/selected pods on given/selected nodes. It can list the output in json/yaml format.`)

	manageNodeExample = templates.Examples(`
		# Block accepting any pods on given nodes
	  %[1]s <mynode> --schedulable=false

	  # Mark selected nodes as schedulable
	  %[1]s --selector="<env=dev>" --schedulable=true

	  # Migrate selected pods
	  %[1]s <mynode> --evacuate --pod-selector="<service=myapp>"

	  # Migrate selected pods, use a grace period of 60 seconds
	  %[1]s <mynode> --evacuate --grace-period=60 --pod-selector="<service=myapp>"

	  # Migrate selected pods not backed by replication controller
	  %[1]s <mynode> --evacuate --force --pod-selector="<service=myapp>"

	  # Show pods that will be migrated
	  %[1]s <mynode> --evacuate --dry-run --pod-selector="<service=myapp>"

	  # List all pods on given nodes
	  %[1]s <mynode1> <mynode2> --list-pods`)
)

var schedulable, evacuate, listpods bool

type ManageNodeOptions struct {
	nodeOptions        *NodeOptions
	evacuateOptions    *EvacuateOptions
	listPodsOptions    *ListPodsOptions
	schedulableOptions *SchedulableOptions

	out, errout io.Writer
}

// NewCommandManageNode implements the OpenShift cli manage-node command
func NewCommandManageNode(f *clientcmd.Factory, commandName, fullName string, out, errout io.Writer) *cobra.Command {
	nodeOpts := &NodeOptions{}
	schedulableOp := &SchedulableOptions{Options: nodeOpts}
	evacuateOp := NewEvacuateOptions(nodeOpts)
	listpodsOp := &ListPodsOptions{Options: nodeOpts, printPodHeaders: true}

	opts := &ManageNodeOptions{
		nodeOptions:        nodeOpts,
		evacuateOptions:    evacuateOp,
		listPodsOptions:    listpodsOp,
		schedulableOptions: schedulableOp,

		out:    out,
		errout: errout,
	}

	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Manage nodes - list pods, evacuate, or mark ready",
		Long:    manageNodeLong,
		Example: fmt.Sprintf(manageNodeExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(c, f, args))
			kcmdutil.CheckErr(opts.Validate(c))
			kcmdutil.CheckErr(opts.RunManageNode(c))
		},
	}
	flags := cmd.Flags()

	// Supported operations
	flags.BoolVar(&schedulable, "schedulable", false, "Control pod schedulability on the node.")
	flags.BoolVar(&evacuate, "evacuate", false, "Migrate all/selected pods on the node.")
	flags.MarkDeprecated("evacuate", "use 'oc adm drain NODE' instead")
	flags.BoolVar(&listpods, "list-pods", false, "List all/selected pods on the node. Printer flags --output, etc. are only valid for this option.")

	// Common optional params
	flags.StringVar(&nodeOpts.PodSelector, "pod-selector", "", "Label selector to filter pods on the node. Optional param for --evacuate or --list-pods")
	flags.StringVar(&nodeOpts.Selector, "selector", "", "Label selector to filter nodes. Either pass one/more nodes as arguments or use this node selector")

	// Operation specific params
	evacuateOp.AddFlags(cmd)
	listpodsOp.AddFlags(cmd)

	return cmd
}

func (n *ManageNodeOptions) Complete(c *cobra.Command, f *clientcmd.Factory, args []string) error {
	return n.nodeOptions.Complete(f, c, args, n.out, n.errout)
}

func (n *ManageNodeOptions) Validate(c *cobra.Command) error {
	if err := ValidOperation(c); err != nil {
		return kcmdutil.UsageErrorf(c, err.Error())
	}

	checkNodeSelector := c.Flag("selector").Changed
	if err := n.nodeOptions.Validate(checkNodeSelector); err != nil {
		return kcmdutil.UsageErrorf(c, err.Error())
	}

	// Cross op validations
	if n.evacuateOptions.DryRun && !evacuate {
		err := errors.New("--dry-run is only applicable for --evacuate")
		return kcmdutil.UsageErrorf(c, err.Error())
	}

	return nil
}

func (n *ManageNodeOptions) RunManageNode(c *cobra.Command) error {
	var err error
	if c.Flag("schedulable").Changed {
		n.schedulableOptions.Schedulable = schedulable
		err = n.schedulableOptions.Run()
	} else if evacuate {
		n.evacuateOptions.printPodHeaders = !kcmdutil.GetFlagBool(c, "no-headers")
		err = n.evacuateOptions.Run()
	} else if listpods {
		n.listPodsOptions.printPodHeaders = !kcmdutil.GetFlagBool(c, "no-headers")
		err = n.listPodsOptions.Run()
	}

	return err
}

func ValidOperation(c *cobra.Command) error {
	numOps := 0
	if c.Flag("schedulable").Changed {
		numOps++
	}
	if evacuate {
		numOps++
	}
	if listpods {
		numOps++
	}

	if numOps == 0 {
		return errors.New("must provide a node operation. Supported operations: --schedulable, --evacuate and --list-pods")
	} else if numOps != 1 {
		return errors.New("must provide only one node operation at a time")
	}
	return nil
}
