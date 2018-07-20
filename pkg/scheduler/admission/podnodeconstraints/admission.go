package podnodeconstraints

import (
	"fmt"
	"io"
	"reflect"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"

	oapps "github.com/openshift/api/apps"
	"github.com/openshift/api/security"
	"github.com/openshift/origin/pkg/api/imagereferencemutators"
	"github.com/openshift/origin/pkg/api/legacy"
	configlatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints"
)

const PluginName = "PodNodeConstraints"

// kindsToIgnore is a list of kinds that contain a PodSpec that
// we choose not to handle in this plugin
var kindsToIgnore = []schema.GroupKind{
	extensions.Kind("DaemonSet"),
}

func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName,
		func(config io.Reader) (admission.Interface, error) {
			pluginConfig, err := readConfig(config)
			if err != nil {
				return nil, err
			}
			if pluginConfig == nil {
				glog.Infof("Admission plugin %q is not configured so it will be disabled.", PluginName)
				return nil, nil
			}
			return NewPodNodeConstraints(pluginConfig, nodeidentifier.NewDefaultNodeIdentifier()), nil
		})
}

// NewPodNodeConstraints creates a new admission plugin to prevent objects that contain pod templates
// from containing node bindings by name or selector based on role permissions.
func NewPodNodeConstraints(config *podnodeconstraints.PodNodeConstraintsConfig, nodeIdentifier nodeidentifier.NodeIdentifier) admission.Interface {
	plugin := podNodeConstraints{
		config:         config,
		Handler:        admission.NewHandler(admission.Create, admission.Update),
		nodeIdentifier: nodeIdentifier,
	}
	if config != nil {
		plugin.selectorLabelBlacklist = sets.NewString(config.NodeSelectorLabelBlacklist...)
	}

	return &plugin
}

type podNodeConstraints struct {
	*admission.Handler
	selectorLabelBlacklist sets.String
	config                 *podnodeconstraints.PodNodeConstraintsConfig
	authorizer             authorizer.Authorizer
	nodeIdentifier         nodeidentifier.NodeIdentifier
}

func shouldCheckResource(resource schema.GroupResource, kind schema.GroupKind) (bool, error) {
	expectedKind, shouldCheck := resourcesToCheck[resource]
	if !shouldCheck {
		return false, nil
	}
	for _, ignore := range kindsToIgnore {
		if ignore == expectedKind {
			return false, nil
		}
	}
	if expectedKind != kind {
		return false, fmt.Errorf("Unexpected resource kind %v for resource %v", &kind, &resource)
	}
	return true, nil
}

// resourcesToCheck is a map of resources and corresponding kinds of things that we want handled in this plugin
var resourcesToCheck = map[schema.GroupResource]schema.GroupKind{
	kapi.Resource("pods"):                   kapi.Kind("Pod"),
	kapi.Resource("podtemplates"):           kapi.Kind("PodTemplate"),
	kapi.Resource("replicationcontrollers"): kapi.Kind("ReplicationController"),
	batch.Resource("jobs"):                  batch.Kind("Job"),
	batch.Resource("jobtemplates"):          batch.Kind("JobTemplate"),

	batch.Resource("cronjobs"):         batch.Kind("CronJob"),
	extensions.Resource("deployments"): extensions.Kind("Deployment"),
	extensions.Resource("replicasets"): extensions.Kind("ReplicaSet"),
	apps.Resource("statefulsets"):      apps.Kind("StatefulSet"),

	legacy.Resource("deploymentconfigs"):                   legacy.Kind("DeploymentConfig"),
	legacy.Resource("podsecuritypolicysubjectreviews"):     legacy.Kind("PodSecurityPolicySubjectReview"),
	legacy.Resource("podsecuritypolicyselfsubjectreviews"): legacy.Kind("PodSecurityPolicySelfSubjectReview"),
	legacy.Resource("podsecuritypolicyreviews"):            legacy.Kind("PodSecurityPolicyReview"),

	oapps.Resource("deploymentconfigs"):                      oapps.Kind("DeploymentConfig"),
	security.Resource("podsecuritypolicysubjectreviews"):     security.Kind("PodSecurityPolicySubjectReview"),
	security.Resource("podsecuritypolicyselfsubjectreviews"): security.Kind("PodSecurityPolicySelfSubjectReview"),
	security.Resource("podsecuritypolicyreviews"):            security.Kind("PodSecurityPolicyReview"),
}

var _ = initializer.WantsAuthorizer(&podNodeConstraints{})

func readConfig(reader io.Reader) (*podnodeconstraints.PodNodeConstraintsConfig, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, nil
	}
	obj, err := configlatest.ReadYAML(reader)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*podnodeconstraints.PodNodeConstraintsConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object: %#v", obj)
	}
	// No validation needed since config is just list of strings
	return config, nil
}

func (o *podNodeConstraints) Admit(attr admission.Attributes) error {
	switch {
	case o.config == nil,
		attr.GetSubresource() != "":
		return nil
	}
	shouldCheck, err := shouldCheckResource(attr.GetResource().GroupResource(), attr.GetKind().GroupKind())
	if err != nil {
		return err
	}
	if !shouldCheck {
		return nil
	}
	// Only check Create operation on pods
	if attr.GetResource().GroupResource() == kapi.Resource("pods") && attr.GetOperation() != admission.Create {
		return nil
	}
	ps, err := o.getPodSpec(attr)
	if err == nil {
		return o.admitPodSpec(attr, ps)
	}
	return err
}

// extract the PodSpec from the pod templates for each object we care about
func (o *podNodeConstraints) getPodSpec(attr admission.Attributes) (kapi.PodSpec, error) {
	spec, _, err := imagereferencemutators.GetPodSpec(attr.GetObject())
	if err != nil {
		return kapi.PodSpec{}, kapierrors.NewInternalError(err)
	}
	return *spec, nil
}

// validate PodSpec if NodeName or NodeSelector are specified
func (o *podNodeConstraints) admitPodSpec(attr admission.Attributes, ps kapi.PodSpec) error {
	// a node creating a mirror pod that targets itself is allowed
	// see the NodeRestriction plugin for further details
	if o.isNodeSelfTargetWithMirrorPod(attr, ps.NodeName) {
		return nil
	}

	matchingLabels := []string{}
	// nodeSelector blacklist filter
	for nodeSelectorLabel := range ps.NodeSelector {
		if o.selectorLabelBlacklist.Has(nodeSelectorLabel) {
			matchingLabels = append(matchingLabels, nodeSelectorLabel)
		}
	}
	// nodeName constraint
	if len(ps.NodeName) > 0 || len(matchingLabels) > 0 {
		allow, err := o.checkPodsBindAccess(attr)
		if err != nil {
			return err
		}
		if !allow {
			switch {
			case len(ps.NodeName) > 0 && len(matchingLabels) == 0:
				return admission.NewForbidden(attr, fmt.Errorf("node selection by nodeName is prohibited by policy for your role"))
			case len(ps.NodeName) == 0 && len(matchingLabels) > 0:
				return admission.NewForbidden(attr, fmt.Errorf("node selection by label(s) %v is prohibited by policy for your role", matchingLabels))
			case len(ps.NodeName) > 0 && len(matchingLabels) > 0:
				return admission.NewForbidden(attr, fmt.Errorf("node selection by nodeName and label(s) %v is prohibited by policy for your role", matchingLabels))
			}
		}
	}
	return nil
}

func (o *podNodeConstraints) SetAuthorizer(a authorizer.Authorizer) {
	o.authorizer = a
}

func (o *podNodeConstraints) ValidateInitialization() error {
	if o.authorizer == nil {
		return fmt.Errorf("%s requires an authorizer", PluginName)
	}
	if o.nodeIdentifier == nil {
		return fmt.Errorf("%s requires a node identifier", PluginName)
	}
	return nil
}

// build LocalSubjectAccessReview struct to validate role via checkAccess
func (o *podNodeConstraints) checkPodsBindAccess(attr admission.Attributes) (bool, error) {
	authzAttr := authorizer.AttributesRecord{
		User:            attr.GetUserInfo(),
		Verb:            "create",
		Namespace:       attr.GetNamespace(),
		Resource:        "pods",
		Subresource:     "binding",
		APIGroup:        kapi.GroupName,
		ResourceRequest: true,
	}
	if attr.GetResource().GroupResource() == kapi.Resource("pods") {
		authzAttr.Name = attr.GetName()
	}
	authorized, _, err := o.authorizer.Authorize(authzAttr)
	return authorized == authorizer.DecisionAllow, err
}

func (o *podNodeConstraints) isNodeSelfTargetWithMirrorPod(attr admission.Attributes, nodeName string) bool {
	// make sure we are actually trying to target a node
	if len(nodeName) == 0 {
		return false
	}
	// this check specifically requires the object to be pod (unlike the other checks where we want any pod spec)
	pod, ok := attr.GetObject().(*kapi.Pod)
	if !ok {
		return false
	}
	// note that anyone can create a mirror pod, but they are not privileged in any way
	// they are actually highly constrained since they cannot reference secrets
	// nodes can only create and delete them, and they will delete any "orphaned" mirror pods
	if _, isMirrorPod := pod.Annotations[kapi.MirrorPodAnnotationKey]; !isMirrorPod {
		return false
	}
	// we are targeting a node with a mirror pod
	// confirm the user is a node that is targeting itself
	actualNodeName, isNode := o.nodeIdentifier.NodeIdentity(attr.GetUserInfo())
	return isNode && actualNodeName == nodeName
}
