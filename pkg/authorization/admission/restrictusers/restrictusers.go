package restrictusers

import (
	"errors"
	"io"
	"sort"
	"sync"
	"time"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/quota"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	oclient "github.com/openshift/origin/pkg/client"
	ocache "github.com/openshift/origin/pkg/client/cache"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/controller/shared"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
)

const (
	UserLabelSelectorAnnotation       = "authorization.openshift.io/user-selector"
	RestrictServiceAccountsAnnotation = "authorization.openshift.io/restrict-service-accounts"
	GroupLabelSelectorAnnotation      = "authorization.openshift.io/group-selector"
)

func init() {
	admission.RegisterPlugin("RestrictSubjectBindings",
		func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
			return NewClusterResourceQuota()
		})
}

// restrictUsersAdmission implements an admission controller that can enforce user constraints
type restrictUsersAdmission struct {
	*admission.Handler

	userLister          *ocache.IndexerToUserLister
	userSynced          func() bool
	groupLister         *ocache.IndexerToGroupLister
	groupSynced         func() bool
	namespaceLister     *ocache.IndexerToNamespaceLister
	namespaceSynced     func() bool
	policyBindingLister oclient.PolicyBindingsListerNamespacer
	policyBindingSynced func() bool
}

var _ oadmission.WantsInformers = &restrictUsersAdmission{}
var _ oadmission.Validator = &restrictUsersAdmission{}

const (
	timeToWaitForCacheSync = 10 * time.Second
)

// NewClusterResourceQuota configures an admission controller that can enforce user constraints
// using the provided registry.  The registry must have the capability to handle group/kinds that
// are persisted by the server this admission controller is intercepting
func NewClusterResourceQuota() (admission.Interface, error) {
	return &restrictUsersAdmission{
		Handler:     admission.NewHandler(admission.Create),
		lockFactory: NewDefaultLockFactory(),
	}, nil
}

// Admit makes admission decisions while enforcing user
func (q *restrictUsersAdmission) Admit(a admission.Attributes) (err error) {
	if a.GetResource().GroupResource() != authorizationapi.Resource("rolebindings") {
		return nil
	}
	// ignore all operations that correspond to sub-resource actions
	if len(a.GetSubresource()) != 0 {
		return nil
	}
	// ignore cluster level resources
	if len(a.GetNamespace()) == 0 {
		return nil
	}

	if !q.waitForSyncedStore(time.After(timeToWaitForCacheSync)) {
		return admission.NewForbidden(a, errors.New("caches not synchronized"))
	}

	// only bother checking deltas
	newSubjects := []kapi.ObjectReference{}
	newUsers := []kapi.ObjectReference{}
	newGroups := []kapi.ObjectReference{}
	newServiceAccounts := []kapi.ObjectReference{}

	binding, ok := a.GetObject().(*authorizationapi.RoleBinding)
	if !ok {
		return admission.NewForbidden(a, fmt.Errorf("wrong object type: %t", a.GetObject()))
	}
	if existingPolicyBinding, _ := q.policyBindingLister.PolicyBindings(a.GetNamespace()).Get(); existingPolicyBinding != nil {
		if existingBinding := existingPolicyBinding.RoleBindings[binding.Name]; existingBinding != nil {
			for _, newSubject := range binding.Subjects {
				for _, existingSubject := range existingBinding.Subjects {
					if existingSubject == newSubject {
						newSubjects = append(newSubjects, newSubject)
						break
					}
				}
			}
		} else {
			newSubjects = binding.Subjects
		}
	}

	for _, subject := range newSubjects {
		switch subject.Kind {
		case "SystemUser", "SystemGroup":
			continue
		case "User":
			newUsers = append(newUsers, subject)
		case "Group":
			newGroups = append(newGroups, subject)
		case "ServiceAccount":
			newServiceAccounts = append(newServiceAccounts, subject)
		}
	}

	namespace, err := q.namespaceLister.Get(a.GetNamespace())
	if err != nil {
		return nil
	}

	if userRestrictions := namespace.Annotations[UserLabelSelectorAnnotation]; len(userRestrictions) > 0 && len(newUsers) > 0 {
		// TODO cache these in an LRU to avoid  reparse
		labelSelector, err := unversioned.ParseToLabelSelector(userRestrictions)
		if err != nil {
			return admission.NewForbidden(a, err)
		}
		selector, err := unversioned.LabelSelectorAsSelector(labelSelector)
		if err != nil {
			return admission.NewForbidden(a, err)
		}

		for _, userSubject := range newUsers {
			user, err := q.userLister.Get(userSubject.Name)
			if err != nil {
				return admission.NewForbidden(a, err)
			}

			if !selector.Matches(labels.Set(user.Labels)) {
				return admission.NewForbidden(a, fmt.Errorf("%v may not be added to %v", user.Name, namespace.Name))
			}
		}
	}

	return nil
}

func (q *clusterQuotaAdmission) waitForSyncedStore(timeout <-chan time.Time) bool {
	for !q.userSynced() || !q.groupSynced() || !q.namespaceSynced() || !q.policyBindingSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return q.userSynced() && q.groupSynced() && q.namespaceSynced() && q.policyBindingSynced()
		}
	}

	return true
}

func (q *restrictUsersAdmission) SetInformers(informers shared.InformerFactory) {
	q.userLister = informers.Users().Lister()
	q.userSynced = informers.Users().Informer().HasSynced
	q.groupLister = informers.Groups().Lister()
	q.groupSynced = informers.Groups().Informer().HasSynced
	q.namespaceLister = informers.Namespaces().Lister()
	q.namespaceSynced = informers.Namespaces().Informer().HasSynced
	q.policyBindingLister = informers.PolicyBindings().Lister()
	q.policyBindingSynced = informers.PolicyBindings().Informer().HasSynced
}

func (q *restrictUsersAdmission) Validate() error {
	if q.userLister == nil {
		return errors.New("missing userLister")
	}
	if q.groupLister == nil {
		return errors.New("missing groupLister")
	}
	if q.namespaceLister == nil {
		return errors.New("missing namespaceLister")
	}
	if q.policyBindingLister == nil {
		return errors.New("missing policyBindingLister")
	}

	return nil
}
