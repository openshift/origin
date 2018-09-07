package node

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

type SchedulableOptions struct {
	Options *NodeOptions

	Schedulable bool
}

func NewSchedulableOptions(nodeOpts *NodeOptions) *SchedulableOptions {
	return &SchedulableOptions{
		Options: nodeOpts,
	}
}

func (s *SchedulableOptions) Run() error {
	nodes, err := s.Options.GetNodes()
	if err != nil {
		return err
	}

	errList := []error{}
	unschedulable := !s.Schedulable
	for _, node := range nodes {
		if node.Spec.Unschedulable != unschedulable {
			patch := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, unschedulable)
			node, err = s.Options.KubeClient.CoreV1().Nodes().Patch(node.Name, types.MergePatchType, []byte(patch))
			if err != nil {
				errList = append(errList, err)
				continue
			}
		}

		message := "schedulable"
		if unschedulable {
			message = "unschedulable"
		}
		p, err := s.Options.ToPrinter(fmt.Sprintf("marked %s", message))
		if err != nil {
			errList = append(errList, err)
			continue
		}

		p.PrintObj(node, s.Options.Out)
	}
	return kerrors.NewAggregate(errList)
}
