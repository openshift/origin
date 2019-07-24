package podtolerationsrestriction

import (
	"fmt"
	"io"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/serviceaccount"
)

const (
	PluginName       = "scheduling.openshift.io/RestrictPodTolerations"
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
	if attr.GetResource().GroupResource() != coreapi.Resource("pods") && attr.GetKind().GroupKind() != coreapi.Kind("Pod") && len(attr.GetSubresource()) == 0 {
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
	if len(podSpec.ServiceAccountName) == 0 {
		return admission.NewForbidden(attr, fmt.Errorf("service account is not provided. The restrict pod tolerations plugin is completely lost"))
	}
	for _, toleration := range tolerations {
		allow := p.IsAuthorized(podSpec.ServiceAccountName, attr, toleration)
		if !allow {
			return admission.NewForbidden(attr, fmt.Errorf("requested user %v doesn't have permission to provide the given tolerations", podSpec.ServiceAccountName))
		}
	}
	return nil
}

func (p *podTolerations) IsAuthorized(podSAName string, attr admission.Attributes, toleration coreapi.Toleration) bool {
	saInfo := serviceaccount.UserInfo(attr.GetNamespace(), podSAName, "")
	// check for hard-coded serviceaccount groups
	SAInNamespace := serviceaccount.UserInfo(attr.GetNamespace(), "system:serviceaccounts:"+attr.GetNamespace(), "")
	SAAcrossAllNamespace := serviceaccount.UserInfo("", "system:serviceaccounts", "")
	return p.checkPodTolerationAccess(saInfo, attr, toleration) || p.checkPodTolerationAccess(SAInNamespace, attr, toleration) || p.checkPodTolerationAccess(SAAcrossAllNamespace, attr, toleration)
}

// buildTolerationsAuthorizationAttributes builds the tolerations attributes from the user
func buildTolerationsAuthorizationAttributes(info user.Info, namespace string, toleration coreapi.Toleration) authorizer.Attributes {
	verb := toleration.Operator
	tolerationKey := toleration.Key
	if len(toleration.Key) == 0 {
		tolerationKey = masterToleration
	}
	authzAttr := authorizer.AttributesRecord{
		User:            info,
		Verb:            string(verb),
		Namespace:       namespace,
		Resource:        tolerationKey,
		APIGroup:        "toleration.scheduling.openshift.io",
		ResourceRequest: true,
		Name:            string(toleration.Effect) + ":" + toleration.Value,
	}
	return authzAttr
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
resourceName: NoSchedule:<empty_string>(effect:value)
verbs: ["Exists"] (Operator)

A cluster role to access toleration resources should have rbac rules like this

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: tolerations-creator
rules:
- apiGroups: ["toleration.scheduling.openshift.io"]
  resources: ["node-role.kubernetes.io/master", "node-role.kubernetes.io/worker",...]
  verbs: ["Exists", "Equal"] This can also be verbs:["*"]

If we want to be much more specific, for example we want to have something like master taint be allowed only, the
cluster role would like this

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: tolerations-creator
rules:
- apiGroups: "toleration.scheduling.openshift.io"
  resources: ["node-role.kubernetes.io/master"]
  resourceNames: NoSchedule:<value_of_toleration>
  verbs: ["Exists", "Equal"]
*/
func (p *podTolerations) checkPodTolerationAccess(userInfo user.Info, attr admission.Attributes, toleration coreapi.Toleration) bool {
	authzAttr := buildTolerationsAuthorizationAttributes(userInfo, attr.GetNamespace(), toleration)
	authorized, reason, err := p.authorizer.Authorize(authzAttr)
	if err != nil {
		klog.V(4).Infof("cannot authorize for the toleration: %v %v", reason, err)
	}
	return authorized == authorizer.DecisionAllow
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
