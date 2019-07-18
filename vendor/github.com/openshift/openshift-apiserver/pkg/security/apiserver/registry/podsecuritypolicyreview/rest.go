package podsecuritypolicyreview

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
	coreapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/serviceaccount"

	securityapi "github.com/openshift/openshift-apiserver/pkg/security/apis/security"
	securityvalidation "github.com/openshift/openshift-apiserver/pkg/security/apis/security/validation"
	"github.com/openshift/openshift-apiserver/pkg/security/apiserver/registry/podsecuritypolicysubjectreview"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/security/securitycontextconstraints/sccmatching"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	sccMatcher sccmatching.SCCMatcher
	saCache    corev1listers.ServiceAccountLister
	client     kubernetes.Interface
}

var _ rest.Creater = &REST{}
var _ rest.Scoper = &REST{}

// NewREST creates a new REST for policies..
func NewREST(m sccmatching.SCCMatcher, saCache corev1listers.ServiceAccountLister, c kubernetes.Interface) *REST {
	return &REST{sccMatcher: m, saCache: saCache, client: c}
}

// New creates a new PodSecurityPolicyReview object
func (r *REST) New() runtime.Object {
	return &securityapi.PodSecurityPolicyReview{}
}

func (s *REST) NamespaceScoped() bool {
	return true
}

// Create registers a given new PodSecurityPolicyReview instance to r.registry.
func (r *REST) Create(ctx context.Context, obj runtime.Object, _ rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	pspr, ok := obj.(*securityapi.PodSecurityPolicyReview)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("not a PodSecurityPolicyReview: %#v", obj))
	}
	if errs := securityvalidation.ValidatePodSecurityPolicyReview(pspr); len(errs) > 0 {
		return nil, apierrors.NewInvalid(coreapi.Kind("PodSecurityPolicyReview"), "", errs)
	}
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("namespace parameter required.")
	}
	serviceAccounts, err := getServiceAccounts(pspr.Spec, r.saCache, ns)
	if err != nil {
		return nil, apierrors.NewBadRequest(err.Error())
	}

	if len(serviceAccounts) == 0 {
		klog.Errorf("No service accounts for namespace %s", ns)
		return nil, apierrors.NewBadRequest(fmt.Sprintf("unable to find ServiceAccount for namespace: %s", ns))
	}

	errs := []error{}
	newStatus := securityapi.PodSecurityPolicyReviewStatus{}
	for _, sa := range serviceAccounts {
		userInfo := serviceaccount.UserInfo(ns, sa.Name, "")
		saConstraints, err := r.sccMatcher.FindApplicableSCCs(ns, userInfo)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to find SecurityContextConstraints for ServiceAccount %s: %v", sa.Name, err))
			continue
		}
		var namespace *corev1.Namespace
		for _, constraint := range saConstraints {
			var (
				provider sccmatching.SecurityContextConstraintsProvider
				err      error
			)
			pspsrs := securityapi.PodSecurityPolicySubjectReviewStatus{}
			if provider, namespace, err = sccmatching.CreateProviderFromConstraint(ns, namespace, constraint, r.client); err != nil {
				errs = append(errs, fmt.Errorf("unable to create provider for service account %s: %v", sa.Name, err))
				continue
			}
			_, err = podsecuritypolicysubjectreview.FillPodSecurityPolicySubjectReviewStatus(&pspsrs, provider, pspr.Spec.Template.Spec, constraint)
			if err != nil {
				klog.Errorf("unable to fill PodSecurityPolicyReviewStatus from constraint %v", err)
				continue
			}
			sapsprs := securityapi.ServiceAccountPodSecurityPolicyReviewStatus{PodSecurityPolicySubjectReviewStatus: pspsrs, Name: sa.Name}
			newStatus.AllowedServiceAccounts = append(newStatus.AllowedServiceAccounts, sapsprs)
		}
	}
	if len(errs) > 0 {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("%s", kerrors.NewAggregate(errs)))
	}
	pspr.Status = newStatus
	return pspr, nil
}

func getServiceAccounts(psprSpec securityapi.PodSecurityPolicyReviewSpec, saLister corev1listers.ServiceAccountLister, namespace string) ([]*corev1.ServiceAccount, error) {
	serviceAccounts := []*corev1.ServiceAccount{}
	//  TODO: express 'all service accounts'
	//if serviceAccountList, err := client.Core().ServiceAccounts(namespace).List(metainternal.ListOptions{}); err == nil {
	//	serviceAccounts = serviceAccountList.Items
	//	return serviceAccounts, fmt.Errorf("unable to retrieve service accounts: %v", err)
	//}

	if len(psprSpec.ServiceAccountNames) > 0 {
		errs := []error{}
		for _, saName := range psprSpec.ServiceAccountNames {
			sa, err := saLister.ServiceAccounts(namespace).Get(saName)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to retrieve ServiceAccount %s: %v", saName, err))
				continue
			}
			serviceAccounts = append(serviceAccounts, sa)
		}
		return serviceAccounts, kerrors.NewAggregate(errs)
	}
	saName := "default"
	if len(psprSpec.Template.Spec.ServiceAccountName) > 0 {
		saName = psprSpec.Template.Spec.ServiceAccountName
	}
	sa, err := saLister.ServiceAccounts(namespace).Get(saName)
	if err != nil {
		return serviceAccounts, fmt.Errorf("unable to retrieve ServiceAccount %s: %v", saName, err)
	}
	serviceAccounts = append(serviceAccounts, sa)
	return serviceAccounts, nil
}
