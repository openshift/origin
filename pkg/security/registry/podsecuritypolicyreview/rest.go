package podsecuritypolicyreview

import (
	"fmt"
	"sort"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	kscc "k8s.io/kubernetes/pkg/securitycontextconstraints"
	"k8s.io/kubernetes/pkg/serviceaccount"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	oscache "github.com/openshift/origin/pkg/client/cache"
	securityapi "github.com/openshift/origin/pkg/security/api"
	securityvalidation "github.com/openshift/origin/pkg/security/api/validation"
	oscc "github.com/openshift/origin/pkg/security/scc"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	sccMatcher oscc.SCCMatcher
	saCache    oscache.StoreToServiceAccountLister
	client     clientset.Interface
}

// NewREST creates a new REST for policies..
func NewREST(m oscc.SCCMatcher, saCache oscache.StoreToServiceAccountLister, c clientset.Interface) *REST {
	return &REST{sccMatcher: m, saCache: saCache, client: c}
}

// New creates a new PodSecurityPolicyReview object
func (r *REST) New() runtime.Object {
	return &securityapi.PodSecurityPolicyReview{}
}

// Create registers a given new PodSecurityPolicyReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	pspr, ok := obj.(*securityapi.PodSecurityPolicyReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a PodSecurityPolicyReview: %#v", obj))
	}
	if errs := securityvalidation.ValidatePodSecurityPolicyReview(pspr); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(kapi.Kind("PodSecurityPolicyReview"), "", errs)
	}
	ns, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("namespace parameter required.")
	}
	serviceAccounts, err := getServiceAccounts(pspr.Spec, r.saCache, ns)
	if err != nil {
		return nil, kapierrors.NewBadRequest(err.Error())
	}

	if len(serviceAccounts) == 0 {
		glog.Errorf("No service accounts for namespace %s", ns)
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("unable to find ServiceAccount for namespace: %s", ns))
	}

	errs := []error{}
	newStatus := securityapi.PodSecurityPolicyReviewStatus{}
	for _, sa := range serviceAccounts {
		userInfo := serviceaccount.UserInfo(ns, sa.Name, "")
		saConstraints, err := r.sccMatcher.FindApplicableSCCs(userInfo)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to find SecurityContextConstraints for ServiceAccount %s: %v", sa.Name, err))
			continue
		}
		saConstraints = oscc.DeduplicateSecurityContextConstraints(saConstraints)
		sort.Sort(oscc.ByPriority(saConstraints))
		if err = oscc.AssignConstraints(saConstraints, ns, r.client,
			func(provider kscc.SecurityContextConstraintsProvider) (psc *kapi.PodSecurityContext, annotations map[string]string, cscs []*kapi.SecurityContext, err error) {
				pspsrs := securityapi.PodSecurityPolicySubjectReviewStatus{}
				pod := &kapi.Pod{
					Spec: pspr.Spec.Template.Spec,
				}
				psc, annotations, cscs, errs := oscc.ResolvePodSecurityContext(provider, pod)
				if len(errs) > 0 {
					pspsrs.Reason = "CantAssignSecurityContextConstraintProvider"
					return nil, nil, nil, fmt.Errorf("unable to resolve PodSecurityContext provider: %v", errs.ToAggregate())
				}
				return psc, annotations, cscs, nil
			},
			func(provider kscc.SecurityContextConstraintsProvider, constraint *kapi.SecurityContextConstraints, psc *kapi.PodSecurityContext, annotations map[string]string, cscs []*kapi.SecurityContext) error {
				pspsrs := securityapi.PodSecurityPolicySubjectReviewStatus{}
				pod := &kapi.Pod{
					Spec: pspr.Spec.Template.Spec,
				}
				ref, err := kapi.GetReference(constraint)
				if err != nil {
					pspsrs.Reason = "CantObtainReference"
					return fmt.Errorf("unable to get SecurityContextConstraints reference: %v", err)
				}
				oscc.SetSecurityContext(pod, psc, annotations, cscs)
				pspsrs.AllowedBy = ref
				if len(pspr.Spec.Template.Spec.ServiceAccountName) > 0 {
					pspsrs.Template.Spec = pod.Spec
				}

				sapsprs := securityapi.ServiceAccountPodSecurityPolicyReviewStatus{pspsrs, sa.Name}
				newStatus.AllowedServiceAccounts = append(newStatus.AllowedServiceAccounts, sapsprs)
				return nil
			}); err != nil {
			errs = append(errs, fmt.Errorf("unable to assign SecurityContextConstraints for ServiceAccount %s: %v", sa.Name, err))
			continue
		}
	}

	if len(errs) > 0 {
		glog.V(4).Infof("PodSecurityPolicyReview err: %v", kerrors.NewAggregate(errs))
	}

	pspr.Status = newStatus
	return pspr, nil
}

func getServiceAccounts(psprSpec securityapi.PodSecurityPolicyReviewSpec, saCache oscache.StoreToServiceAccountLister, namespace string) ([]*kapi.ServiceAccount, error) {
	serviceAccounts := []*kapi.ServiceAccount{}
	//  TODO: express 'all service accounts'
	//if serviceAccountList, err := client.Core().ServiceAccounts(namespace).List(kapi.ListOptions{}); err == nil {
	//	serviceAccounts = serviceAccountList.Items
	//	return serviceAccounts, fmt.Errorf("unable to retrieve service accounts: %v", err)
	//}

	if len(psprSpec.ServiceAccountNames) > 0 {
		errs := []error{}
		for _, saName := range psprSpec.ServiceAccountNames {
			sa, err := saCache.ServiceAccounts(namespace).Get(saName)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to retrieve ServiceAccount %s: %v", saName, err))
			}
			serviceAccounts = append(serviceAccounts, sa)
		}
		return serviceAccounts, kerrors.NewAggregate(errs)
	}
	saName := "default"
	if len(psprSpec.Template.Spec.ServiceAccountName) > 0 {
		saName = psprSpec.Template.Spec.ServiceAccountName
	}
	sa, err := saCache.ServiceAccounts(namespace).Get(saName)
	if err != nil {
		return serviceAccounts, fmt.Errorf("unable to retrieve ServiceAccount %s: %v", saName, err)
	}
	serviceAccounts = append(serviceAccounts, sa)
	return serviceAccounts, nil
}
