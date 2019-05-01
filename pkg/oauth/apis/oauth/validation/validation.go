package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	routev1 "github.com/openshift/api/route/v1"
	authorizerscopes "github.com/openshift/origin/pkg/authorization/authorizer/scope"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/bootstrap"
	uservalidation "github.com/openshift/origin/pkg/user/apis/user/validation"
)

const (
	MinTokenLength = 32
	// MinimumInactivityTimeoutSeconds defines the the smallest value allowed
	// for AccessTokenInactivityTimeoutSeconds.
	// It also defines the ticker interval for the token update routine as
	// MinimumInactivityTimeoutSeconds / 3 is used there.
	MinimumInactivityTimeoutSeconds = 5 * 60
)

// PKCE [RFC7636] code challenge methods supported
// https://tools.ietf.org/html/rfc7636#section-4.3
const (
	codeChallengeMethodPlain  = "plain"
	codeChallengeMethodSHA256 = "S256"
)

var CodeChallengeMethodsSupported = []string{codeChallengeMethodPlain, codeChallengeMethodSHA256}

func ValidateTokenName(name string, prefix bool) []string {
	if reasons := path.ValidatePathSegmentName(name, prefix); len(reasons) != 0 {
		return reasons
	}

	if len(name) < MinTokenLength {
		return []string{fmt.Sprintf("must be at least %d characters long", MinTokenLength)}
	}
	return nil
}

func ValidateRedirectURI(redirect string) (bool, string) {
	if len(redirect) == 0 {
		return false, "may not be empty"
	}

	u, err := url.Parse(redirect)
	if err != nil {
		return false, err.Error()
	}
	if len(u.Fragment) != 0 {
		return false, "may not contain a fragment"
	}
	for _, s := range strings.Split(u.Path, "/") {
		if s == "." {
			return false, "may not contain a path segment of ."
		}
		if s == ".." {
			return false, "may not contain a path segment of .."
		}
	}
	return true, ""
}

func ValidateAccessToken(accessToken *oauthapi.OAuthAccessToken) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&accessToken.ObjectMeta, false, ValidateTokenName, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateClientNameField(accessToken.ClientName, field.NewPath("clientName"))...)
	allErrs = append(allErrs, ValidateUserNameField(accessToken.UserName, field.NewPath("userName"))...)
	allErrs = append(allErrs, ValidateScopes(accessToken.Scopes, field.NewPath("scopes"))...)

	if len(accessToken.UserUID) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("userUID"), ""))
	}
	// negative values are not allowed
	if accessToken.InactivityTimeoutSeconds < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("inactivityTimeoutSeconds"), accessToken.InactivityTimeoutSeconds, "cannot be a negative value"))
	}
	if ok, msg := ValidateRedirectURI(accessToken.RedirectURI); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("redirectURI"), accessToken.RedirectURI, msg))
	}

	if accessToken.ExpiresIn < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("expiresIn"), accessToken.ExpiresIn, "cannot be a negative value"))
	}

	return allErrs
}

func ValidateAccessTokenUpdate(newToken, oldToken *oauthapi.OAuthAccessToken) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newToken.ObjectMeta, &oldToken.ObjectMeta, field.NewPath("metadata"))
	// since InactivityTimeoutSeconds can be concurrently updated by multipe masters we do
	// not allow it to decrease in value, as that would cause "updated" tokens
	// to timeout earlier than they should. 0 is the exception, as this value
	// indicates that the token does not expire, and it is an allowed new
	// value.
	if newToken.InactivityTimeoutSeconds > 0 && newToken.InactivityTimeoutSeconds < oldToken.InactivityTimeoutSeconds {
		allErrs = append(allErrs, field.Invalid(field.NewPath("inactivityTimeoutSeconds"), newToken.InactivityTimeoutSeconds,
			fmt.Sprintf("cannot be less than the current value=%d", oldToken.InactivityTimeoutSeconds)))
	}
	// negative values are not allowed either
	if newToken.InactivityTimeoutSeconds < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("inactivityTimeoutSeconds"), newToken.InactivityTimeoutSeconds,
			"cannot be a negative value"))
	}
	// we do not allow tokens to turn into timing out tokens after issuance
	if oldToken.InactivityTimeoutSeconds == 0 && newToken.InactivityTimeoutSeconds != 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("inactivityTimeoutSeconds"), newToken.InactivityTimeoutSeconds,
			"cannot update non-timing-out token"))
	}
	copied := *oldToken
	copied.ObjectMeta = newToken.ObjectMeta
	// allow only InactivityTimeoutSeconds to be changed
	copied.InactivityTimeoutSeconds = newToken.InactivityTimeoutSeconds
	return append(allErrs, validation.ValidateImmutableField(newToken, &copied, field.NewPath(""))...)
}

var codeChallengeRegex = regexp.MustCompile("^[a-zA-Z0-9._~-]{43,128}$")

func ValidateAuthorizeToken(authorizeToken *oauthapi.OAuthAuthorizeToken) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&authorizeToken.ObjectMeta, false, ValidateTokenName, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateClientNameField(authorizeToken.ClientName, field.NewPath("clientName"))...)
	allErrs = append(allErrs, ValidateUserNameField(authorizeToken.UserName, field.NewPath("userName"))...)
	allErrs = append(allErrs, ValidateScopes(authorizeToken.Scopes, field.NewPath("scopes"))...)

	if len(authorizeToken.UserUID) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("userUID"), ""))
	}
	if ok, msg := ValidateRedirectURI(authorizeToken.RedirectURI); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("redirectURI"), authorizeToken.RedirectURI, msg))
	}

	if len(authorizeToken.CodeChallenge) > 0 || len(authorizeToken.CodeChallengeMethod) > 0 {
		switch {
		case len(authorizeToken.CodeChallenge) == 0:
			allErrs = append(allErrs, field.Required(field.NewPath("codeChallenge"), "required if codeChallengeMethod is specified"))
		case !codeChallengeRegex.MatchString(authorizeToken.CodeChallenge):
			allErrs = append(allErrs, field.Invalid(field.NewPath("codeChallenge"), authorizeToken.CodeChallenge, "must be 43-128 characters [a-zA-Z0-9.~_-]"))
		}

		switch authorizeToken.CodeChallengeMethod {
		case "":
			allErrs = append(allErrs, field.Required(field.NewPath("codeChallengeMethod"), "required if codeChallenge is specified"))
		case codeChallengeMethodPlain, codeChallengeMethodSHA256:
			// no-op, good
		default:
			allErrs = append(allErrs, field.NotSupported(field.NewPath("codeChallengeMethod"), authorizeToken.CodeChallengeMethod, CodeChallengeMethodsSupported))
		}
	}

	if authorizeToken.ExpiresIn <= 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("expiresIn"), authorizeToken.ExpiresIn, "must be greater than zero"))
	}

	return allErrs
}

func ValidateAuthorizeTokenUpdate(newToken, oldToken *oauthapi.OAuthAuthorizeToken) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newToken.ObjectMeta, &oldToken.ObjectMeta, field.NewPath("metadata"))
	copied := *oldToken
	copied.ObjectMeta = newToken.ObjectMeta
	return append(allErrs, validation.ValidateImmutableField(newToken, &copied, field.NewPath(""))...)
}

func ValidateClient(client *oauthapi.OAuthClient) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&client.ObjectMeta, false, apimachineryvalidation.NameIsDNSSubdomain, field.NewPath("metadata"))
	for i, redirect := range client.RedirectURIs {
		if ok, msg := ValidateRedirectURI(redirect); !ok {
			allErrs = append(allErrs, field.Invalid(field.NewPath("redirectURIs").Index(i), redirect, msg))
		}
	}

	for i, secret := range client.AdditionalSecrets {
		if len(secret) == 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("additionalSecrets").Index(i), "", "may not be empty"))
		}
	}

	for i, restriction := range client.ScopeRestrictions {
		allErrs = append(allErrs, ValidateScopeRestriction(restriction, field.NewPath("scopeRestrictions").Index(i))...)
	}

	if accessTokenMaxAgeSeconds := client.AccessTokenMaxAgeSeconds; accessTokenMaxAgeSeconds != nil {
		if *accessTokenMaxAgeSeconds < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("accessTokenMaxAgeSeconds"), *accessTokenMaxAgeSeconds, "value cannot be negative"))
		}
	}

	if client.AccessTokenInactivityTimeoutSeconds != nil {
		timeout := *client.AccessTokenInactivityTimeoutSeconds
		if timeout > 0 && timeout < MinimumInactivityTimeoutSeconds {
			msg := fmt.Sprintf("The minimum valid timeout value is %d seconds", MinimumInactivityTimeoutSeconds)
			allErrs = append(allErrs, field.Invalid(field.NewPath("accessTokenInactivityTimeoutSeconds"), timeout, msg))
		}
		if timeout < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("accessTokenInactivityTimeoutSeconds"), timeout, "value cannot be negative"))
		}
	}

	if len(client.GrantMethod) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("grantMethod"),
			fmt.Sprintf("must be %s or %s", oauthapi.GrantHandlerAuto, oauthapi.GrantHandlerPrompt)))
	} else {
		switch client.GrantMethod {
		case oauthapi.GrantHandlerAuto, oauthapi.GrantHandlerPrompt:
			// valid grant methods
		default:
			allErrs = append(allErrs, field.NotSupported(field.NewPath("grantMethod"), string(client.GrantMethod),
				[]string{string(oauthapi.GrantHandlerAuto), string(oauthapi.GrantHandlerPrompt)},
			))
		}
	}

	return allErrs
}

func ValidateScopeRestriction(restriction oauthapi.ScopeRestriction, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	specifiers := 0
	if len(restriction.ExactValues) > 0 {
		specifiers = specifiers + 1
	}
	if restriction.ClusterRole != nil {
		specifiers = specifiers + 1
	}
	if specifiers != 1 {
		allErrs = append(allErrs, field.Invalid(fldPath, restriction, "exactly one of literals, clusterRole is required"))
		return allErrs
	}

	switch {
	case len(restriction.ExactValues) > 0:
		for i, literal := range restriction.ExactValues {
			if len(literal) == 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("literals").Index(i), literal, "may not be empty"))
			}
		}

	case restriction.ClusterRole != nil:
		if len(restriction.ClusterRole.RoleNames) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("clusterRole", "roleNames"), "won't match anything"))
		}
		if len(restriction.ClusterRole.Namespaces) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("clusterRole", "namespaces"), "won't match anything"))
		}
	}

	return allErrs
}

func ValidateClientUpdate(client *oauthapi.OAuthClient, oldClient *oauthapi.OAuthClient) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateClient(client)...)
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&client.ObjectMeta, &oldClient.ObjectMeta, field.NewPath("metadata"))...)

	return allErrs
}

func ValidateClientAuthorizationName(name string, prefix bool) []string {
	if reasons := path.ValidatePathSegmentName(name, prefix); len(reasons) != 0 {
		return reasons
	}

	lastColon := strings.Index(name, ":")
	if lastColon <= 0 || lastColon >= len(name)-1 {
		return []string{"must be in the format <userName>:<clientName>"}
	}

	return nil
}

func ValidateClientAuthorization(clientAuthorization *oauthapi.OAuthClientAuthorization) field.ErrorList {
	allErrs := field.ErrorList{}

	expectedName := fmt.Sprintf("%s:%s", clientAuthorization.UserName, clientAuthorization.ClientName)

	metadataErrs := validation.ValidateObjectMeta(&clientAuthorization.ObjectMeta, false, ValidateClientAuthorizationName, field.NewPath("metadata"))
	if len(metadataErrs) > 0 {
		allErrs = append(allErrs, metadataErrs...)
	} else if clientAuthorization.Name != expectedName {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "name"), clientAuthorization.Name, "must be in the format <userName>:<clientName>"))
	}

	allErrs = append(allErrs, ValidateClientNameField(clientAuthorization.ClientName, field.NewPath("clientName"))...)
	allErrs = append(allErrs, ValidateUserNameField(clientAuthorization.UserName, field.NewPath("userName"))...)
	allErrs = append(allErrs, ValidateScopes(clientAuthorization.Scopes, field.NewPath("scopes"))...)

	if len(clientAuthorization.UserUID) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("useruid"), ""))
	}

	return allErrs
}

func ValidateClientAuthorizationUpdate(newAuth *oauthapi.OAuthClientAuthorization, oldAuth *oauthapi.OAuthClientAuthorization) field.ErrorList {
	allErrs := ValidateClientAuthorization(newAuth)

	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&newAuth.ObjectMeta, &oldAuth.ObjectMeta, field.NewPath("metadata"))...)

	if oldAuth.ClientName != newAuth.ClientName {
		allErrs = append(allErrs, field.Invalid(field.NewPath("clientName"), newAuth.ClientName, "clientName is not a mutable field"))
	}
	if oldAuth.UserName != newAuth.UserName {
		allErrs = append(allErrs, field.Invalid(field.NewPath("userName"), newAuth.UserName, "userName is not a mutable field"))
	}
	if oldAuth.UserUID != newAuth.UserUID {
		allErrs = append(allErrs, field.Invalid(field.NewPath("userUID"), newAuth.UserUID, "userUID is not a mutable field"))
	}

	return allErrs
}

func ValidateClientNameField(value string, fldPath *field.Path) field.ErrorList {
	if len(value) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	} else if _, saName, err := serviceaccount.SplitUsername(value); err == nil {
		if reasons := validation.ValidateServiceAccountName(saName, false); len(reasons) != 0 {
			return field.ErrorList{field.Invalid(fldPath, value, strings.Join(reasons, ", "))}
		}
	} else if reasons := apimachineryvalidation.NameIsDNSSubdomain(value, false); len(reasons) != 0 {
		return field.ErrorList{field.Invalid(fldPath, value, strings.Join(reasons, ", "))}
	}
	return field.ErrorList{}
}

func ValidateUserNameField(value string, fldPath *field.Path) field.ErrorList {
	if len(value) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}
	// we explicitly allow the bootstrap user in the username field
	// note that we still do not allow the user API objects to have such a name
	if value == bootstrap.BootstrapUser {
		return field.ErrorList{}
	}
	if reasons := uservalidation.ValidateUserName(value, false); len(reasons) != 0 {
		return field.ErrorList{field.Invalid(fldPath, value, strings.Join(reasons, ", "))}
	}
	return field.ErrorList{}
}

func ValidateScopes(scopes []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(scopes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "may not be empty"))
	}

	for i, scope := range scopes {
		illegalCharacter := false
		// https://tools.ietf.org/html/rfc6749#section-3.3 (full list of allowed chars is %x21 / %x23-5B / %x5D-7E)
		// for those without an ascii table, that's `!`, `#-[`, `]-~` inclusive.
		for _, ch := range scope {
			switch {
			case ch == '!':
			case ch >= '#' && ch <= '[':
			case ch >= ']' && ch <= '~':
			default:
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i), scope, fmt.Sprintf("%v not allowed", ch)))
				illegalCharacter = true
			}
		}
		if illegalCharacter {
			continue
		}

		found := false
		for _, evaluator := range authorizerscopes.ScopeEvaluators {
			if !evaluator.Handles(scope) {
				continue
			}

			found = true
			if err := evaluator.Validate(scope); err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i), scope, err.Error()))
				break
			}
		}

		if !found {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), scope, "no scope handler found"))
		}
	}

	return allErrs
}

func ValidateOAuthRedirectReference(sref *oauthapi.OAuthRedirectReference) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&sref.ObjectMeta, true, path.ValidatePathSegmentName, field.NewPath("metadata"))
	return append(allErrs, validateRedirectReference(&sref.Reference, field.NewPath("reference"))...)
}

func validateRedirectReference(ref *oauthapi.RedirectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "may not be empty"))
	} else {
		for _, msg := range path.ValidatePathSegmentName(ref.Name, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), ref.Name, msg))
		}
	}
	switch ref.Kind {
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "may not be empty"))
	case "Route":
		// Valid, TODO add ingress once we support it and update error message
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), ref.Kind, "must be Route"))
	}
	switch ref.Group {
	case "", routev1.GroupName:
	// valid group names
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("group"), ref.Group, []string{"", routev1.GroupName}))
	}
	return allErrs
}
