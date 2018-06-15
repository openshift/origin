package podsecuritypolicyselfsubjectreview

import (
	"fmt"
	"sort"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/golang/glog"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityvalidation "github.com/openshift/origin/pkg/security/apis/security/validation"
	podsecuritypolicysubjectreview "github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview"
	scc "github.com/openshift/origin/pkg/security/securitycontextconstraints"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	sccMatcher scc.SCCMatcher
	client     clientset.Interface
}

var _ rest.Creater = &REST{}

// NewREST creates a new REST for policies..
func NewREST(m scc.SCCMatcher, c clientset.Interface) *REST {
	return &REST{sccMatcher: m, client: c}
}

// New creates a new PodSecurityPolicySelfSubjectReview object
func (r *REST) New() runtime.Object {
	return &securityapi.PodSecurityPolicySelfSubjectReview{}
}

// Create registers a given new pspssr instance to r.registry.
func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	pspssr, ok := obj.(*securityapi.PodSecurityPolicySelfSubjectReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a PodSecurityPolicySelfSubjectReview: %#v", obj))
	}
	if errs := securityvalidation.ValidatePodSecurityPolicySelfSubjectReview(pspssr); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(kapi.Kind("PodSecurityPolicySelfSubjectReview"), "", errs)
	}
	userInfo, ok := apirequest.UserFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("no user data associated with context"))
	}
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("namespace parameter required.")
	}

	matchedConstraints, err := r.sccMatcher.FindApplicableSCCs(userInfo, ns)
	if err != nil {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("unable to find SecurityContextConstraints: %v", err))
	}
	saName := pspssr.Spec.Template.Spec.ServiceAccountName
	if len(saName) > 0 {
		saUserInfo := serviceaccount.UserInfo(ns, saName, "")
		saConstraints, err := r.sccMatcher.FindApplicableSCCs(saUserInfo, ns)
		if err != nil {
			return nil, kapierrors.NewBadRequest(fmt.Sprintf("unable to find SecurityContextConstraints: %v", err))
		}
		matchedConstraints = append(matchedConstraints, saConstraints...)
	}
	scc.DeduplicateSecurityContextConstraints(matchedConstraints)
	sort.Sort(scc.ByPriority(matchedConstraints))
	var namespace *kapi.Namespace
	for _, constraint := range matchedConstraints {
		var (
			provider scc.SecurityContextConstraintsProvider
			err      error
		)
		if provider, namespace, err = scc.CreateProviderFromConstraint(ns, namespace, constraint, r.client); err != nil {
			glog.Errorf("Unable to create provider for constraint: %v", err)
			continue
		}
		filled, err := podsecuritypolicysubjectreview.FillPodSecurityPolicySubjectReviewStatus(&pspssr.Status, provider, pspssr.Spec.Template.Spec, constraint)
		if err != nil {
			glog.Errorf("unable to fill PodSecurityPolicySelfSubjectReview from constraint %v", err)
			continue
		}
		if filled {
			return pspssr, nil
		}
	}
	return pspssr, nil
}
