package oauth

import (
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	configv1 "github.com/openshift/api/config/v1"
	crvalidation "github.com/openshift/origin/pkg/admission/customresourcevalidation"
	oauthvalidation "github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
	userapivalidation "github.com/openshift/origin/pkg/user/apis/user/validation"
)

const PluginName = "config.openshift.io/ValidateOAuth"

var validMappingMethods = sets.NewString(
	string(configv1.MappingMethodLookup),
	string(configv1.MappingMethodClaim),
	string(configv1.MappingMethodAdd),
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return crvalidation.NewValidator(
			map[schema.GroupResource]bool{
				configv1.GroupVersion.WithResource("oauths").GroupResource(): true,
			},
			map[schema.GroupVersionKind]crvalidation.ObjectValidator{
				configv1.GroupVersion.WithKind("OAuth"): oauthV1{},
			})
	})
}

func toOAuthV1(uncastObj runtime.Object) (*configv1.OAuth, field.ErrorList) {
	if uncastObj == nil {
		return nil, nil
	}

	errs := field.ErrorList{}

	obj, ok := uncastObj.(*configv1.OAuth)
	if !ok {
		return nil, append(errs,
			field.NotSupported(field.NewPath("kind"), fmt.Sprintf("%T", uncastObj), []string{"OAuth"}),
			field.NotSupported(field.NewPath("apiVersion"), fmt.Sprintf("%T", uncastObj), []string{"config.openshift.io/v1"}))
	}

	return obj, nil
}

type oauthV1 struct{}

func (oauthV1) ValidateCreate(uncastObj runtime.Object) field.ErrorList {
	obj, errs := toOAuthV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}

	errs = append(errs, validation.ValidateObjectMeta(&obj.ObjectMeta, false, crvalidation.RequireNameCluster, field.NewPath("metadata"))...)
	errs = append(errs, validateOAuthSpecCreate(obj.Spec)...)

	return errs
}

func (oauthV1) ValidateUpdate(uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, errs := toOAuthV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}
	oldObj, errs := toOAuthV1(uncastOldObj)
	if len(errs) > 0 {
		return errs
	}

	errs = append(errs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)
	errs = append(errs, validateOAuthSpecUpdate(obj.Spec, oldObj.Spec)...)

	return errs
}

func (oauthV1) ValidateStatusUpdate(uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, errs := toOAuthV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}
	oldObj, errs := toOAuthV1(uncastOldObj)
	if len(errs) > 0 {
		return errs
	}

	// TODO validate the obj.  remember that status validation should *never* fail on spec validation errors.
	errs = append(errs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)
	errs = append(errs, validateOAuthStatus(obj.Status)...)

	return errs
}

func validateOAuthSpecCreate(spec configv1.OAuthSpec) field.ErrorList {
	return validateOAuthSpec(spec)
}

func validateOAuthSpecUpdate(newspec, oldspec configv1.OAuthSpec) field.ErrorList {
	return validateOAuthSpec(newspec)
}

func validateOAuthSpec(spec configv1.OAuthSpec) field.ErrorList {
	errs := field.ErrorList{}
	specPath := field.NewPath("spec")

	providerNames := sets.NewString()

	challengeIssuingIdentityProviders := []string{}
	challengeRedirectingIdentityProviders := []string{}

	for i, identityProvider := range spec.IdentityProviders {

		if isUsedAsChallenger(identityProvider.IdentityProviderConfig) {
			// RequestHeaderIdentityProvider is special, it can only react to challenge clients by redirecting them
			// Make sure we don't have more than a single redirector, and don't have a mix of challenge issuers and redirectors
			if identityProvider.Type == configv1.IdentityProviderTypeRequestHeader {
				challengeRedirectingIdentityProviders = append(challengeRedirectingIdentityProviders, identityProvider.Name)
			} else {
				challengeIssuingIdentityProviders = append(challengeIssuingIdentityProviders, identityProvider.Name)
			}
		}

		identityProviderPath := specPath.Child("identityProvider").Index(i)
		errs = append(errs, ValidateIdentityProvider(identityProvider, identityProviderPath)...)

		if len(identityProvider.Name) > 0 {
			if providerNames.Has(identityProvider.Name) {
				errs = append(errs, field.Invalid(identityProviderPath.Child("name"), identityProvider.Name, "must have a unique name"))
			}
			providerNames.Insert(identityProvider.Name)
		}
	}

	if len(challengeRedirectingIdentityProviders) > 1 {
		errs = append(errs, field.Forbidden(specPath.Child("identityProviders"), fmt.Sprintf("only one identity provider can redirect clients requesting an authentication challenge, found: %v", strings.Join(challengeRedirectingIdentityProviders, ", "))))
	}
	if len(challengeRedirectingIdentityProviders) > 0 && len(challengeIssuingIdentityProviders) > 0 {
		errs = append(errs, field.Forbidden(specPath.Child("identityProviders"), fmt.Sprintf(
			"cannot mix providers that redirect clients requesting auth challenges (%s) with providers issuing challenges to those clients (%s)",
			strings.Join(challengeRedirectingIdentityProviders, ", "),
			strings.Join(challengeIssuingIdentityProviders, ", "),
		)))
	}

	timeout := spec.TokenConfig.AccessTokenInactivityTimeoutSeconds
	if timeout > 0 && timeout < oauthvalidation.MinimumInactivityTimeoutSeconds {
		errs = append(errs, field.Invalid(
			specPath.Child("tokenConfig", "accessTokenInactivityTimeoutSeconds"), timeout,
			fmt.Sprintf("the minimum acceptable token timeout value is %d seconds",
				oauthvalidation.MinimumInactivityTimeoutSeconds)))
	}

	emptyTemplates := configv1.OAuthTemplates{}
	if spec.Templates != emptyTemplates {
		errs = append(errs, crvalidation.ValidateSecretReference(specPath.Child("templates", "login"), spec.Templates.Login, false)...)
		errs = append(errs, crvalidation.ValidateSecretReference(specPath.Child("templates", "providerSelection"), spec.Templates.ProviderSelection, false)...)
		errs = append(errs, crvalidation.ValidateSecretReference(specPath.Child("templates", "error"), spec.Templates.Error, false)...)
	}

	return errs
}

func validateOAuthStatus(status configv1.OAuthStatus) field.ErrorList {
	errs := field.ErrorList{}

	// TODO

	return errs
}

func ValidateIdentityProvider(identityProvider configv1.IdentityProvider, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(identityProvider.Name) == 0 {
		errs = append(errs, field.Required(fldPath.Child("name"), ""))
	} else if reasons := userapivalidation.ValidateIdentityProviderName(identityProvider.Name); len(reasons) != 0 {
		errs = append(errs, field.Invalid(fldPath.Child("name"), identityProvider.Name, strings.Join(reasons, ", ")))
	}

	if len(identityProvider.MappingMethod) > 0 && !validMappingMethods.Has(string(identityProvider.MappingMethod)) {
		errs = append(errs, field.NotSupported(fldPath.Child("mappingMethod"), identityProvider.MappingMethod, validMappingMethods.List()))
	}

	provider := identityProvider.IdentityProviderConfig
	switch provider.Type {
	case "":
		errs = append(errs, field.Required(fldPath.Child("type"), ""))
	case configv1.IdentityProviderTypeRequestHeader:
		errs = append(errs, ValidateRequestHeaderIdentityProvider(provider.RequestHeader, fldPath)...)

	case configv1.IdentityProviderTypeBasicAuth:
		if provider.BasicAuth == nil {
			errs = append(errs, field.Required(fldPath.Child("basicAuth"), ""))
		} else {
			errs = append(errs, ValidateRemoteConnectionInfo(provider.BasicAuth.OAuthRemoteConnectionInfo, fldPath.Child("basicauth"))...)
		}

	case configv1.IdentityProviderTypeHTPasswd:
		if provider.HTPasswd == nil {
			errs = append(errs, field.Required(fldPath.Child("htpasswd"), ""))
		} else {
			errs = append(errs, crvalidation.ValidateSecretReference(fldPath.Child("htpasswd", "filedata"), provider.HTPasswd.FileData, true)...)
		}

	case configv1.IdentityProviderTypeLDAP:
		errs = append(errs, ValidateLDAPIdentityProvider(provider.LDAP, fldPath.Child("ldap"))...)

	case configv1.IdentityProviderTypeKeystone:
		errs = append(errs, ValidateKeystoneIdentityProvider(provider.Keystone, fldPath.Child("keystone"))...)

	case configv1.IdentityProviderTypeGitHub:
		errs = append(errs, ValidateGitHubIdentityProvider(provider.GitHub, identityProvider.MappingMethod, fldPath.Child("github"))...)

	case configv1.IdentityProviderTypeGitLab:
		errs = append(errs, ValidateGitLabIdentityProvider(provider.GitLab, fldPath.Child("gitlab"))...)

	case configv1.IdentityProviderTypeGoogle:
		errs = append(errs, ValidateGoogleIdentityProvider(provider.Google, identityProvider.MappingMethod, fldPath.Child("google"))...)

	case configv1.IdentityProviderTypeOpenID:
		errs = append(errs, ValidateOpenIDIdentityProvider(provider.OpenID, fldPath.Child("openID"))...)

	default:
		errs = append(errs, field.Invalid(fldPath.Child("type"), identityProvider.Type, "not a valid provider type"))
	}

	return errs
}

func ValidateOAuthIdentityProvider(clientID string, clientSecretRef configv1.SecretNameReference, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(clientID) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath.Child("clientID"), ""))
	}

	allErrs = append(allErrs, crvalidation.ValidateSecretReference(fieldPath.Child("clientSecret"), clientSecretRef, true)...)

	return allErrs
}

func isUsedAsChallenger(idp configv1.IdentityProviderConfig) bool {
	switch idp.Type {
	// whitelist all the IdPs that we set `UseAsChallenger: true` in cluster-authentication-operator
	case configv1.IdentityProviderTypeBasicAuth, configv1.IdentityProviderTypeGitLab,
		configv1.IdentityProviderTypeHTPasswd, configv1.IdentityProviderTypeKeystone,
		configv1.IdentityProviderTypeLDAP:
		return true
	case configv1.IdentityProviderTypeRequestHeader:
		if idp.RequestHeader == nil {
			// this is an error reported elsewhere
			return false
		}
		return len(idp.RequestHeader.ChallengeURL) > 0
	default:
		return false
	}
}
