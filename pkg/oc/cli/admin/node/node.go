package node

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
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

	genericclioptions.IOStreams
}

func NewManageNodeOptions(streams genericclioptions.IOStreams) *ManageNodeOptions {
	nodeOpts := NewNodeOptions(streams)
	return &ManageNodeOptions{
		nodeOptions:     nodeOpts,
		evacuateOptions: NewEvacuateOptions(nodeOpts),
		listPodsOptions: &ListPodsOptions{
			Options:         nodeOpts,
			printPodHeaders: true,
		},
		schedulableOptions: &SchedulableOptions{
			Options: nodeOpts,
		},

		IOStreams: streams,
	}
}

// NewCommandManageNode implements the OpenShift cli manage-node command
func NewCommandManageNode(f kcmdutil.Factory, commandName, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewManageNodeOptions(streams)
	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Manage nodes - list pods, evacuate, or mark ready",
		Long:    manageNodeLong,
		Example: fmt.Sprintf(manageNodeExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(c, f, args))
			kcmdutil.CheckErr(o.Validate(c))
			kcmdutil.CheckErr(o.RunManageNode(c))
		},
	}

	// Supported operations
	cmd.Flags().BoolVar(&schedulable, "schedulable", false, "Control pod schedulability on the node.")
	cmd.Flags().BoolVar(&evacuate, "evacuate", false, "Migrate all/selected pods on the node.")
	cmd.Flags().MarkDeprecated("evacuate", "use 'oc adm drain NODE' instead")
	cmd.Flags().BoolVar(&listpods, "list-pods", false, "List all/selected pods on the node. Printer flags --output, etc. are only valid for this option.")

	// Common optional params
	cmd.Flags().StringVar(&o.nodeOptions.PodSelector, "pod-selector", o.nodeOptions.PodSelector, "Label selector to filter pods on the node. Optional param for --evacuate or --list-pods")
	cmd.Flags().StringVar(&o.nodeOptions.Selector, "selector", o.nodeOptions.Selector, "Label selector to filter nodes. Either pass one/more nodes as arguments or use this node selector")

	// Operation specific params
	o.evacuateOptions.AddFlags(cmd)
	o.listPodsOptions.AddFlags(cmd)

	return cmd
}

func (o *ManageNodeOptions) Complete(c *cobra.Command, f kcmdutil.Factory, args []string) error {
	return o.nodeOptions.Complete(f, c, args)
}

func (o *ManageNodeOptions) Validate(c *cobra.Command) error {
	if err := ValidOperation(c); err != nil {
		return kcmdutil.UsageErrorf(c, err.Error())
	}

	checkNodeSelector := c.Flag("selector").Changed
	if err := o.nodeOptions.Validate(checkNodeSelector); err != nil {
		return kcmdutil.UsageErrorf(c, err.Error())
	}

	// Cross op validations
	if o.evacuateOptions.DryRun && !evacuate {
		err := errors.New("--dry-run is only applicable for --evacuate")
		return kcmdutil.UsageErrorf(c, err.Error())
	}

	return nil
}

func (o *ManageNodeOptions) RunManageNode(c *cobra.Command) error {
	var err error
	if c.Flag("schedulable").Changed {
		o.schedulableOptions.Schedulable = schedulable
		err = o.schedulableOptions.Run()
	} else if evacuate {
		o.evacuateOptions.printPodHeaders = !kcmdutil.GetFlagBool(c, "no-headers")
		err = o.evacuateOptions.Run()
	} else if listpods {
		o.listPodsOptions.printPodHeaders = !kcmdutil.GetFlagBool(c, "no-headers")
		err = o.listPodsOptions.Run()
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
