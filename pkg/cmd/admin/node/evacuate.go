package node

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
)

type EvacuateOptions struct {
	Options *NodeOptions

	// Optional params
	DryRun bool
	Force  bool
}

func (e *EvacuateOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVar(&e.DryRun, "dry-run", false, "Show pods that will be migrated. Optional param for --evacuate")
	flags.BoolVar(&e.Force, "force", false, "Delete pods not backed by replication controller. Optional param for --evacuate")
}

func (e *EvacuateOptions) Run() error {
	nodes, err := e.Options.GetNodes()
	if err != nil {
		return err
	}

	errList := []error{}
	for _, node := range nodes {
		err := e.RunEvacuate(node)
		if err != nil {
			// Don't bail out if one node fails
			errList = append(errList, err)
		}
	}
	return kerrors.NewAggregate(errList)
}

func (e *EvacuateOptions) RunEvacuate(node *kapi.Node) error {
	if e.DryRun {
		listpodsOp := ListPodsOptions{Options: e.Options}
		return listpodsOp.Run()
	}

	// We do *not* automatically mark the node unschedulable to perform evacuation.
	// Rationale: If we unschedule the node and later the operation is unsuccessful (stopped by user, network error, etc.),
	// we may not be able to recover in some cases to mark the node back to schedulable. To avoid these cases, we recommend
	// user to explicitly set the node to schedulable/unschedulable.
	if !node.Spec.Unschedulable {
		return fmt.Errorf("Node '%s' must be unschedulable to perform evacuation.\nYou can mark the node unschedulable with 'openshift admin manage-node %s --schedulable=false'", node.ObjectMeta.Name, node.ObjectMeta.Name)
	}

	labelSelector, err := labels.Parse(e.Options.PodSelector)
	if err != nil {
		return err
	}
	fieldSelector := fields.Set{GetPodHostFieldLabel(node.TypeMeta.APIVersion): node.ObjectMeta.Name}.AsSelector()

	// Filter all pods that satisfies pod label selector and belongs to the given node
	pods, err := e.Options.Kclient.Pods(kapi.NamespaceAll).List(labelSelector, fieldSelector)
	if err != nil {
		return err
	}
	rcs, err := e.Options.Kclient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
	if err != nil {
		return err
	}

	printerWithHeaders, printerNoHeaders, err := e.Options.GetPrintersByResource("pod")
	if err != nil {
		return err
	}

	errList := []error{}
	firstPod := true
	numPodsWithNoRC := 0
	// grace = 0 implies delete the pod immediately
	grace := int64(0)
	deleteOptions := &kapi.DeleteOptions{GracePeriodSeconds: &grace}

	for _, pod := range pods.Items {
		foundrc := false
		for _, rc := range rcs.Items {
			selector := labels.SelectorFromSet(rc.Spec.Selector)
			if selector.Matches(labels.Set(pod.Labels)) {
				foundrc = true
				break
			}
		}

		if firstPod {
			fmt.Fprintln(e.Options.Writer, "\nMigrating these pods on node: ", node.ObjectMeta.Name, "\n")
			firstPod = false
			printerWithHeaders.PrintObj(&pod, e.Options.Writer)
		} else {
			printerNoHeaders.PrintObj(&pod, e.Options.Writer)
		}

		if foundrc || e.Force {
			if err := e.Options.Kclient.Pods(pod.Namespace).Delete(pod.Name, deleteOptions); err != nil {
				glog.Errorf("Unable to delete a pod: %+v, error: %v", pod, err)
				errList = append(errList, err)
				continue
			}
		} else { // Pods without replication controller and no --force option
			numPodsWithNoRC++
		}
	}
	if numPodsWithNoRC > 0 {
		err := fmt.Errorf(`Unable to evacuate some pods because they are not backed by replication controller.
Suggested options:
- You can list bare pods in json/yaml format using '--list-pods -o json|yaml'
- Force deletion of bare pods with --force option to --evacuate
- Optionally recreate these bare pods by massaging the json/yaml output from above list pods
`)
		errList = append(errList, err)
	}

	if len(errList) != 0 {
		return kerrors.NewAggregate(errList)
	}
	return nil
}
