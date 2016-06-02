package podspecreview

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/serviceaccount"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/security/admission"
	securityapi "github.com/openshift/origin/pkg/security/api"
	securityvalidation "github.com/openshift/origin/pkg/security/api/validation"
	securitycache "github.com/openshift/origin/pkg/security/cache"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	securityCache *securitycache.SecurityCache
	client        clientset.Interface
}

// NewREST creates a new REST for policies..
func NewREST(c *securitycache.SecurityCache) *REST {
	return &REST{securityCache: c}
}

// New creates a new PodSpecReview object
func (r *REST) New() runtime.Object {
	return &securityapi.PodSpecReview{}
}

// Create registers a given new PodSpecReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	podSpecReview, ok := obj.(*securityapi.PodSpecReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a podspecreview: %#v", obj))
	}
	if errs := securityvalidation.ValidatePodSpecReview(podSpecReview); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(securityapi.Kind(podSpecReview.Kind), "", errs)
	}
	newPodSpecReview := &securityapi.PodSpecReview{}
	newPodSpecReview.Spec = podSpecReview.Spec

	namespace := ""
	// iterator over all serviceAccounts
	serviceAccoutList, err := r.client.Core().ServiceAccounts(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return podSpecReview, fmt.Errorf("unable to retrieve service accounts: %v", err)
	}

	errs := []error{}
	newStatus := securityapi.PodSpecReviewStatus{}
	for _, sa := range serviceAccoutList.Items {
		//		podSpec := kapi.PodSpec{}
		userInfo := serviceaccount.UserInfo(namespace, sa.Name, "") // TODO: @sdminone
		saConstraints, err := admission.GetMatchingSecurityContextConstraints(r.securityCache, userInfo)
		if err != nil {
			//TODO: @sdminonne
		}
		for scc, _ := range saConstraints {
			fmt.Printf("---------------> %v\n", scc)
		}
		//... TODO:
	}
	if len(errs) > 0 {
		return podSpecReview, kerrors.NewAggregate(errs)
	}
	podSpecReview.Status = newStatus
	return podSpecReview, nil
}
