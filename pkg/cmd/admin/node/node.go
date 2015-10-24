package node

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	ManageNodeCommandName = "manage-node"

	manageNodeLong = `
Manage nodes

This command provides common operations on nodes for administrators.

schedulable: Marking node schedulable will allow pods to be schedulable on the node and
	     marking node unschedulable will block pods to be scheduled on the node.

evacuate: Migrate all/selected pod on the provided nodes.

list-pods: List all/selected pods on given/selected nodes. It can list the output in json/yaml format.`

	manageNodeExample = `	# Block accepting any pods on given nodes
	$ %[1]s <mynode> --schedulable=false

	# Mark selected nodes as schedulable
	$ %[1]s --selector="<env=dev>" --schedulable=true

	# Migrate selected pods
	$ %[1]s <mynode> --evacuate --pod-selector="<service=myapp>"

	# Show pods that will be migrated
	$ %[1]s <mynode> --evacuate --dry-run --pod-selector="<service=myapp>"

	# List all pods on given nodes
	$ %[1]s <mynode1> <mynode2> --list-pods`
)

var schedulable, evacuate, listpods bool

// NewCommandManageNode implements the OpenShift cli manage-node command
func NewCommandManageNode(f *clientcmd.Factory, commandName, fullName string, out io.Writer) *cobra.Command {
	opts := &NodeOptions{}
	schedulableOp := &SchedulableOptions{Options: opts}
	evacuateOp := NewEvacuateOptions(opts)
	listpodsOp := &ListPodsOptions{Options: opts}

	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Manage nodes - list pods, evacuate, or mark ready",
		Long:    manageNodeLong,
		Example: fmt.Sprintf(manageNodeExample, fullName),
		Run: func(c *cobra.Command, args []string) {

			if err := ValidOperation(c); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			if err := opts.Complete(f, c, args, out); err != nil {
				kcmdutil.CheckErr(err)
			}

			checkNodeSelector := c.Flag("selector").Changed
			if err := opts.Validate(checkNodeSelector); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			// Cross op validations
			if evacuateOp.DryRun && !evacuate {
				err := errors.New("--dry-run is only applicable for --evacuate")
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			var err error
			if c.Flag("schedulable").Changed {
				schedulableOp.Schedulable = schedulable
				err = schedulableOp.Run()
			} else if evacuate {
				err = evacuateOp.Run()
			} else if listpods {
				err = listpodsOp.Run()
			}
			kcmdutil.CheckErr(err)
		},
	}
	flags := cmd.Flags()

	// Supported operations
	flags.BoolVar(&schedulable, "schedulable", false, "Control pod schedulability on the node.")
	flags.BoolVar(&evacuate, "evacuate", false, "Migrate all/selected pods on the node.")
	flags.BoolVar(&listpods, "list-pods", false, "List all/selected pods on the node. Printer flags --output, etc. are only valid for this option.")

	// Common optional params
	flags.StringVar(&opts.PodSelector, "pod-selector", "", "Label selector to filter pods on the node. Optional param for --evacuate or --list-pods")
	flags.StringVar(&opts.Selector, "selector", "", "Label selector to filter nodes. Either pass one/more nodes as arguments or use this node selector")

	// Operation specific params
	evacuateOp.AddFlags(cmd)
	listpodsOp.AddFlags(cmd)

	return cmd
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
