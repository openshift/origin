package restrictnodes

import (
	"errors"
	"io"
	"strings"

	"github.com/golang/glog"

	"k8s.io/client-go/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller/informers"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/labels"

	"k8s.io/kubernetes/pkg/admission"
)

func init() {
	admission.RegisterPlugin("openshift.io/RestrictNodes",
		func(kclient kclientset.Interface, config io.Reader) (admission.Interface, error) {
			return NewRestrictNodesAdmission(kclient)
		})
}

// restrictNodesAdmission implements admission.Interface and enforces
// restrictions on nodes to limit their access to mutate other nodes,
// mutate pods bound to other nodes, and to access secrets.
type restrictNodesAdmission struct {
	*admission.Handler

	kclient kclientset.Interface
	pods    *cache.StoreToPodLister
}

var _ admission.WantsInformerFactory = &restrictNodesAdmission{}

// NewRestrictNodesAdmission configures an admission plugin that enforces
// restrictions on adding role bindings in a project.
func NewRestrictNodesAdmission(kclient kclientset.Interface) (admission.Interface, error) {
	return &restrictNodesAdmission{
		Handler: admission.NewHandler(admission.Get, admission.Create, admission.Update, admission.Delete),
		kclient: kclient,
	}, nil
}

var (
	secretsResource = kapi.Resource("secrets")
	nodesResource   = kapi.Resource("nodes")
	podsResource    = kapi.Resource("pods")
)

func (q *restrictNodesAdmission) Admit(a admission.Attributes) error {
	// We only care about nodes we can identify
	userInfo := a.GetUserInfo()
	if userInfo == nil {
		return nil
	}
	username := userInfo.GetName()
	if !strings.HasPrefix(username, "system:node:") {
		return nil
	}
	nodeName := strings.TrimPrefix(username, "system:node:")

	// We only care about secrets, pods, and nodes; ignore anything else.
	switch a.GetResource().GroupResource() {
	case secretsResource:
		return q.admitSecret(nodeName, a)
	case nodesResource:
		return q.admitNode(nodeName, a)
	case podsResource:
		return q.admitPod(nodeName, a)
	default:
		return nil
	}
}

var (
	everything = labels.Everything()
)

func (q *restrictNodesAdmission) admitSecret(nodeName string, a admission.Attributes) error {
	ns := a.GetNamespace()
	name := a.GetName()
	pods, err := q.pods.Pods(ns).List(everything)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		if pod.Spec.NodeName != nodeName {
			continue
		}
		found := false
		visitSecretNames(pod, func(referencedName string) bool {
			if name == referencedName {
				found = true
				return false
			}
			return true
		})
		if found {
			return nil
		}
	}

	// TODO: other secret references nodes need? PVs?

	glog.Errorf("NODE-FORBIDDEN: Node %s tried to access secret %s/%s", nodeName, a.GetName(), a.GetNamespace())
	return kapierrors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), errors.New("cannot access secret"))
}

func visitSecretNames(pod *kapi.Pod, visitor func(string) bool) {
	for _, s := range pod.Spec.ImagePullSecrets {
		if !visitor(s.Name) {
			return
		}
	}
	for _, v := range pod.Spec.Volumes {
		if v.Secret != nil && !visitor(v.Secret.SecretName) {
			return
		}
	}
	for _, v := range pod.Spec.InitContainers {
		for _, e := range v.Env {
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil && !visitor(e.ValueFrom.SecretKeyRef.Name) {
				return
			}
		}
	}
	for _, v := range pod.Spec.Containers {
		for _, e := range v.Env {
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil && !visitor(e.ValueFrom.SecretKeyRef.Name) {
				return
			}
		}
	}
}

func (q *restrictNodesAdmission) admitNode(nodeName string, a admission.Attributes) error {
	// Only let nodes modify their own API resource
	if a.GetName() == nodeName {
		return nil
	}
	glog.Errorf("NODE-FORBIDDEN: Node %s tried to access node %s", nodeName, a.GetName())
	return kapierrors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), errors.New("cannot access other nodes"))
}

func (q *restrictNodesAdmission) admitPod(nodeName string, a admission.Attributes) error {
	if len(a.GetSubresource()) > 0 && a.GetSubresource() != "status" {
		return nil
	}

	switch a.GetOperation() {
	case admission.Create:
		// only let nodes create mirror pods targeted to themselves
		object := a.GetObject()
		if object == nil {
			// TODO: fail closed?
			return nil
		}
		pod, ok := object.(*kapi.Pod)
		if !ok {
			// TODO: fail closed?
			return nil
		}
		if pod.Spec.NodeName != nodeName {
			glog.Errorf("NODE-FORBIDDEN: Node %s tried to create pod %s/%s with nodeName %s", nodeName, pod.Name, pod.Namespace, pod.Spec.NodeName)
			return kapierrors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), errors.New("can only create pods bound to self"))
		}
		if _, isMirrorPod := pod.Annotations[types.ConfigMirrorAnnotationKey]; !isMirrorPod {
			glog.Errorf("NODE-FORBIDDEN: Node %s tried to create non-mirror pod %s/%s", nodeName, pod.Name, pod.Namespace)
			return kapierrors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), errors.New("can only create mirror pods"))
		}
		secrets := sets.NewString()
		visitSecretNames(pod, func(name string) bool {
			secrets.Insert(name)
			return true
		})
		if len(secrets) > 0 {
			glog.Errorf("NODE-FORBIDDEN: Node %s tried to create pod %s/%s referencing secrets %q", nodeName, pod.Name, pod.Namespace, secrets.List())
			return kapierrors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), errors.New("can not reference secrets in mirror pods"))
		}

	case admission.Update:
		// only let nodes mutate pods targeted to themselves
		oldObject := a.GetOldObject()
		if oldObject == nil {
			// TODO: fail closed?
			return nil
		}
		oldPod, ok := oldObject.(*kapi.Pod)
		if !ok {
			// TODO: fail closed?
			return nil
		}
		if oldPod.Spec.NodeName != nodeName {
			glog.Errorf("NODE-FORBIDDEN: Node %s tried to update pod %s/%s with nodeName %s", nodeName, oldPod.Name, oldPod.Namespace, oldPod.Spec.NodeName)
			return kapierrors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), errors.New("cannot update pod bound to other node"))
		}

	case admission.Delete:
		// only let nodes delete pods targeted to themselves
		pod, err := q.pods.Pods(a.GetNamespace()).Get(a.GetName())
		if err != nil {
			// TODO: fail closed?
			return nil
		}
		if pod.Spec.NodeName != nodeName {
			glog.Errorf("NODE-FORBIDDEN: Node %s tried to delete pod %s/%s with nodeName %s", nodeName, pod.Name, pod.Namespace, pod.Spec.NodeName)
			return kapierrors.NewForbidden(a.GetResource().GroupResource(), a.GetName(), errors.New("cannot delete pod bound to other node"))
		}
	}
	return nil
}

func (q *restrictNodesAdmission) SetInformerFactory(factory informers.SharedInformerFactory) {
	q.pods = factory.Pods().Lister()
}

func (q *restrictNodesAdmission) Validate() error {
	if q.kclient == nil {
		return errors.New("RestrictNodesAdmission plugin requires a Kubernetes client")
	}
	if q.pods == nil {
		return errors.New("RestrictNodesAdmission plugin requires a pods informer")
	}

	return nil
}
