package node

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

const (
	flagGracePeriod = "grace-period"
	flagDryRun      = "dry-run"
	flagForce       = "force"
)

type EvacuateOptions struct {
	Options *NodeOptions

	// Optional params
	DryRun      bool
	Force       bool
	GracePeriod int64

	printPodHeaders bool
}

// NewEvacuateOptions creates a new EvacuateOptions with default values.
func NewEvacuateOptions(nodeOptions *NodeOptions) *EvacuateOptions {
	return &EvacuateOptions{
		Options:     nodeOptions,
		DryRun:      false,
		Force:       false,
		GracePeriod: 30,

		printPodHeaders: true,
	}
}

func (e *EvacuateOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVar(&e.DryRun, flagDryRun, e.DryRun, "Show pods that will be migrated. Optional param for --evacuate")
	flags.BoolVar(&e.Force, flagForce, e.Force, "Delete pods not backed by replication controller. Optional param for --evacuate")
	flags.Int64Var(&e.GracePeriod, flagGracePeriod, e.GracePeriod, "Grace period (seconds) for pods being deleted. Ignored if negative. Optional param for --evacuate")

}

func (e *EvacuateOptions) Run() error {
	if e.DryRun {
		listpodsOp := ListPodsOptions{Options: e.Options, printPodHeaders: e.printPodHeaders}
		return listpodsOp.Run()
	}

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
	// We do *not* automatically mark the node unschedulable to perform evacuation.
	// Rationale: If we unschedule the node and later the operation is unsuccessful (stopped by user, network error, etc.),
	// we may not be able to recover in some cases to mark the node back to schedulable. To avoid these cases, we recommend
	// user to explicitly set the node to schedulable/unschedulable.
	if !node.Spec.Unschedulable {
		return fmt.Errorf("Node '%s' must be unschedulable to perform evacuation.\nYou can mark the node unschedulable with 'oc adm manage-node %s --schedulable=false'", node.ObjectMeta.Name, node.ObjectMeta.Name)
	}

	labelSelector, err := labels.Parse(e.Options.PodSelector)
	if err != nil {
		return err
	}
	fieldSelector := fields.Set{GetPodHostFieldLabel(node.TypeMeta.APIVersion): node.ObjectMeta.Name}.AsSelector()

	// Filter all pods that satisfies pod label selector and belongs to the given node
	pods, err := e.Options.KubeClient.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{LabelSelector: labelSelector.String(), FieldSelector: fieldSelector.String()})
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		fmt.Fprint(e.Options.ErrWriter, "\nNo pods found on node: ", node.ObjectMeta.Name, "\n\n")
		return nil
	}
	rcs, err := e.Options.KubeClient.Core().ReplicationControllers(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	rss, err := e.Options.KubeClient.Extensions().ReplicaSets(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	dss, err := e.Options.KubeClient.Extensions().DaemonSets(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	jobs, err := e.Options.KubeClient.Batch().Jobs(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	printer, err := e.Options.GetPrintersByResource(schema.GroupVersionResource{Resource: "pod"}, false)
	if err != nil {
		return err
	}

	errList := []error{}
	firstPod := true
	numUnmanagedPods := 0

	var deleteOptions *metav1.DeleteOptions
	if e.GracePeriod >= 0 {
		deleteOptions = e.makeDeleteOptions()
	}

	for _, pod := range pods.Items {
		isManaged := false
		for _, rc := range rcs.Items {
			selector := labels.SelectorFromSet(rc.Spec.Selector)
			if selector.Matches(labels.Set(pod.Labels)) {
				isManaged = true
				break
			}
		}

		for _, rs := range rss.Items {
			selector := labels.SelectorFromSet(rs.Spec.Selector.MatchLabels)
			if selector.Matches(labels.Set(pod.Labels)) {
				isManaged = true
				break
			}
		}

		for _, ds := range dss.Items {
			selector := labels.SelectorFromSet(ds.Spec.Selector.MatchLabels)
			if selector.Matches(labels.Set(pod.Labels)) {
				isManaged = true
				break
			}
		}

		for _, job := range jobs.Items {
			selector := labels.SelectorFromSet(job.Spec.Selector.MatchLabels)
			if selector.Matches(labels.Set(pod.Labels)) {
				isManaged = true
				break
			}
		}

		if firstPod {
			fmt.Fprint(e.Options.ErrWriter, "\nMigrating these pods on node: ", node.ObjectMeta.Name, "\n\n")
			firstPod = false
		}

		printer.PrintObj(&pod, e.Options.Writer)

		if isManaged || e.Force {
			if err := e.Options.KubeClient.Core().Pods(pod.Namespace).Delete(pod.Name, deleteOptions); err != nil {
				glog.Errorf("Unable to delete a pod: %+v, error: %v", pod, err)
				errList = append(errList, err)
				continue
			}
		} else { // Pods without replication controller and no --force option
			numUnmanagedPods++
		}
	}
	if numUnmanagedPods > 0 {
		err := fmt.Errorf(`Unable to evacuate some pods because they are not managed by replication controller or replica set or deaemon set.
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

// makeDeleteOptions creates the delete options that will be used for pod evacuation.
func (e *EvacuateOptions) makeDeleteOptions() *metav1.DeleteOptions {
	return &metav1.DeleteOptions{GracePeriodSeconds: &e.GracePeriod}
}
