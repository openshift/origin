package podnodeconstraints

import (
	"fmt"
	"io"
	"reflect"

	"github.com/golang/glog"

	admission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/api/meta"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api"
)

// kindsToIgnore is a list of kinds that contain a PodSpec that
// we choose not to handle in this plugin
var kindsToIgnore = []unversioned.GroupKind{
	extensions.Kind("DaemonSet"),
}

func init() {
	admission.RegisterPlugin("PodNodeConstraints", func(c clientset.Interface, config io.Reader) (admission.Interface, error) {
		pluginConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		if pluginConfig == nil {
			glog.Infof("Admission plugin %q is not configured so it will be disabled.", "PodNodeConstraints")
			return nil, nil
		}
		return NewPodNodeConstraints(pluginConfig), nil
	})
}

// NewPodNodeConstraints creates a new admission plugin to prevent objects that contain pod templates
// from containing node bindings by name or selector based on role permissions.
func NewPodNodeConstraints(config *api.PodNodeConstraintsConfig) admission.Interface {
	plugin := podNodeConstraints{
		config:  config,
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
	if config != nil {
		plugin.selectorLabelBlacklist = sets.NewString(config.NodeSelectorLabelBlacklist...)
	}

	return &plugin
}

type podNodeConstraints struct {
	*admission.Handler
	selectorLabelBlacklist sets.String
	config                 *api.PodNodeConstraintsConfig
	authorizer             authorizer.Authorizer
}

func shouldCheckResource(resource unversioned.GroupResource, kind unversioned.GroupKind) (bool, error) {
	expectedKind, shouldCheck := meta.HasPodSpec(resource)
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

var _ = oadmission.Validator(&podNodeConstraints{})
var _ = oadmission.WantsAuthorizer(&podNodeConstraints{})

func readConfig(reader io.Reader) (*api.PodNodeConstraintsConfig, error) {
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
	config, ok := obj.(*api.PodNodeConstraintsConfig)
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
	spec, _, err := meta.GetPodSpec(attr.GetObject())
	if err != nil {
		return kapi.PodSpec{}, kapierrors.NewInternalError(err)
	}
	return *spec, nil
}

// validate PodSpec if NodeName or NodeSelector are specified
func (o *podNodeConstraints) admitPodSpec(attr admission.Attributes, ps kapi.PodSpec) error {
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

func (o *podNodeConstraints) Validate() error {
	if o.authorizer == nil {
		return fmt.Errorf("PodNodeConstraints needs an Openshift Authorizer")
	}
	return nil
}

// build LocalSubjectAccessReview struct to validate role via checkAccess
func (o *podNodeConstraints) checkPodsBindAccess(attr admission.Attributes) (bool, error) {
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), attr.GetNamespace()), attr.GetUserInfo())
	authzAttr := authorizer.DefaultAuthorizationAttributes{
		Verb:     "create",
		Resource: "pods/binding",
		APIGroup: kapi.GroupName,
	}
	if attr.GetResource().GroupResource() == kapi.Resource("pods") {
		authzAttr.ResourceName = attr.GetName()
	}
	allow, _, err := o.authorizer.Authorize(ctx, authzAttr)
	return allow, err
}
