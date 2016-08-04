package admission

import (
	"fmt"
	"io"
	"sort"
	"strings"

	oscache "github.com/openshift/origin/pkg/client/cache"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/controller/shared"
	oscc "github.com/openshift/origin/pkg/security/scc"
	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	scc "k8s.io/kubernetes/pkg/securitycontextconstraints"
	"k8s.io/kubernetes/pkg/serviceaccount"

	allocator "github.com/openshift/origin/pkg/security"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/golang/glog"
)

func init() {
	kadmission.RegisterPlugin("SecurityContextConstraint",
		func(client clientset.Interface, config io.Reader) (kadmission.Interface, error) {
			return NewConstraint(client), nil
		})
}

type constraint struct {
	*kadmission.Handler
	client    clientset.Interface
	sccLister *oscache.IndexerToSecurityContextConstraintsLister
}

var _ kadmission.Interface = &constraint{}
var _ = oadmission.WantsInformers(&constraint{})

// NewConstraint creates a new SCC constraint admission plugin.
func NewConstraint(kclient clientset.Interface) *constraint {
	return &constraint{
		Handler: kadmission.NewHandler(kadmission.Create, kadmission.Update),
		client:  kclient,
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
func (c *constraint) Admit(a kadmission.Attributes) error {
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

	// get all constraints that are usable by the user
	glog.V(4).Infof("getting security context constraints for pod %s (generate: %s) in namespace %s with user info %v", pod.Name, pod.GenerateName, a.GetNamespace(), a.GetUserInfo())

	sccMatcher := oscc.NewDefaultSCCMatcher(c.sccLister)
	matchedConstraints, err := sccMatcher.FindApplicableSCCs(a.GetUserInfo())
	if err != nil {
		return kadmission.NewForbidden(a, err)
	}

	// get all constraints that are usable by the SA
	if len(pod.Spec.ServiceAccountName) > 0 {
		userInfo := serviceaccount.UserInfo(a.GetNamespace(), pod.Spec.ServiceAccountName, "")
		glog.V(4).Infof("getting security context constraints for pod %s (generate: %s) with service account info %v", pod.Name, pod.GenerateName, userInfo)
		saConstraints, err := sccMatcher.FindApplicableSCCs(userInfo)
		if err != nil {
			return kadmission.NewForbidden(a, err)
		}
		matchedConstraints = append(matchedConstraints, saConstraints...)
	}

	// remove duplicate constraints and sort
	matchedConstraints = oscc.DeduplicateSecurityContextConstraints(matchedConstraints)
	sort.Sort(oscc.ByPriority(matchedConstraints))

	providers, errs := oscc.CreateProvidersFromConstraints(a.GetNamespace(), matchedConstraints, c.client)
	logProviders(pod, providers, errs)

	if len(providers) == 0 {
		return kadmission.NewForbidden(a, fmt.Errorf("no providers available to validate pod request"))
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
	return kadmission.NewForbidden(a, fmt.Errorf("unable to validate against any security context constraint: %v", validationErrs))
}

// SetInformers implements WantsInformers interface for constraint.
func (c *constraint) SetInformers(informers shared.InformerFactory) {
	c.sccLister = informers.SecurityContextConstraints().Lister()
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
