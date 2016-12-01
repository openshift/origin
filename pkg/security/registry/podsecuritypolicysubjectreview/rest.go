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
	matchedConstraints = oscc.DeduplicateSecurityContextConstraints(matchedConstraints)
	sort.Sort(oscc.ByPriority(matchedConstraints))
	if err = oscc.AssignConstraints(matchedConstraints, ns, r.client,
		func(provider kscc.SecurityContextConstraintsProvider) (*kapi.PodSecurityContext, map[string]string, []*kapi.SecurityContext, error) {
			pod := &kapi.Pod{
				Spec: pspsr.Spec.Template.Spec,
			}
			psc, annotations, cscs, errs := oscc.ResolvePodSecurityContext(provider, pod)
			if len(errs) > 0 {
				pspsr.Status.Reason = "CantAssignSecurityContextConstraintProvider"
				return nil, nil, nil, fmt.Errorf("unable to assign SecurityContextConstraints provider: %v", errs.ToAggregate())
			}
			return psc, annotations, cscs, nil
		},
		func(provider kscc.SecurityContextConstraintsProvider, constraint *kapi.SecurityContextConstraints, psc *kapi.PodSecurityContext, annotations map[string]string, cscs []*kapi.SecurityContext) error {
			pod := &kapi.Pod{
				Spec: pspsr.Spec.Template.Spec,
			}

			ref, err := kapi.GetReference(constraint)
			if err != nil {
				pspsr.Status.Reason = "CantObtainReference"
				return fmt.Errorf("unable to get SecurityContextConstraints reference: %v", err)
			}
			oscc.SetSecurityContext(pod, psc, annotations, cscs)
			pspsr.Status.AllowedBy = ref
			if len(pspsr.Spec.Template.Spec.ServiceAccountName) > 0 {
				pspsr.Status.Template.Spec = pod.Spec
			}
			return nil
		}); err != nil {
		glog.V(4).Infof("PodSecurityPolicySubjectReview error: %v", err)
	}
	return pspsr, nil
}
