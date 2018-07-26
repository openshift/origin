package accessrestriction

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1alpha1 "github.com/openshift/api/authorization/v1alpha1"
	userv1 "github.com/openshift/api/user/v1"
	authorizationinformers "github.com/openshift/client-go/authorization/informers/externalversions/authorization/v1alpha1"
	authorizationlisters "github.com/openshift/client-go/authorization/listers/authorization/v1alpha1"
	userinformers "github.com/openshift/client-go/user/informers/externalversions/user/v1"
	userlisters "github.com/openshift/client-go/user/listers/user/v1"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"

	"github.com/golang/glog"
)

func NewAuthorizer(accessRestrictionInformer authorizationinformers.AccessRestrictionInformer, userInformer userinformers.UserInformer, groupInformer userinformers.GroupInformer) authorizer.Authorizer {
	ar := accessRestrictionInformer.Informer()
	u := userInformer.Informer()
	g := groupInformer.Informer()

	return &accessRestrictionAuthorizer{
		synced: func() bool {
			return ar.HasSynced() && u.HasSynced() && g.HasSynced()
		},
		accessRestrictionLister: accessRestrictionInformer.Lister(),
		userLister:              userInformer.Lister(),
		groupLister:             groupInformer.Lister(),
	}
}

type accessRestrictionAuthorizer struct {
	synced                  cache.InformerSynced
	accessRestrictionLister authorizationlisters.AccessRestrictionLister
	userLister              userlisters.UserLister
	groupLister             userlisters.GroupLister
}

func (a *accessRestrictionAuthorizer) Authorize(requestAttributes authorizer.Attributes) (authorizer.Decision, string, error) {
	// prevent cycle based deadlock between the kube and openshift api servers
	if isIgnored(requestAttributes) {
		// resource is cluster scoped or in a reserved namespace so we state that we have no opinion
		// the reason must be blank, otherwise we would spam all RBAC denies with it (which is generally not useful)
		return authorizer.DecisionNoOpinion, "", nil
	}

	// make sure all of our informers are ready
	if !a.synced() {
		reason := "access restriction informers are not synced"
		glog.Infof("%s, denied request attributes %#v for user %#v", reason, requestAttributes, requestAttributes.GetUser())
		// fail closed (this should only occur for a short period of time when the master is starting)
		return authorizer.DecisionDeny, reason, nil
	}

	accessRestrictions, err := a.accessRestrictionLister.List(labels.Everything())
	if err != nil {
		reason := "cannot determine access restrictions"
		glog.Errorf("%s: %v, denied request attributes %#v for user %#v", reason, err, requestAttributes, requestAttributes.GetUser())
		// fail closed (but this should never happen because it means some static generated code is broken)
		return authorizer.DecisionDeny, reason, err
	}

	// check all access restrictions and only short circuit on affirmative deny
	for _, accessRestriction := range accessRestrictions {
		// does this access restriction match the given request attributes
		if !matches(accessRestriction, requestAttributes) {
			continue
		}

		// it does match, meaning we need to check if it denies the request
		if a.allowed(accessRestriction, requestAttributes.GetUser()) {
			glog.V(4).Infof("access restriction %#v matched but did not deny request attributes %#v for user %#v", accessRestriction, requestAttributes, requestAttributes.GetUser())
			continue
		}

		// deny the request because it is not allowed by the current access restriction
		// the reason is opaque because normal users have no visibility into access restriction objects
		glog.V(2).Infof("access restriction %#v denied request attributes %#v for user %#v", accessRestriction, requestAttributes, requestAttributes.GetUser())
		return authorizer.DecisionDeny, "denied by access restriction", nil
	}

	// no access restriction matched and denied this request, so we state that we have no opinion
	// the reason must be blank, otherwise we would spam all RBAC denies with it (which is generally not useful)
	return authorizer.DecisionNoOpinion, "", nil
}

func isIgnored(requestAttributes authorizer.Attributes) bool {
	// non-resource request is inherently cluster scoped
	if !requestAttributes.IsResourceRequest() {
		return true
	}

	ns := requestAttributes.GetNamespace()

	// is this cluster scoped
	if ns == v1.NamespaceNone {
		return true
	}

	// is this in a reserved namespace
	return clusterresourceoverride.IsExemptedNamespace(ns)
}

func matches(accessRestriction *authorizationv1alpha1.AccessRestriction, requestAttributes authorizer.Attributes) bool {
	if len(accessRestriction.Spec.MatchAttributes) == 0 {
		return true // fail closed (but validation prevents this)
	}
	return rbac.RulesAllow(requestAttributes, accessRestriction.Spec.MatchAttributes...)
}

func (a *accessRestrictionAuthorizer) allowed(accessRestriction *authorizationv1alpha1.AccessRestriction, user user.Info) bool {
	return a.subjectsMatch(accessRestriction.Spec.AllowedSubjects, user) || !a.subjectsMatch(accessRestriction.Spec.DeniedSubjects, user)
}

func (a *accessRestrictionAuthorizer) subjectsMatch(subjects []authorizationv1alpha1.SubjectMatcher, user user.Info) bool {
	for _, subject := range subjects {
		if a.subjectMatches(subject, user) {
			return true
		}
	}
	return false
}

func (a *accessRestrictionAuthorizer) subjectMatches(subject authorizationv1alpha1.SubjectMatcher, user user.Info) bool {
	switch {
	case subject.UserRestriction != nil && subject.GroupRestriction == nil:
		return a.userMatches(subject.UserRestriction, user)
	case subject.GroupRestriction != nil && subject.UserRestriction == nil:
		return a.groupMatches(subject.GroupRestriction, user)
	}
	return false // fail closed on whitelist, fail open on blacklist (but validation prevents this)
}

func (a *accessRestrictionAuthorizer) userMatches(userRestriction *authorizationv1.UserRestriction, user user.Info) bool {
	if has(userRestriction.Users, user.GetName()) {
		return true
	}
	if hasAny(userRestriction.Groups, user.GetGroups()) {
		return true
	}
	for _, labelSelector := range userRestriction.Selectors {
		for _, u := range a.labelSelectorToUsers(labelSelector) {
			if u.Name == user.GetName() || hasAny(u.Groups, user.GetGroups()) { // TODO not sure if we should check groups here
				return true
			}
		}
	}
	return false
}

func (a *accessRestrictionAuthorizer) labelSelectorToUsers(labelSelector v1.LabelSelector) []*userv1.User {
	users, err := a.userLister.List(labelSelectorAsSelector(labelSelector))
	if err != nil {
		runtime.HandleError(err) // this should never happen because it means some static generated code is broken
	}
	// it is safe to return this even when err != nil
	return users
}

func (a *accessRestrictionAuthorizer) groupMatches(groupRestriction *authorizationv1.GroupRestriction, user user.Info) bool {
	if hasAny(groupRestriction.Groups, user.GetGroups()) {
		return true
	}
	for _, labelSelector := range groupRestriction.Selectors {
		for _, group := range a.labelSelectorToGroups(labelSelector) {
			if has(user.GetGroups(), group.Name) || has(group.Users, user.GetName()) {
				return true
			}
		}
	}
	return false
}

func (a *accessRestrictionAuthorizer) labelSelectorToGroups(labelSelector v1.LabelSelector) []*userv1.Group {
	groups, err := a.groupLister.List(labelSelectorAsSelector(labelSelector))
	if err != nil {
		runtime.HandleError(err) // this should never happen because it means some static generated code is broken
	}
	// it is safe to return this even when err != nil
	return groups
}

func has(set []string, ele string) bool {
	for _, s := range set {
		if s == ele {
			return true
		}
	}
	return false
}

func hasAny(set, any []string) bool {
	for _, a := range any {
		if has(set, a) {
			return true
		}
	}
	return false
}

func labelSelectorAsSelector(labelSelector v1.LabelSelector) labels.Selector {
	selector, err := v1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		runtime.HandleError(err) // validation prevents this from occurring
		return labels.Nothing()  // fail closed on whitelist, fail open on blacklist
	}
	return selector
}
