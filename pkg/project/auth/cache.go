package auth

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	rbacinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/rbac/internalversion"
	rbaclisters "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
)

// Lister enforces ability to enumerate a resource based on role
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
	// the map of role uid to resource version that was observed to make this request
	roleUIDToResourceVersion map[types.UID]string
	// the map of role binding uid to resource version that was observed to make this request
	roleBindingUIDToResourceVersion map[types.UID]string
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

type unionLastSyncResourceVersioner []LastSyncResourceVersioner

func (u unionLastSyncResourceVersioner) LastSyncResourceVersion() string {
	resourceVersions := []string{}
	for _, versioner := range u {
		resourceVersions = append(resourceVersions, versioner.LastSyncResourceVersion())
	}
	return strings.Join(resourceVersions, "")
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

type SyncedClusterRoleLister interface {
	rbaclisters.ClusterRoleLister
	LastSyncResourceVersioner
}

type SyncedClusterRoleBindingLister interface {
	rbaclisters.ClusterRoleBindingLister
	LastSyncResourceVersioner
}
type SyncedRoleLister interface {
	rbaclisters.RoleLister
	LastSyncResourceVersioner
}
type SyncedRoleBindingLister interface {
	rbaclisters.RoleBindingLister
	LastSyncResourceVersioner
}

type syncedRoleLister struct {
	rbaclisters.RoleLister
	versioner LastSyncResourceVersioner
}

func (l syncedRoleLister) LastSyncResourceVersion() string {
	return l.versioner.LastSyncResourceVersion()
}

type syncedClusterRoleLister struct {
	rbaclisters.ClusterRoleLister
	versioner LastSyncResourceVersioner
}

func (l syncedClusterRoleLister) LastSyncResourceVersion() string {
	return l.versioner.LastSyncResourceVersion()
}

type syncedRoleBindingLister struct {
	rbaclisters.RoleBindingLister
	versioner LastSyncResourceVersioner
}

func (l syncedRoleBindingLister) LastSyncResourceVersion() string {
	return l.versioner.LastSyncResourceVersion()
}

type syncedClusterRoleBindingLister struct {
	rbaclisters.ClusterRoleBindingLister
	versioner LastSyncResourceVersioner
}

func (l syncedClusterRoleBindingLister) LastSyncResourceVersion() string {
	return l.versioner.LastSyncResourceVersion()
}

// AuthorizationCache maintains a cache on the set of namespaces a user or group can access.
type AuthorizationCache struct {
	// allKnownNamespaces we track all the known namespaces, so we can detect deletes.
	// TODO remove this in favor of a list/watch mechanism for projects
	allKnownNamespaces        sets.String
	namespaceStore            cache.Store
	lastSyncResourceVersioner LastSyncResourceVersioner

	clusterRoleLister             SyncedClusterRoleLister
	clusterRoleBindingLister      SyncedClusterRoleBindingLister
	roleNamespacer                SyncedRoleLister
	roleBindingNamespacer         SyncedRoleBindingLister
	roleLastSyncResourceVersioner LastSyncResourceVersioner

	reviewRecordStore       cache.Store
	userSubjectRecordStore  cache.Store
	groupSubjectRecordStore cache.Store

	clusterBindingResourceVersions sets.String
	clusterRoleResourceVersions    sets.String

	skip      skipSynchronizer
	lastState string

	reviewer Reviewer

	syncHandler func(request *reviewRequest, userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) error

	watchers    []CacheWatcher
	watcherLock sync.Mutex
}

// NewAuthorizationCache creates a new AuthorizationCache
func NewAuthorizationCache(
	namespaces cache.SharedIndexInformer,
	reviewer Reviewer,
	informers rbacinformers.Interface,
) *AuthorizationCache {
	scrLister := syncedClusterRoleLister{
		informers.ClusterRoles().Lister(),
		informers.ClusterRoles().Informer(),
	}
	scrbLister := syncedClusterRoleBindingLister{
		informers.ClusterRoleBindings().Lister(),
		informers.ClusterRoleBindings().Informer(),
	}
	srLister := syncedRoleLister{
		informers.Roles().Lister(),
		informers.Roles().Informer(),
	}
	srbLister := syncedRoleBindingLister{
		informers.RoleBindings().Lister(),
		informers.RoleBindings().Informer(),
	}
	ac := &AuthorizationCache{
		allKnownNamespaces: sets.String{},
		namespaceStore:     namespaces.GetStore(),

		clusterRoleResourceVersions:    sets.NewString(),
		clusterBindingResourceVersions: sets.NewString(),

		clusterRoleLister:             scrLister,
		clusterRoleBindingLister:      scrbLister,
		roleNamespacer:                srLister,
		roleBindingNamespacer:         srbLister,
		roleLastSyncResourceVersioner: unionLastSyncResourceVersioner{scrLister, scrbLister, srLister, srbLister},

		reviewRecordStore:       cache.NewStore(reviewRecordKeyFn),
		userSubjectRecordStore:  cache.NewStore(subjectRecordKeyFn),
		groupSubjectRecordStore: cache.NewStore(subjectRecordKeyFn),

		reviewer: reviewer,
		skip:     &neverSkipSynchronizer{},

		watchers: []CacheWatcher{},
	}
	ac.lastSyncResourceVersioner = namespaces.(LastSyncResourceVersioner)
	ac.syncHandler = ac.syncRequest
	return ac
}

// Run begins watching and synchronizing the cache
func (ac *AuthorizationCache) Run(period time.Duration) {
	ac.skip = &statelessSkipSynchronizer{}

	go utilwait.Forever(func() { ac.synchronize() }, period)
}

func (ac *AuthorizationCache) AddWatcher(watcher CacheWatcher) {
	ac.watcherLock.Lock()
	defer ac.watcherLock.Unlock()

	ac.watchers = append(ac.watchers, watcher)
}

func (ac *AuthorizationCache) RemoveWatcher(watcher CacheWatcher) {
	ac.watcherLock.Lock()
	defer ac.watcherLock.Unlock()

	lastIndex := len(ac.watchers) - 1
	for i := 0; i < len(ac.watchers); i++ {
		if ac.watchers[i] == watcher {
			if i < lastIndex {
				// if we're not the last element, shift
				copy(ac.watchers[i:], ac.watchers[i+1:])
			}
			ac.watchers = ac.watchers[:lastIndex]
			break
		}
	}
}

func (ac *AuthorizationCache) GetClusterRoleLister() SyncedClusterRoleLister {
	return ac.clusterRoleLister
}

// synchronizeNamespaces synchronizes access over each namespace and returns a set of namespace names that were looked at in last sync
func (ac *AuthorizationCache) synchronizeNamespaces(userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) sets.String {
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
			utilruntime.HandleError(fmt.Errorf("error synchronizing: %v", err))
		}
	}
	return namespaceSet
}

// synchronizePolicies synchronizes access over each role
func (ac *AuthorizationCache) synchronizePolicies(userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) {
	roleList, err := ac.roleNamespacer.Roles(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	for _, role := range roleList {
		reviewRequest := &reviewRequest{
			namespace:                role.Namespace,
			roleUIDToResourceVersion: map[types.UID]string{role.UID: role.ResourceVersion},
		}
		if err := ac.syncHandler(reviewRequest, userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore); err != nil {
			utilruntime.HandleError(fmt.Errorf("error synchronizing: %v", err))
		}
	}
}

// synchronizeRoleBindings synchronizes access over each role binding
func (ac *AuthorizationCache) synchronizeRoleBindings(userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) {
	roleBindingList, err := ac.roleBindingNamespacer.RoleBindings(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	for _, roleBinding := range roleBindingList {
		reviewRequest := &reviewRequest{
			namespace:                       roleBinding.Namespace,
			roleBindingUIDToResourceVersion: map[types.UID]string{roleBinding.UID: roleBinding.ResourceVersion},
		}
		if err := ac.syncHandler(reviewRequest, userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore); err != nil {
			utilruntime.HandleError(fmt.Errorf("error synchronizing: %v", err))
		}
	}
}

// purgeDeletedNamespaces will remove all namespaces enumerated in a reviewRecordStore that are not in the namespace set
func (ac *AuthorizationCache) purgeDeletedNamespaces(oldNamespaces, newNamespaces sets.String, userSubjectRecordStore cache.Store, groupSubjectRecordStore cache.Store, reviewRecordStore cache.Store) {
	reviewRecordItems := reviewRecordStore.List()
	for i := range reviewRecordItems {
		reviewRecord := reviewRecordItems[i].(*reviewRecord)
		if !newNamespaces.Has(reviewRecord.namespace) {
			deleteNamespaceFromSubjects(userSubjectRecordStore, reviewRecord.users, reviewRecord.namespace)
			deleteNamespaceFromSubjects(groupSubjectRecordStore, reviewRecord.groups, reviewRecord.namespace)
			reviewRecordStore.Delete(reviewRecord)
		}
	}

	for namespace := range oldNamespaces.Difference(newNamespaces) {
		ac.notifyWatchers(namespace, nil, sets.String{}, sets.String{})
	}
}

// invalidateCache returns true if there was a change in the cluster namespace that holds cluster role and role bindings
func (ac *AuthorizationCache) invalidateCache() bool {
	invalidateCache := false

	clusterRoleList, err := ac.clusterRoleLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return invalidateCache
	}

	temporaryVersions := sets.NewString()
	for _, clusterRole := range clusterRoleList {
		temporaryVersions.Insert(clusterRole.ResourceVersion)
	}
	if (len(ac.clusterRoleResourceVersions) != len(temporaryVersions)) || !ac.clusterRoleResourceVersions.HasAll(temporaryVersions.List()...) {
		invalidateCache = true
		ac.clusterRoleResourceVersions = temporaryVersions
	}

	clusterRoleBindingList, err := ac.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return invalidateCache
	}

	temporaryVersions.Delete(temporaryVersions.List()...)
	for _, clusterRoleBinding := range clusterRoleBindingList {
		temporaryVersions.Insert(clusterRoleBinding.ResourceVersion)
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
	skip, currentState := ac.skip.SkipSynchronize(ac.lastState, ac.lastSyncResourceVersioner, ac.roleLastSyncResourceVersioner)
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
	newKnownNamespaces := ac.synchronizeNamespaces(userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore)
	ac.synchronizePolicies(userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore)
	ac.synchronizeRoleBindings(userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore)
	ac.purgeDeletedNamespaces(ac.allKnownNamespaces, newKnownNamespaces, userSubjectRecordStore, groupSubjectRecordStore, reviewRecordStore)

	// if we did a full rebuild, now we swap the fully rebuilt cache
	if invalidateCache {
		ac.userSubjectRecordStore = userSubjectRecordStore
		ac.groupSubjectRecordStore = groupSubjectRecordStore
		ac.reviewRecordStore = reviewRecordStore
	}
	ac.allKnownNamespaces = newKnownNamespaces

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
	ac.notifyWatchers(namespace, lastKnownValue, sets.NewString(review.Users()...), sets.NewString(review.Groups()...))

	if errMsg := review.EvaluationError(); len(errMsg) > 0 {
		return errors.New(errMsg)
	}
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

	allowedNamespaces, err := scope.ScopesToVisibleNamespaces(userInfo.GetExtra()[authorizationapi.ScopesKey], ac.clusterRoleLister)
	if err != nil {
		return nil, err
	}

	namespaceList := &kapi.NamespaceList{}
	for _, key := range keys.List() {
		namespaceObj, exists, err := ac.namespaceStore.GetByKey(key)
		if err != nil {
			return nil, err
		}
		if exists {
			namespace := *namespaceObj.(*kapi.Namespace)
			if allowedNamespaces.Has("*") || allowedNamespaces.Has(namespace.Name) {
				namespaceList.Items = append(namespaceList.Items, namespace)
			}
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

	// if you see a new role binding, or a newer version, we need to do a review
	for k, v := range request.roleBindingUIDToResourceVersion {
		oldValue, exists := lastKnownValue.roleBindingUIDToResourceVersion[k]
		if !exists || v != oldValue {
			return false
		}
	}

	// if you see a new role, or a newer version, we need to do a review
	for k, v := range request.roleUIDToResourceVersion {
		oldValue, exists := lastKnownValue.roleUIDToResourceVersion[k]
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

func (ac *AuthorizationCache) notifyWatchers(namespace string, exists *reviewRecord, users, groups sets.String) {
	ac.watcherLock.Lock()
	defer ac.watcherLock.Unlock()
	for _, watcher := range ac.watchers {
		watcher.GroupMembershipChanged(namespace, users, groups)
	}
}

// cacheReviewRecord updates the cache based on the request processed
func cacheReviewRecord(request *reviewRequest, lastKnownValue *reviewRecord, review Review, reviewRecordStore cache.Store) {
	reviewRecord := &reviewRecord{
		reviewRequest: &reviewRequest{namespace: request.namespace, roleUIDToResourceVersion: map[types.UID]string{}, roleBindingUIDToResourceVersion: map[types.UID]string{}},
		groups:        review.Groups(),
		users:         review.Users(),
	}
	// keep what we last believe we knew by default
	if lastKnownValue != nil {
		reviewRecord.namespaceResourceVersion = lastKnownValue.namespaceResourceVersion
		for k, v := range lastKnownValue.roleUIDToResourceVersion {
			reviewRecord.roleUIDToResourceVersion[k] = v
		}
		for k, v := range lastKnownValue.roleBindingUIDToResourceVersion {
			reviewRecord.roleBindingUIDToResourceVersion[k] = v
		}
	}

	// update the review record relative to what drove this request
	if len(request.namespaceResourceVersion) > 0 {
		reviewRecord.namespaceResourceVersion = request.namespaceResourceVersion
	}
	for k, v := range request.roleUIDToResourceVersion {
		reviewRecord.roleUIDToResourceVersion[k] = v
	}
	for k, v := range request.roleBindingUIDToResourceVersion {
		reviewRecord.roleBindingUIDToResourceVersion[k] = v
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
