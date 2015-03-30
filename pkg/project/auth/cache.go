package auth

import (
	"fmt"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/types"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
)

// Lister enforces ability to enumerate a resource based on policy
type Lister interface {
	// List returns the list of Namespace items that the user can access
	List(user user.Info) (*kapi.NamespaceList, error)
}

// subjectRecord is a cache record for the set of namespaces a subject can access
type subjectRecord struct {
	subject    string
	namespaces util.StringSet
}

// reviewRequest is the resource we want to review
type reviewRequest struct {
	namespace string
	// the resource version of the namespace that was observed to make this request
	namespaceResourceVersion string
	// the map of policy uid to resource version that was observed to make this request
	policyUIDToResourceVersion map[types.UID]string
	// the map of policy binding uid to resource version that was observed to make this request
	policyBindingUIDToResourceVersion map[types.UID]string
}

// reviewRecord is a cache record for the result of a resource access review
type reviewRecord struct {
	*reviewRequest
	users  []string
	groups []string
}

// reviewRecordKeyFn is a key func for reviewRecord objects
func reviewRecordKeyFn(obj interface{}) (string, error) {
	reviewRecord, ok := obj.(*reviewRecord)
	if !ok {
		return "", fmt.Errorf("expected reviewRecord")
	}
	return reviewRecord.namespace, nil
}

// subjectRecordKeyFn is a key func for subjectRecord objects
func subjectRecordKeyFn(obj interface{}) (string, error) {
	subjectRecord, ok := obj.(*subjectRecord)
	if !ok {
		return "", fmt.Errorf("expected subjectRecord")
	}
	return subjectRecord.subject, nil
}

// AuthorizationCache maintains a cache on the set of namespaces a user or group can access.
type AuthorizationCache struct {
	namespaceStore          cache.Store
	policyBindingIndexer    cache.Indexer
	policyIndexer           cache.Indexer
	reviewRecordStore       cache.Store
	userSubjectRecordStore  cache.Store
	groupSubjectRecordStore cache.Store

	masterNamespace              string
	masterBindingResourceVersion string
	masterPolicyResourceVersion  string

	reviewer Reviewer

	namespaceInterface       kclient.NamespaceInterface
	policyBindingsNamespacer client.PolicyBindingsNamespacer
	policiesNamespacer       client.PoliciesNamespacer

	syncHandler func(request *reviewRequest, userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) error
}

// NewAuthorizationCache creates a new AuthorizationCache
func NewAuthorizationCache(reviewer Reviewer, namespaceInterface kclient.NamespaceInterface, policyBindingsNamespacer client.PolicyBindingsNamespacer, policiesNamespacer client.PoliciesNamespacer, masterNamespace string) *AuthorizationCache {
	result := &AuthorizationCache{
		namespaceStore:               cache.NewStore(cache.MetaNamespaceKeyFunc),
		policyIndexer:                cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}),
		policyBindingIndexer:         cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}),
		reviewRecordStore:            cache.NewStore(reviewRecordKeyFn),
		userSubjectRecordStore:       cache.NewStore(subjectRecordKeyFn),
		groupSubjectRecordStore:      cache.NewStore(subjectRecordKeyFn),
		masterNamespace:              masterNamespace,
		masterBindingResourceVersion: "",
		masterPolicyResourceVersion:  "",
		namespaceInterface:           namespaceInterface,
		policyBindingsNamespacer:     policyBindingsNamespacer,
		policiesNamespacer:           policiesNamespacer,
		reviewer:                     reviewer,
	}
	result.syncHandler = result.syncRequest
	return result
}

// Run begins watching and synchronizing the cache
func (ac *AuthorizationCache) Run(period time.Duration) {

	namespaceReflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return ac.namespaceInterface.List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return ac.namespaceInterface.Watch(labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&kapi.Namespace{},
		ac.namespaceStore,
		2*time.Minute,
	)
	namespaceReflector.Run()

	policyBindingReflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return ac.policyBindingsNamespacer.PolicyBindings(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return ac.policyBindingsNamespacer.PolicyBindings(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&authorizationapi.PolicyBinding{},
		ac.policyBindingIndexer,
		2*time.Minute,
	)
	policyBindingReflector.Run()

	policyReflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return ac.policiesNamespacer.Policies(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return ac.policiesNamespacer.Policies(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&authorizationapi.Policy{},
		ac.policyIndexer,
		2*time.Minute,
	)
	policyReflector.Run()

	go util.Forever(func() { ac.synchronize() }, period)
}

// synchronizeNamespaces synchronizes access over each namespace and returns a set of namespace names that were looked at in last sync
func (ac *AuthorizationCache) synchronizeNamespaces(userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) *util.StringSet {
	namespaceSet := util.NewStringSet()
	items := ac.namespaceStore.List()
	for i := range items {
		namespace := items[i].(*kapi.Namespace)
		namespaceSet.Insert(namespace.Name)
		reviewRequest := &reviewRequest{
			namespace:                namespace.Name,
			namespaceResourceVersion: namespace.ResourceVersion,
		}
		if err := ac.syncHandler(reviewRequest, userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore); err != nil {
			util.HandleError(fmt.Errorf("error synchronizing: %v", err))
		}
	}
	return &namespaceSet
}

// synchronizePolicies synchronizes access over each policy
func (ac *AuthorizationCache) synchronizePolicies(userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) {
	items := ac.policyIndexer.List()
	for i := range items {
		policy := items[i].(*authorizationapi.Policy)
		reviewRequest := &reviewRequest{
			namespace:                  policy.Namespace,
			policyUIDToResourceVersion: map[types.UID]string{policy.UID: policy.ResourceVersion},
		}
		if err := ac.syncHandler(reviewRequest, userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore); err != nil {
			util.HandleError(fmt.Errorf("error synchronizing: %v", err))
		}
	}
}

// synchronizePolicyBindings synchronizes access over each policy binding
func (ac *AuthorizationCache) synchronizePolicyBindings(userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) {
	items := ac.policyBindingIndexer.List()
	for i := range items {
		binding := items[i].(*authorizationapi.PolicyBinding)
		reviewRequest := &reviewRequest{
			namespace:                         binding.Namespace,
			policyBindingUIDToResourceVersion: map[types.UID]string{binding.UID: binding.ResourceVersion},
		}
		if err := ac.syncHandler(reviewRequest, userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore); err != nil {
			util.HandleError(fmt.Errorf("error synchronizing: %v", err))
		}
	}
}

// purgeDeletedNamespaces will remove all namespaces enumerated in a reviewRecordStore that are not in the namespace set
func purgeDeletedNamespaces(namespaceSet *util.StringSet, userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) {
	reviewRecordItems := reviewRecordStore.List()
	for i := range reviewRecordItems {
		reviewRecord := reviewRecordItems[i].(*reviewRecord)
		if !namespaceSet.Has(reviewRecord.namespace) {
			deleteSubjectsToNamespace(userSubjectRecordStore, reviewRecord.users, reviewRecord.namespace)
			deleteSubjectsToNamespace(groupSubjectRecordStore, reviewRecord.groups, reviewRecord.namespace)
			reviewRecordStore.Delete(reviewRecord)
		}
	}
}

// invalidateCache returns true if there was a change in the master namespace that holds global policy and policy bindings
func (ac *AuthorizationCache) invalidateCache() bool {
	invalidateCache := false

	masterPolicies, err := ac.policyIndexer.Index("namespace", &authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Namespace: ac.masterNamespace}})
	if err != nil {
		return true
	}
	for i := range masterPolicies {
		policy := masterPolicies[i].(*authorizationapi.Policy)
		if policy.ResourceVersion != ac.masterPolicyResourceVersion {
			invalidateCache = true
			ac.masterPolicyResourceVersion = policy.ResourceVersion
		}
	}

	masterPolicyBindings, err := ac.policyBindingIndexer.Index("namespace", &authorizationapi.PolicyBinding{ObjectMeta: kapi.ObjectMeta{Namespace: ac.masterNamespace}})
	if err != nil {
		return true
	}
	for i := range masterPolicyBindings {
		policyBinding := masterPolicyBindings[i].(*authorizationapi.PolicyBinding)
		if policyBinding.ResourceVersion != ac.masterBindingResourceVersion {
			invalidateCache = true
			ac.masterBindingResourceVersion = policyBinding.ResourceVersion
		}
	}
	return invalidateCache
}

// synchronize runs a a full synchronization over the cache data.  it must be run in a single-writer model, it's not thread-safe by design.
func (ac *AuthorizationCache) synchronize() {
	// TODO: upstream cache object should support a high-water mark, if there was no change in any of our caches, then we should be able to return quickly
	skip := false
	if skip {
		return
	}

	// by default, we update our current caches and do an incremental change
	userSubjectRecordStore := ac.userSubjectRecordStore
	groupSubjectRecordStore := ac.groupSubjectRecordStore
	reviewRecordStore := ac.reviewRecordStore

	// if there was a global change that forced complete invalidation, we rebuild our cache and do a fast swap at end
	invalidateCache := ac.invalidateCache()
	if invalidateCache {
		userSubjectRecordStore = cache.NewStore(subjectRecordKeyFn)
		groupSubjectRecordStore = cache.NewStore(subjectRecordKeyFn)
		reviewRecordStore = cache.NewStore(reviewRecordKeyFn)
	}

	// iterate over caches and synchronize our three caches
	namespaceSet := ac.synchronizeNamespaces(userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore)
	ac.synchronizePolicies(userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore)
	ac.synchronizePolicyBindings(userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore)
	purgeDeletedNamespaces(namespaceSet, userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore)

	// if we did a full rebuild, now we swap the fully rebuilt cache
	if invalidateCache {
		ac.userSubjectRecordStore = userSubjectRecordStore
		ac.groupSubjectRecordStore = groupSubjectRecordStore
		ac.reviewRecordStore = reviewRecordStore
	}

}

// syncRequest takes a reviewRequest and determines if it should update the caches supplied, it is not thread-safe
func (ac *AuthorizationCache) syncRequest(request *reviewRequest, userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) error {

	lastKnownValue, err := lastKnown(reviewRecordStore, request.namespace)
	if err != nil {
		return err
	}

	if skipReview(request, lastKnownValue) {
		return nil
	}

	namespace := request.namespace
	review, err := ac.reviewer.Review(namespace)
	if err != nil {
		return err
	}

	usersToRemove := util.NewStringSet()
	groupsToRemove := util.NewStringSet()
	if lastKnownValue != nil {
		usersToRemove.Insert(lastKnownValue.users...)
		usersToRemove.Delete(review.Users()...)
		groupsToRemove.Insert(lastKnownValue.groups...)
		groupsToRemove.Delete(review.Groups()...)
	}

	deleteSubjectsToNamespace(userSubjectRecordStore, usersToRemove.List(), namespace)
	deleteSubjectsToNamespace(groupSubjectRecordStore, groupsToRemove.List(), namespace)
	addSubjectsToNamespace(userSubjectRecordStore, review.Users(), namespace)
	addSubjectsToNamespace(groupSubjectRecordStore, review.Groups(), namespace)
	cacheReviewRecord(request, lastKnownValue, review, reviewRecordStore)
	return nil
}

// List returns the set of namespace names the user has access to view
func (ac *AuthorizationCache) List(userInfo user.Info) (*kapi.NamespaceList, error) {
	keys := util.StringSet{}
	user := userInfo.GetName()
	groups := userInfo.GetGroups()

	obj, exists, _ := ac.userSubjectRecordStore.GetByKey(user)
	if exists {
		subjectRecord := obj.(*subjectRecord)
		keys.Insert(subjectRecord.namespaces.List()...)
	}

	for _, group := range groups {
		obj, exists, _ := ac.groupSubjectRecordStore.GetByKey(group)
		if exists {
			subjectRecord := obj.(*subjectRecord)
			keys.Insert(subjectRecord.namespaces.List()...)
		}
	}

	namespaceList := &kapi.NamespaceList{}
	for key := range keys {
		namespace, exists, err := ac.namespaceStore.GetByKey(key)
		if err != nil {
			return nil, err
		}
		if exists {
			namespaceList.Items = append(namespaceList.Items, *namespace.(*kapi.Namespace))
		}
	}
	return namespaceList, nil
}

// skipReview returns true if the request was satisfied by the lastKnown
func skipReview(request *reviewRequest, lastKnownValue *reviewRecord) bool {

	// if your request is nil, you have no reason to make a review
	if request == nil {
		return true
	}

	// if you know nothing from a prior review, you better make a request
	if lastKnownValue == nil {
		return false
	}
	// if you are asking about a specific namespace, and you think you knew about a different one, you better check again
	if request.namespace != lastKnownValue.namespace {
		return false
	}

	// if you are making your request relative to a specific resource version, only make it if its different
	if len(request.namespaceResourceVersion) > 0 && request.namespaceResourceVersion != lastKnownValue.namespaceResourceVersion {
		return false
	}

	// if you see a new policy binding, or a newer version, we need to do a review
	for k, v := range request.policyBindingUIDToResourceVersion {
		oldValue, exists := lastKnownValue.policyBindingUIDToResourceVersion[k]
		if !exists || v != oldValue {
			return false
		}
	}

	// if you see a new policy, or a newer version, we need to do a review
	for k, v := range request.policyUIDToResourceVersion {
		oldValue, exists := lastKnownValue.policyUIDToResourceVersion[k]
		if !exists || v != oldValue {
			return false
		}
	}
	return true
}

// deleteSubjectsToNamespace removes the namespace from each subject
// if no other namespaces are active to that subject, it will also delete the subject from the cache entirely
func deleteSubjectsToNamespace(subjectRecordStore cache.Store, subjects []string, namespace string) {
	for _, subject := range subjects {
		obj, exists, _ := subjectRecordStore.GetByKey(subject)
		if exists {
			subjectRecord := obj.(*subjectRecord)
			delete(subjectRecord.namespaces, namespace)
			if len(subjectRecord.namespaces) == 0 {
				subjectRecordStore.Delete(subjectRecord)
			}
		}
	}
}

// addSubjectsToNamespace adds the specified namespace to each subject
func addSubjectsToNamespace(subjectRecordStore cache.Store, subjects []string, namespace string) {
	for _, subject := range subjects {
		var item *subjectRecord
		obj, exists, _ := subjectRecordStore.GetByKey(subject)
		if exists {
			item = obj.(*subjectRecord)
		} else {
			item = &subjectRecord{subject: subject, namespaces: util.NewStringSet()}
			subjectRecordStore.Add(item)
		}
		item.namespaces.Insert(namespace)
	}
}

// cacheReviewRecord updates the cache based on the request processed
func cacheReviewRecord(request *reviewRequest, lastKnownValue *reviewRecord, review Review, reviewRecordStore cache.Store) {
	reviewRecord := &reviewRecord{
		reviewRequest: &reviewRequest{namespace: request.namespace, policyUIDToResourceVersion: map[types.UID]string{}, policyBindingUIDToResourceVersion: map[types.UID]string{}},
		groups:        review.Groups(),
		users:         review.Users(),
	}
	// keep what we last believe we knew by default
	if lastKnownValue != nil {
		reviewRecord.namespaceResourceVersion = lastKnownValue.namespaceResourceVersion
		for k, v := range lastKnownValue.policyUIDToResourceVersion {
			reviewRecord.policyUIDToResourceVersion[k] = v
		}
		for k, v := range lastKnownValue.policyBindingUIDToResourceVersion {
			reviewRecord.policyBindingUIDToResourceVersion[k] = v
		}
	}

	// update the review record relative to what drove this request
	if len(request.namespaceResourceVersion) > 0 {
		reviewRecord.namespaceResourceVersion = request.namespaceResourceVersion
	}
	for k, v := range request.policyUIDToResourceVersion {
		reviewRecord.policyUIDToResourceVersion[k] = v
	}
	for k, v := range request.policyBindingUIDToResourceVersion {
		reviewRecord.policyBindingUIDToResourceVersion[k] = v
	}
	// update the cache record
	reviewRecordStore.Add(reviewRecord)
}

func lastKnown(reviewRecordStore cache.Store, namespace string) (*reviewRecord, error) {
	obj, exists, err := reviewRecordStore.GetByKey(namespace)
	if err != nil {
		return nil, err
	}
	if exists {
		return obj.(*reviewRecord), nil
	}
	return nil, nil
}
