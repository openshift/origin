package podtolerations

import (
	"fmt"
	"io"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
)

const (
	PluginName       = "PodTolerations"
	masterToleration = "node-role.kubernetes.io/master"
)

type podTolerations struct {
	*admission.Handler
	authorizer authorizer.Authorizer
}

var _ = initializer.WantsAuthorizer(&podTolerations{})
var _ = admission.ValidationInterface(&podTolerations{})

func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return NewPodTolerationsPlugin(), nil
	})
}

func NewPodTolerationsPlugin() admission.Interface {
	return &podTolerations{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

func (p *podTolerations) Validate(attr admission.Attributes, _ admission.ObjectInterfaces) error {
	if attr.GetResource().GroupResource() != coreapi.Resource("pods") && attr.GetKind().GroupKind() != coreapi.Kind("Pod") {
		return nil
	}
	pod, ok := attr.GetObject().(*coreapi.Pod)
	if !ok {
		return admission.NewForbidden(attr, fmt.Errorf("unexpected Type %T", attr.GetObject()))
	}
	return p.validatePodTolerations(attr, pod.Spec)
}

func (p *podTolerations) validatePodTolerations(attr admission.Attributes, podSpec coreapi.PodSpec) error {
	// if a pod has no tolerations, then no need to validate it's tolerations
	if len(podSpec.Tolerations) == 0 {
		return nil
	}
	tolerations := podSpec.Tolerations
	for _, toleration := range tolerations {
		allow, err := p.checkPodTolerationAccess(attr, toleration)
		if err != nil {
			return err
		}
		if !allow {
			return fmt.Errorf("requested user %v doesn't have permission to provide the given tolerations", attr.GetUserInfo().GetName())
		}
	}

	return nil
}

// checkPodTolerationAccess checks if pods are allowed to have a particular toleration. In this case, we're focusing
// on master tolerations. For example,
/*
tolerations:
- effect: NoSchedule
  key: node-role.kubernetes.io/master
  operator: Exists
are converted to

apiGroup: toleration.scheduling.openshift.io
Resource: node-role.kubernetes.io/master
resourceName: <empty_string>/NoSchedule
verbs: ["Exists"]
*/
func (p *podTolerations) checkPodTolerationAccess(attr admission.Attributes, toleration coreapi.Toleration) (bool, error) {
	verb := toleration.Operator
	tolerationKey := ""
	// TODO: Add some more cases for blanket toleration etc.
	if len(toleration.Key) == 0 {
		tolerationKey = masterToleration
	}
	authzAttr := authorizer.AttributesRecord{
		User:            attr.GetUserInfo(),
		Verb:            string(verb),
		Namespace:       attr.GetNamespace(),
		Resource:        tolerationKey,
		APIGroup:        "toleration.scheduling.openshift.io",
		ResourceRequest: true,
	}
	if attr.GetResource().GroupResource() == coreapi.Resource("pods") {
		authzAttr.Name = toleration.Value + "/" + string(toleration.Effect)
	}

	authorized, _, err := p.authorizer.Authorize(authzAttr)
	return authorized == authorizer.DecisionAllow, err
}

func (p *podTolerations) SetAuthorizer(a authorizer.Authorizer) {
	p.authorizer = a
}

func (p *podTolerations) ValidateInitialization() error {
	if p.authorizer == nil {
		return fmt.Errorf("%s requires an authorizer", PluginName)
	}
	return nil
}
