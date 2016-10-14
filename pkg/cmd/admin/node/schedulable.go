package node

import (
	"k8s.io/kubernetes/pkg/kubectl"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
)

type SchedulableOptions struct {
	Options *NodeOptions

	Schedulable bool
}

func (s *SchedulableOptions) Run() error {
	nodes, err := s.Options.GetNodes()
	if err != nil {
		return err
	}

	errList := []error{}
	var printer kubectl.ResourcePrinter
	unschedulable := !s.Schedulable
	for _, node := range nodes {
		if node.Spec.Unschedulable != unschedulable {
			node.Spec.Unschedulable = unschedulable
			node, err = s.Options.KubeClient.Nodes().Update(node)
			if err != nil {
				errList = append(errList, err)
				continue
			}
		}

		if printer == nil {
			p, err := s.Options.GetPrintersByObject(node)
			if err != nil {
				return err
			}
			printer = p
		}

		printer.PrintObj(node, s.Options.Writer)
	}
	return kerrors.NewAggregate(errList)
}
