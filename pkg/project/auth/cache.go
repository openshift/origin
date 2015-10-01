package auth

import (
	"fmt"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	policyclient "github.com/openshift/origin/pkg/authorization/client"
)

// Lister enforces ability to enumerate a resource based on policy
type Lister interface {
	// List returns the list of Namespace items that the user can access
	List(user user.Info) (*kapi.NamespaceList, error)
}

// subjectRecord is a cache record for the set of namespaces a subject can access
type subjectRecord struct {
	subject    string
	namespaces sets.String
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

type skipSynchronizer interface {
	// SkipSynchronize returns true if if its safe to skip synchronization of the cache based on provided token from previous observation
	SkipSynchronize(prevState string, versionedObjects ...LastSyncResourceVersioner) (skip bool, currentState string)
}

// LastSyncResourceVersioner is any object that can divulge a LastSyncResourceVersion
type LastSyncResourceVersioner interface {
	LastSyncResourceVersion() string
}

type unchangingLastSyncResourceVersioner struct{}

func (u unchangingLastSyncResourceVersioner) LastSyncResourceVersion() string {
	return "0"
}

type statelessSkipSynchronizer struct{}

func (rs *statelessSkipSynchronizer) SkipSynchronize(prevState string, versionedObjects ...LastSyncResourceVersioner) (skip bool, currentState string) {
	resourceVersions := []string{}
	for i := range versionedObjects {
		resourceVersions = append(resourceVersions, versionedObjects[i].LastSyncResourceVersion())
	}
	currentState = strings.Join(resourceVersions, ",")
	skip = currentState == prevState

	return skip, currentState
}

type neverSkipSynchronizer struct{}

func (s *neverSkipSynchronizer) SkipSynchronize(prevState string, versionedObjects ...LastSyncResourceVersioner) (bool, string) {
	return false, ""
}

// AuthorizationCache maintains a cache on the set of namespaces a user or group can access.
type AuthorizationCache struct {
	namespaceStore            cache.Store
	namespaceInterface        kclient.NamespaceInterface
	lastSyncResourceVersioner LastSyncResourceVersioner

	policyClient policyclient.ReadOnlyPolicyClient

	reviewRecordStore       cache.Store
	userSubjectRecordStore  cache.Store
	groupSubjectRecordStore cache.Store

	clusterBindingResourceVersions sets.String
	clusterPolicyResourceVersions  sets.String

	skip      skipSynchronizer
	lastState string

	reviewer Reviewer

	syncHandler func(request *reviewRequest, userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) error
}

// NewAuthorizationCache creates a new AuthorizationCache
func NewAuthorizationCache(reviewer Reviewer, namespaceInterface kclient.NamespaceInterface, policyClient policyclient.ReadOnlyPolicyClient) *AuthorizationCache {
	result := &AuthorizationCache{
		namespaceStore:            cache.NewStore(cache.MetaNamespaceKeyFunc),
		namespaceInterface:        namespaceInterface,
		lastSyncResourceVersioner: &unchangingLastSyncResourceVersioner{},

		clusterPolicyResourceVersions:  sets.NewString(),
		clusterBindingResourceVersions: sets.NewString(),

		policyClient: policyClient,

		reviewRecordStore:       cache.NewStore(reviewRecordKeyFn),
		userSubjectRecordStore:  cache.NewStore(subjectRecordKeyFn),
		groupSubjectRecordStore: cache.NewStore(subjectRecordKeyFn),

		reviewer: reviewer,
		skip:     &neverSkipSynchronizer{},
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
	ac.lastSyncResourceVersioner = namespaceReflector

	ac.skip = &statelessSkipSynchronizer{}

	go util.Forever(func() { ac.synchronize() }, period)
}

// synchronizeNamespaces synchronizes access over each namespace and returns a set of namespace names that were looked at in last sync
func (ac *AuthorizationCache) synchronizeNamespaces(userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) *sets.String {
	namespaceSet := sets.NewString()
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
	policyList, err := ac.policyClient.ReadOnlyPolicies(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		util.HandleError(err)
		return
	}
	for _, policy := range policyList.Items {
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
	policyBindingList, err := ac.policyClient.ReadOnlyPolicyBindings(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		util.HandleError(err)
		return
	}
	for _, policyBinding := range policyBindingList.Items {
		reviewRequest := &reviewRequest{
			namespace:                         policyBinding.Namespace,
			policyBindingUIDToResourceVersion: map[types.UID]string{policyBinding.UID: policyBinding.ResourceVersion},
		}
		if err := ac.syncHandler(reviewRequest, userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore); err != nil {
			util.HandleError(fmt.Errorf("error synchronizing: %v", err))
		}
	}
}

// purgeDeletedNamespaces will remove all namespaces enumerated in a reviewRecordStore that are not in the namespace set
func purgeDeletedNamespaces(namespaceSet *sets.String, userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) {
	reviewRecordItems := reviewRecordStore.List()
	for i := range reviewRecordItems {
		reviewRecord := reviewRecordItems[i].(*reviewRecord)
		if !namespaceSet.Has(reviewRecord.namespace) {
			deleteNamespaceFromSubjects(userSubjectRecordStore, reviewRecord.users, reviewRecord.namespace)
			deleteNamespaceFromSubjects(groupSubjectRecordStore, reviewRecord.groups, reviewRecord.namespace)
			reviewRecordStore.Delete(reviewRecord)
		}
	}
}

// invalidateCache returns true if there was a change in the cluster namespace that holds cluster policy and policy bindings
func (ac *AuthorizationCache) invalidateCache() bool {
	invalidateCache := false

	clusterPolicyList, err := ac.policyClient.ReadOnlyClusterPolicies().List(labels.Everything(), fields.Everything())
	if err != nil {
		util.HandleError(err)
		return invalidateCache
	}

	temporaryVersions := sets.NewString()
	for _, clusterPolicy := range clusterPolicyList.Items {
		temporaryVersions.Insert(clusterPolicy.ResourceVersion)
	}
	if (len(ac.clusterPolicyResourceVersions) != len(temporaryVersions)) || !ac.clusterPolicyResourceVersions.HasAll(temporaryVersions.List()...) {
		invalidateCache = true
		ac.clusterPolicyResourceVersions = temporaryVersions
	}

	clusterPolicyBindingList, err := ac.policyClient.ReadOnlyClusterPolicyBindings().List(labels.Everything(), fields.Everything())
	if err != nil {
		util.HandleError(err)
		return invalidateCache
	}

	temporaryVersions.Delete(temporaryVersions.List()...)
	for _, clusterPolicyBinding := range clusterPolicyBindingList.Items {
		temporaryVersions.Insert(clusterPolicyBinding.ResourceVersion)
	}
	if (len(ac.clusterBindingResourceVersions) != len(temporaryVersions)) || !ac.clusterBindingResourceVersions.HasAll(temporaryVersions.List()...) {
		invalidateCache = true
		ac.clusterBindingResourceVersions = temporaryVersions
	}
	return invalidateCache
}

// synchronize runs a a full synchronization over the cache data.  it must be run in a single-writer model, it's not thread-safe by design.
func (ac *AuthorizationCache) synchronize() {
	// if none of our internal reflectors changed, then we can skip reviewing the cache
	skip, currentState := ac.skip.SkipSynchronize(ac.lastState, ac.lastSyncResourceVersioner, ac.policyClient)
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

	// we were able to update our cache since this last observation period
	ac.lastState = currentState
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

	usersToRemove := sets.NewString()
	groupsToRemove := sets.NewString()
	if lastKnownValue != nil {
		usersToRemove.Insert(lastKnownValue.users...)
		usersToRemove.Delete(review.Users()...)
		groupsToRemove.Insert(lastKnownValue.groups...)
		groupsToRemove.Delete(review.Groups()...)
	}

	deleteNamespaceFromSubjects(userSubjectRecordStore, usersToRemove.List(), namespace)
	deleteNamespaceFromSubjects(groupSubjectRecordStore, groupsToRemove.List(), namespace)
	addSubjectsToNamespace(userSubjectRecordStore, review.Users(), namespace)
	addSubjectsToNamespace(groupSubjectRecordStore, review.Groups(), namespace)
	cacheReviewRecord(request, lastKnownValue, review, reviewRecordStore)
	return nil
}

// List returns the set of namespace names the user has access to view
func (ac *AuthorizationCache) List(userInfo user.Info) (*kapi.NamespaceList, error) {
	keys := sets.String{}
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

func (ac *AuthorizationCache) ReadyForAccess() bool {
	return len(ac.lastState) > 0
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

// deleteNamespaceFromSubjects removes the namespace from each subject
// if no other namespaces are active to that subject, it will also delete the subject from the cache entirely
func deleteNamespaceFromSubjects(subjectRecordStore cache.Store, subjects []string, namespace string) {
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
			item = &subjectRecord{subject: subject, namespaces: sets.NewString()}
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
