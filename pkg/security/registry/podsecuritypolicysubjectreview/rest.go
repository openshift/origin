package podsecuritypolicysubjectreview

import (
	"fmt"

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
	assigner := newSCCAssigner(&pspsr.Status, pspsr.Spec.Template.Spec)
	if err = oscc.AssignConstraints(r.sccMatcher, matchedConstraints, ns, r.client, assigner); err != nil {
		glog.V(4).Infof("PodSecurityPolicySelfSubjectReview error: %v", err)
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

type sCCAssigner struct {
	status *securityapi.PodSecurityPolicySubjectReviewStatus
	spec   kapi.PodSpec
}

var _ oscc.SCCAssigner = &sCCAssigner{}

func newSCCAssigner(status *securityapi.PodSecurityPolicySubjectReviewStatus, spec kapi.PodSpec) oscc.SCCAssigner {
	return &sCCAssigner{
		status: status,
		spec:   spec,
	}
}

func (a *sCCAssigner) Assign(provider kscc.SecurityContextConstraintsProvider, constraint *kapi.SecurityContextConstraints) error {
	filled, err := FillPodSecurityPolicySubjectReviewStatus(a.status, provider, a.spec, constraint)
	if !filled || err != nil {
		if err == nil {
			err = fmt.Errorf("unknown reason")
		}
		return fmt.Errorf("unable to fill PodSecurityPolicySubjectReviewStatus from constraint: %v", err)
	}
	return nil
}
