package admission

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	client "k8s.io/kubernetes/pkg/client/unversioned"

	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/util/labelselector"
)

func init() {
	admission.RegisterPlugin("OriginPodNodeEnvironment", func(client client.Interface, config io.Reader) (admission.Interface, error) {
		return NewPodNodeEnvironment(client)
	})
}

// podNodeEnvironment is an implementation of admission.Interface.
type podNodeEnvironment struct {
	client client.Interface
}

// Admit enforces that pod and its project node label selectors matches at least a node in the cluster.
func (p *podNodeEnvironment) Admit(a admission.Attributes) (err error) {
	// ignore anything except create or update of pods
	if !(a.GetOperation() == admission.Create || a.GetOperation() == admission.Update) {
		return nil
	}
	resource := a.GetResource()
	if resource != "pods" {
		return nil
	}

	obj := a.GetObject()
	pod, ok := obj.(*kapi.Pod)
	if !ok {
		return nil
	}

	name := pod.Name

	projects, err := projectcache.GetProjectCache()
	if err != nil {
		return err
	}
	namespace, err := projects.GetNamespaceObject(a.GetNamespace())
	if err != nil {
		return apierrors.NewForbidden(resource, name, err)
	}
	projectNodeSelector, err := projects.GetNodeSelectorMap(namespace)
	if err != nil {
		return err
	}

	if labelselector.Conflicts(projectNodeSelector, pod.Spec.NodeSelector) {
		return apierrors.NewForbidden(resource, name, fmt.Errorf("pod node label selector conflicts with its project node label selector"))
	}

	// modify pod node selector = project node selector + current pod node selector
	pod.Spec.NodeSelector = labelselector.Merge(projectNodeSelector, pod.Spec.NodeSelector)

	return nil
}

func (p *podNodeEnvironment) Handles(operation admission.Operation) bool {
	return operation == admission.Create || operation == admission.Update
}

func NewPodNodeEnvironment(client client.Interface) (admission.Interface, error) {
	return &podNodeEnvironment{
		client: client,
	}, nil
}
