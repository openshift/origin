package admission

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/golang/glog"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	rbacregistry "k8s.io/kubernetes/pkg/registry/rbac"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	allocator "github.com/openshift/origin/pkg/security"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	securitylisters "github.com/openshift/origin/pkg/security/generated/listers/security/internalversion"
	scc "github.com/openshift/origin/pkg/security/securitycontextconstraints"
	admission "k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/pkg/serviceaccount"
)

const PluginName = "SecurityContextConstraint"

func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName,
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
	// if we can't convert then fail closed since we've already checked that this is supposed to be a pod object.
	// this shouldn't normally happen during admission but could happen if an integrator passes a versioned
	// pod object rather than an internal object.
	if !ok {
		return admission.NewForbidden(a, fmt.Errorf("object was marked as kind pod but was unable to be converted: %v", a.GetObject()))
	}

	// if this is an update, see if we are only updating the ownerRef.  Garbage collection does this
	// and we should allow it in general, since you had the power to update and the power to delete.
	// The worst that happens is that you delete something, but you aren't controlling the privileged object itself
	if a.GetOldObject() != nil && rbacregistry.IsOnlyMutatingGCFields(a.GetObject(), a.GetOldObject(), kapihelper.Semantic) {
		return nil
	}

	// get all constraints that are usable by the user
	glog.V(4).Infof("getting security context constraints for pod %s (generate: %s) in namespace %s with user info %v", pod.Name, pod.GenerateName, a.GetNamespace(), a.GetUserInfo())

	sccMatcher := scc.NewDefaultSCCMatcher(c.sccLister)
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
	matchedConstraints = scc.DeduplicateSecurityContextConstraints(matchedConstraints)
	sort.Sort(scc.ByPriority(matchedConstraints))

	providers, errs := scc.CreateProvidersFromConstraints(a.GetNamespace(), matchedConstraints, c.client)
	logProviders(pod, providers, errs)

	if len(providers) == 0 {
		return admission.NewForbidden(a, fmt.Errorf("no providers available to validate pod request"))
	}

	// TODO(liggitt): allow spec mutation during initializing updates?
	specMutationAllowed := a.GetOperation() == admission.Create

	// all containers in a single pod must validate under a single provider or we will reject the request
	validationErrs := field.ErrorList{}
	var (
		allowedPod       *kapi.Pod
		allowingProvider scc.SecurityContextConstraintsProvider
	)

loop:
	for _, provider := range providers {
		podCopy := pod.DeepCopy()

		if errs := scc.AssignSecurityContext(provider, podCopy, field.NewPath(fmt.Sprintf("provider %s: ", provider.GetSCCName()))); len(errs) > 0 {
			validationErrs = append(validationErrs, errs...)
			continue
		}

		// the entire pod validated
		switch {
		case specMutationAllowed:
			// if mutation is allowed, use the first found SCC that allows the pod.
			// This behavior is different from Kubernetes which tries to search a non-mutating provider
			// even on creating. We prefer most restrictive SCC in this case even if it mutates a pod.
			allowedPod = podCopy
			allowingProvider = provider
			glog.V(6).Infof("pod %s (generate: %s) validated against provider %s with mutation", pod.Name, pod.GenerateName, provider.GetSCCName())
			break loop
		case apiequality.Semantic.DeepEqual(pod, podCopy):
			// if we don't allow mutation, only use the validated pod if it didn't require any spec changes
			allowedPod = podCopy
			allowingProvider = provider
			glog.V(6).Infof("pod %s (generate: %s) validated against provider %s without mutation", pod.Name, pod.GenerateName, provider.GetSCCName())
			break loop
		default:
			glog.V(6).Infof("pod %s (generate: %s) validated against provider %s, but required mutation, skipping", pod.Name, pod.GenerateName, provider.GetSCCName())
		}
	}

	if allowedPod != nil {
		*pod = *allowedPod
		// annotate and accept the pod
		glog.V(4).Infof("pod %s (generate: %s) validated against provider %s", pod.Name, pod.GenerateName, allowingProvider.GetSCCName())
		if pod.ObjectMeta.Annotations == nil {
			pod.ObjectMeta.Annotations = map[string]string{}
		}
		pod.ObjectMeta.Annotations[allocator.ValidatedSCCAnnotation] = allowingProvider.GetSCCName()
		return nil
	}

	// we didn't validate against any security context constraint provider, reject the pod and give the errors for each attempt
	glog.V(4).Infof("unable to validate pod %s (generate: %s) against any security context constraint: %v", pod.Name, pod.GenerateName, validationErrs)
	return admission.NewForbidden(a, fmt.Errorf("unable to validate against any security context constraint: %v", validationErrs))
}

// SetSecurityInformers implements WantsSecurityInformer interface for constraint.
func (c *constraint) SetSecurityInformers(informers securityinformer.SharedInformerFactory) {
	c.sccLister = informers.Security().InternalVersion().SecurityContextConstraints().Lister()
}

func (c *constraint) SetInternalKubeClientSet(client kclientset.Interface) {
	c.client = client
}

// ValidateInitialization implements InitializationValidator interface for constraint.
func (c *constraint) ValidateInitialization() error {
	if c.sccLister == nil {
		return fmt.Errorf("%s requires an sccLister", PluginName)
	}
	if c.client == nil {
		return fmt.Errorf("%s requires a client", PluginName)
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
