package node

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
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
	ignoreHeaders := false
	for _, node := range nodes {
		err := s.RunSchedulable(node, &ignoreHeaders)
		if err != nil {
			// Don't bail out if one node fails
			errList = append(errList, err)
		}
	}
	if len(errList) != 0 {
		return kerrors.NewAggregate(errList)
	}
	return nil
}

func (s *SchedulableOptions) RunSchedulable(node *kapi.Node, ignoreHeaders *bool) error {
	var updatedNode *kapi.Node
	var err error

	if node.Spec.Unschedulable != !s.Schedulable {
		node.Spec.Unschedulable = !s.Schedulable
		updatedNode, err = s.Options.Kclient.Nodes().Update(node)
		if err != nil {
			return err
		}
	} else {
		updatedNode = node
	}

	printerWithHeaders, printerNoHeaders, err := s.Options.GetPrintersByObject(updatedNode)
	if err != nil {
		return err
	}
	if *ignoreHeaders {
		printerNoHeaders.PrintObj(updatedNode, s.Options.Writer)
	} else {
		printerWithHeaders.PrintObj(updatedNode, s.Options.Writer)
		*ignoreHeaders = true
	}
	return nil
}
