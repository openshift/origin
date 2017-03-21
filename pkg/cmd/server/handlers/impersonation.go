package handlers

import (
	"fmt"
	"net/http"

	kapi "k8s.io/kubernetes/pkg/api"
	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/httplog"
	"k8s.io/kubernetes/pkg/serviceaccount"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	userapi "github.com/openshift/origin/pkg/user/api"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
)

type GroupCache interface {
	GroupsFor(string) ([]*userapi.Group, error)
}

// ImpersonationFilter checks for impersonation rules against the current user.
func ImpersonationFilter(handler http.Handler, a kauthorizer.Authorizer, groupCache GroupCache, contextMapper kapi.RequestContextMapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestedUser := req.Header.Get(authenticationapi.ImpersonateUserHeader)
		if len(requestedUser) == 0 {
			handler.ServeHTTP(w, req)
			return
		}

		subjects := authorizationapi.BuildSubjects([]string{requestedUser}, req.Header[authenticationapi.ImpersonateGroupHeader],
			// validates whether the usernames are regular users or system users
			uservalidation.ValidateUserName,
			// validates group names are regular groups or system groups
			uservalidation.ValidateGroupName)

		ctx, exists := contextMapper.Get(req)
		if !exists {
			Forbidden("context not found", nil, w, req)
			return
		}
		oldUser, ok := kapi.UserFrom(ctx)
		if !ok {
			Forbidden("user not found", nil, w, req)
			return
		}

		// if groups are not specified, then we need to look them up differently depending on the type of user
		// if they are specified, then they are the authority
		groupsSpecified := len(req.Header[authenticationapi.ImpersonateGroupHeader]) > 0

		// make sure we're allowed to impersonate each subject.  While we're iterating through, start building username
		// and group information
		username := ""
		groups := []string{}
		for _, subject := range subjects {
			actingAsAttributes := &kauthorizer.AttributesRecord{
				User:            oldUser,
				Verb:            "impersonate",
				Namespace:       subject.Namespace,
				ResourceRequest: true,
			}

			gk := subject.GetObjectKind().GroupVersionKind().GroupKind()
			switch {
			case userapi.IsKindOrLegacy(authorizationapi.GroupKind, gk):
				actingAsAttributes.APIGroup = userapi.GroupName
				actingAsAttributes.Resource = authorizationapi.GroupResource
				actingAsAttributes.Name = subject.Name
				groups = append(groups, subject.Name)

			case userapi.IsKindOrLegacy(authorizationapi.SystemGroupKind, gk):
				actingAsAttributes.APIGroup = userapi.GroupName
				actingAsAttributes.Resource = authorizationapi.SystemGroupResource
				actingAsAttributes.Name = subject.Name
				groups = append(groups, subject.Name)

			case userapi.IsKindOrLegacy(authorizationapi.UserKind, gk):
				actingAsAttributes.APIGroup = userapi.GroupName
				actingAsAttributes.Resource = authorizationapi.UserResource
				actingAsAttributes.Name = subject.Name
				username = subject.Name
				if !groupsSpecified {
					if actualGroups, err := groupCache.GroupsFor(subject.Name); err == nil {
						for _, group := range actualGroups {
							groups = append(groups, group.Name)
						}
					}
					groups = append(groups, bootstrappolicy.AuthenticatedGroup, bootstrappolicy.AuthenticatedOAuthGroup)
				}

			case userapi.IsKindOrLegacy(authorizationapi.SystemUserKind, gk):
				actingAsAttributes.APIGroup = userapi.GroupName
				actingAsAttributes.Resource = authorizationapi.SystemUserResource
				actingAsAttributes.Name = subject.Name
				username = subject.Name
				if !groupsSpecified {
					if subject.Name == bootstrappolicy.UnauthenticatedUsername {
						groups = append(groups, bootstrappolicy.UnauthenticatedGroup)
					} else {
						groups = append(groups, bootstrappolicy.AuthenticatedGroup)
					}
				}

			case gk == kapi.Kind(authorizationapi.ServiceAccountKind):
				actingAsAttributes.APIGroup = kapi.GroupName
				actingAsAttributes.Resource = authorizationapi.ServiceAccountResource
				actingAsAttributes.Name = subject.Name
				username = serviceaccount.MakeUsername(subject.Namespace, subject.Name)
				if !groupsSpecified {
					groups = append(serviceaccount.MakeGroupNames(subject.Namespace, subject.Name), bootstrappolicy.AuthenticatedGroup)
				}

			default:
				Forbidden(fmt.Sprintf("unknown subject type: %v", subject), actingAsAttributes, w, req)
				return
			}

			allowed, reason, err := a.Authorize(actingAsAttributes)
			if err != nil {
				Forbidden(err.Error(), actingAsAttributes, w, req)
				return
			}
			if !allowed {
				Forbidden(reason, actingAsAttributes, w, req)
				return
			}
		}

		var extra map[string][]string
		if requestScopes, ok := req.Header[authenticationapi.ImpersonateUserScopeHeader]; ok {
			extra = map[string][]string{authorizationapi.ScopesKey: requestScopes}
		}

		newUser := &user.DefaultInfo{
			Name:   username,
			Groups: groups,
			Extra:  extra,
		}
		contextMapper.Update(req, kapi.WithUser(ctx, newUser))

		httplog.LogOf(req, w).Addf("%v is acting as %v", oldUser, newUser)

		handler.ServeHTTP(w, req)
	})
}
