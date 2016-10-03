package podsecuritypolicysubjectreview

import (
	"fmt"
	"sort"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	kscc "k8s.io/kubernetes/pkg/securitycontextconstraints"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/util/validation/field"

	securityapi "github.com/openshift/origin/pkg/security/api"
	securityvalidation "github.com/openshift/origin/pkg/security/api/validation"
	oscc "github.com/openshift/origin/pkg/security/scc"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	sccMatcher oscc.SCCMatcher
	client     clientset.Interface
}

// NewREST creates a new REST for policies..
func NewREST(m oscc.SCCMatcher, c clientset.Interface) *REST {
	return &REST{sccMatcher: m, client: c}
}

// New creates a new PodSecurityPolicySubjectReview object
func (r *REST) New() runtime.Object {
	return &securityapi.PodSecurityPolicySubjectReview{}
}

// Create registers a given new PodSecurityPolicySubjectReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	pspsr, ok := obj.(*securityapi.PodSecurityPolicySubjectReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a PodSecurityPolicySubjectReview: %#v", obj))
	}

	ns, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("namespace parameter required.")
	}

	if errs := securityvalidation.ValidatePodSecurityPolicySubjectReview(pspsr); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(kapi.Kind("PodSecurityPolicySubjectReview"), "", errs)
	}

	userInfo := &user.DefaultInfo{Name: pspsr.Spec.User, Groups: pspsr.Spec.Groups}
	matchedConstraints, err := r.sccMatcher.FindApplicableSCCs(userInfo)
	if err != nil {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("unable to find SecurityContextConstraints: %v", err))
	}

	saName := pspsr.Spec.Template.Spec.ServiceAccountName
	if len(saName) > 0 {
		saUserInfo := serviceaccount.UserInfo(ns, saName, "")
		saConstraints, err := r.sccMatcher.FindApplicableSCCs(saUserInfo)
		if err != nil {
			return nil, kapierrors.NewBadRequest(fmt.Sprintf("unable to find SecurityContextConstraints: %v", err))
		}
		matchedConstraints = append(matchedConstraints, saConstraints...)
	}
	oscc.DeduplicateSecurityContextConstraints(matchedConstraints)
	sort.Sort(oscc.ByPriority(matchedConstraints))
	var namespace *kapi.Namespace
	for _, constraint := range matchedConstraints {
		var (
			provider kscc.SecurityContextConstraintsProvider
			err      error
		)
		if provider, namespace, err = oscc.CreateProviderFromConstraint(ns, namespace, constraint, r.client); err != nil {
			glog.Errorf("Unable to create provider for constraint: %v", err)
			continue
		}
		filled, err := FillPodSecurityPolicySubjectReviewStatus(&pspsr.Status, provider, pspsr.Spec.Template.Spec, constraint)
		if err != nil {
			glog.Errorf("unable to fill PodSecurityPolicySubjectReviewStatus from constraint %v", err)
			continue
		}
		if filled {
			return pspsr, nil
		}
	}
	return pspsr, nil
}

// FillPodSecurityPolicySubjectReviewStatus fills PodSecurityPolicySubjectReviewStatus assigning SecurityContectConstraint to the PodSpec
func FillPodSecurityPolicySubjectReviewStatus(s *securityapi.PodSecurityPolicySubjectReviewStatus, provider kscc.SecurityContextConstraintsProvider, spec kapi.PodSpec, constraint *kapi.SecurityContextConstraints) (bool, error) {
	pod := &kapi.Pod{
		Spec: spec,
	}
	if errs := oscc.AssignSecurityContext(provider, pod, field.NewPath(fmt.Sprintf("provider %s: ", provider.GetSCCName()))); len(errs) > 0 {
		glog.Errorf("unable to assign SecurityContextConstraints provider: %v", errs)
		s.Reason = "CantAssignSecurityContextConstraintProvider"
		return false, fmt.Errorf("unable to assign SecurityContextConstraints provider: %v", errs.ToAggregate())
	}
	ref, err := kapi.GetReference(constraint)
	if err != nil {
		s.Reason = "CantObtainReference"
		return false, fmt.Errorf("unable to get SecurityContextConstraints reference: %v", err)
	}
	s.AllowedBy = ref

	if len(spec.ServiceAccountName) > 0 {
		s.Template.Spec = pod.Spec
	}
	return true, nil
}
