package podsecuritypolicyselfsubjectreview

import (
	"context"
	"fmt"

	"k8s.io/kubernetes/openshift-kube-apiserver/admission/security/securitycontextconstraints/sccmatching"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/serviceaccount"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityvalidation "github.com/openshift/origin/pkg/security/apis/security/validation"
	podsecuritypolicysubjectreview "github.com/openshift/origin/pkg/security/apiserver/registry/podsecuritypolicysubjectreview"
	"k8s.io/klog"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	sccMatcher sccmatching.SCCMatcher
	client     kubernetes.Interface
}

var _ rest.Creater = &REST{}
var _ rest.Scoper = &REST{}

// NewREST creates a new REST for policies..
func NewREST(m sccmatching.SCCMatcher, c kubernetes.Interface) *REST {
	return &REST{sccMatcher: m, client: c}
}

// New creates a new PodSecurityPolicySelfSubjectReview object
func (r *REST) New() runtime.Object {
	return &securityapi.PodSecurityPolicySelfSubjectReview{}
}

func (s *REST) NamespaceScoped() bool {
	return true
}

// Create registers a given new pspssr instance to r.registry.
func (r *REST) Create(ctx context.Context, obj runtime.Object, _ rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	pspssr, ok := obj.(*securityapi.PodSecurityPolicySelfSubjectReview)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("not a PodSecurityPolicySelfSubjectReview: %#v", obj))
	}
	if errs := securityvalidation.ValidatePodSecurityPolicySelfSubjectReview(pspssr); len(errs) > 0 {
		return nil, apierrors.NewInvalid(coreapi.Kind("PodSecurityPolicySelfSubjectReview"), "", errs)
	}
	userInfo, ok := apirequest.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("no user data associated with context"))
	}
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("namespace parameter required.")
	}

	users := []user.Info{userInfo}
	saName := pspssr.Spec.Template.Spec.ServiceAccountName
	if len(saName) > 0 {
		users = append(users, serviceaccount.UserInfo(ns, saName, ""))
	}

	matchedConstraints, err := r.sccMatcher.FindApplicableSCCs(ns, users...)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("unable to find SecurityContextConstraints: %v", err))
	}

	var namespace *corev1.Namespace
	for _, constraint := range matchedConstraints {
		var (
			provider sccmatching.SecurityContextConstraintsProvider
			err      error
		)
		if provider, namespace, err = sccmatching.CreateProviderFromConstraint(ns, namespace, constraint, r.client); err != nil {
			klog.Errorf("Unable to create provider for constraint: %v", err)
			continue
		}
		filled, err := podsecuritypolicysubjectreview.FillPodSecurityPolicySubjectReviewStatus(&pspssr.Status, provider, pspssr.Spec.Template.Spec, constraint)
		if err != nil {
			klog.Errorf("unable to fill PodSecurityPolicySelfSubjectReview from constraint %v", err)
			continue
		}
		if filled {
			return pspssr, nil
		}
	}
	return pspssr, nil
}
