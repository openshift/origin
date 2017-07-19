package admission

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/validation/field"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	allocator "github.com/openshift/origin/pkg/security"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securitylisters "github.com/openshift/origin/pkg/security/generated/listers/security/internalversion"
	oscc "github.com/openshift/origin/pkg/security/scc"
	scc "github.com/openshift/origin/pkg/security/securitycontextconstraints"
	admission "k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/pkg/serviceaccount"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("SecurityContextConstraint",
		func(config io.Reader) (admission.Interface, error) {
			return NewConstraint(), nil
		})
}

type constraint struct {
	*admission.Handler
	client    kclientset.Interface
	sccLister securitylisters.SecurityContextConstraintsLister
}

var _ admission.Interface = &constraint{}
var _ = oadmission.WantsSecurityInformer(&constraint{})
var _ = kadmission.WantsInternalKubeClientSet(&constraint{})

// NewConstraint creates a new SCC constraint admission plugin.
func NewConstraint() *constraint {
	return &constraint{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

// Admit determines if the pod should be admitted based on the requested security context
// and the available SCCs.
//
// 1.  Find SCCs for the user.
// 2.  Find SCCs for the SA.  If there is an error retrieving SA SCCs it is not fatal.
// 3.  Remove duplicates between the user/SA SCCs.
// 4.  Create the providers, includes setting pre-allocated values if necessary.
// 5.  Try to generate and validate an SCC with providers.  If we find one then admit the pod
//     with the validated SCC.  If we don't find any reject the pod and give all errors from the
//     failed attempts.
// On updates, the BeforeUpdate of the pod strategy only zeroes out the status.  That means that
// any change that claims the pod is no longer privileged will be removed.  That should hold until
// we get a true old/new set of objects in.
func (c *constraint) Admit(a admission.Attributes) error {
	if a.GetResource().GroupResource() != kapi.Resource("pods") {
		return nil
	}
	if len(a.GetSubresource()) != 0 {
		return nil
	}

	pod, ok := a.GetObject().(*kapi.Pod)
	// if we can't convert then we don't handle this object so just return
	if !ok {
		return nil
	}

	// if this is an update, see if we are only updating the ownerRef.  Garbage collection does this
	// and we should allow it in general, since you had the power to update and the power to delete.
	// The worst that happens is that you delete something, but you aren't controlling the privileged object itself
	if a.GetOldObject() != nil && oadmission.IsOnlyMutatingGCFields(a.GetObject(), a.GetOldObject()) {
		return nil
	}

	// get all constraints that are usable by the user
	glog.V(4).Infof("getting security context constraints for pod %s (generate: %s) in namespace %s with user info %v", pod.Name, pod.GenerateName, a.GetNamespace(), a.GetUserInfo())

	sccMatcher := oscc.NewDefaultSCCMatcher(c.sccLister)
	matchedConstraints, err := sccMatcher.FindApplicableSCCs(a.GetUserInfo())
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	// get all constraints that are usable by the SA
	if len(pod.Spec.ServiceAccountName) > 0 {
		userInfo := serviceaccount.UserInfo(a.GetNamespace(), pod.Spec.ServiceAccountName, "")
		glog.V(4).Infof("getting security context constraints for pod %s (generate: %s) with service account info %v", pod.Name, pod.GenerateName, userInfo)
		saConstraints, err := sccMatcher.FindApplicableSCCs(userInfo)
		if err != nil {
			return admission.NewForbidden(a, err)
		}
		matchedConstraints = append(matchedConstraints, saConstraints...)
	}

	// remove duplicate constraints and sort
	matchedConstraints = oscc.DeduplicateSecurityContextConstraints(matchedConstraints)
	sort.Sort(oscc.ByPriority(matchedConstraints))

	providers, errs := oscc.CreateProvidersFromConstraints(a.GetNamespace(), matchedConstraints, c.client)
	logProviders(pod, providers, errs)

	if len(providers) == 0 {
		return admission.NewForbidden(a, fmt.Errorf("no providers available to validate pod request"))
	}

	// all containers in a single pod must validate under a single provider or we will reject the request
	validationErrs := field.ErrorList{}
	for _, provider := range providers {
		if errs := oscc.AssignSecurityContext(provider, pod, field.NewPath(fmt.Sprintf("provider %s: ", provider.GetSCCName()))); len(errs) > 0 {
			validationErrs = append(validationErrs, errs...)
			continue
		}

		// the entire pod validated, annotate and accept the pod
		glog.V(4).Infof("pod %s (generate: %s) validated against provider %s", pod.Name, pod.GenerateName, provider.GetSCCName())
		if pod.ObjectMeta.Annotations == nil {
			pod.ObjectMeta.Annotations = map[string]string{}
		}
		pod.ObjectMeta.Annotations[allocator.ValidatedSCCAnnotation] = provider.GetSCCName()
		return nil
	}

	// we didn't validate against any security context constraint provider, reject the pod and give the errors for each attempt
	glog.V(4).Infof("unable to validate pod %s (generate: %s) against any security context constraint: %v", pod.Name, pod.GenerateName, validationErrs)
	return admission.NewForbidden(a, fmt.Errorf("unable to validate against any security context constraint: %v", validationErrs))
}

// SetInformers implements WantsInformers interface for constraint.

func (c *constraint) SetSecurityInformers(informers securityinformer.SharedInformerFactory) {
	c.sccLister = informers.Security().InternalVersion().SecurityContextConstraints().Lister()
}

func (c *constraint) SetInternalKubeClientSet(client kclientset.Interface) {
	c.client = client
}

// Validate defines actions to vallidate security admission
func (c *constraint) Validate() error {
	if c.sccLister == nil {
		return fmt.Errorf("sccLister not initialized")
	}
	return nil
}

// logProviders logs what providers were found for the pod as well as any errors that were encountered
// while creating providers.
func logProviders(pod *kapi.Pod, providers []scc.SecurityContextConstraintsProvider, providerCreationErrs []error) {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.GetSCCName()
	}
	glog.V(4).Infof("validating pod %s (generate: %s) against providers %s", pod.Name, pod.GenerateName, strings.Join(names, ","))

	for _, err := range providerCreationErrs {
		glog.V(4).Infof("provider creation error: %v", err)
	}
}
