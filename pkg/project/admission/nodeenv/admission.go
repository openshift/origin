package nodeenv

import (
	"fmt"
	"io"

	"encoding/json"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/labels"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/util/labelselector"
)

func init() {
	admission.RegisterPlugin("OriginPodNodeEnvironment", func(client client.Interface, config io.Reader) (admission.Interface, error) {
		return NewPodNodeEnvironment(client)
	})
}

// podNodeEnvironment is an implementation of admission.Interface.
type podNodeEnvironment struct {
	*admission.Handler
	client client.Interface
	cache  *cache.ProjectCache
}

var _ = oadmission.WantsProjectCache(&podNodeEnvironment{})
var _ = oadmission.Validator(&podNodeEnvironment{})

// Admit enforces that pod and its project node label selectors matches at least a node in the cluster.
func (p *podNodeEnvironment) Admit(a admission.Attributes) (err error) {
	resource := a.GetResource()
	if resource != kapi.Resource("pods") {
		return nil
	}
	if a.GetSubresource() != "" {
		// only run the checks below on pods proper and not subresources
		return nil
	}

	obj := a.GetObject()
	pod, ok := obj.(*kapi.Pod)
	if !ok {
		return nil
	}

	name := pod.Name

	if !p.cache.Running() {
		return err
	}

	// if this is a pod being created by a daemonset we do not want to set the node selector automatically
	isDaemonset, err := p.isDaemonsetPod(pod)
	if err != nil {
		return apierrors.NewForbidden(resource, name, fmt.Errorf("an unexpected error occured when checking for daemonsets: %v", err))
	}
	if isDaemonset {
		return nil
	}

	namespace, err := p.cache.GetNamespace(a.GetNamespace())
	if err != nil {
		return apierrors.NewForbidden(resource, name, err)
	}
	projectNodeSelector, err := p.cache.GetNodeSelectorMap(namespace)
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

func (p *podNodeEnvironment) SetProjectCache(c *cache.ProjectCache) {
	p.cache = c
}

func (p *podNodeEnvironment) Validate() error {
	if p.cache == nil {
		return fmt.Errorf("project node environment plugin needs a project cache")
	}
	return nil
}

// isDaemonsetPod determines if the pod is being created by a daemonset or not.
func (p *podNodeEnvironment) isDaemonsetPod(pod *kapi.Pod) (bool, error) {
	annotation, ok := pod.Annotations[controller.CreatedByAnnotation]
	if !ok {
		return false, nil
	}

	var r kapi.SerializedReference
	err := json.Unmarshal([]byte(annotation), &r)
	if err != nil {
		return false, err
	}

	// if it says it's a daemonset double check, just in case
	if r.Reference.Kind == "DaemonSet" {
		ds, err := p.client.Extensions().DaemonSets(r.Reference.Namespace).Get(r.Reference.Name)
		if err != nil {
			return false, err
		}

		// we got a daemonset back, make sure this pod falls under its selector
		selector, err := extensions.LabelSelectorAsSelector(ds.Spec.Selector)
		if err != nil {
			return false, fmt.Errorf("unable to create selector for %s/%s: %v", r.Reference.Namespace, r.Reference.Name, err)
		}

		podLabels := labels.Set(pod.Labels)
		if selector.Matches(podLabels) {
			return true, nil
		} else {
			return false, fmt.Errorf("found daemonset %s/%s but pod does not match the selector", r.Reference.Namespace, r.Reference.Name)
		}
	}
	return false, nil
}

func NewPodNodeEnvironment(client client.Interface) (admission.Interface, error) {
	return &podNodeEnvironment{
		Handler: admission.NewHandler(admission.Create),
		client:  client,
	}, nil
}
