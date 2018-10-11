package configconversion

import (
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	osinv1 "github.com/openshift/api/osin/v1"
)

func Convert_legacyconfigv1_OAuthConfig_to_osinv1_OAuthConfig(in *legacyconfigv1.OAuthConfig, out *osinv1.OAuthConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	if err := converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames|conversion.IgnoreMissingFields, meta); err != nil {
		return err
	}
	// TODO there is probably a better way to do this
	for i, identityProvider := range out.IdentityProviders {
		var idp runtime.Object
		switch identityProvider.Provider.Object.(type) {
		case *legacyconfigv1.RequestHeaderIdentityProvider:
			idp = &osinv1.RequestHeaderIdentityProvider{}

		case *legacyconfigv1.BasicAuthPasswordIdentityProvider:
			idp = &osinv1.BasicAuthPasswordIdentityProvider{}

		case *legacyconfigv1.AllowAllPasswordIdentityProvider:
			idp = &osinv1.AllowAllPasswordIdentityProvider{}

		case *legacyconfigv1.DenyAllPasswordIdentityProvider:
			idp = &osinv1.DenyAllPasswordIdentityProvider{}

		case *legacyconfigv1.HTPasswdPasswordIdentityProvider:
			idp = &osinv1.HTPasswdPasswordIdentityProvider{}

		case *legacyconfigv1.LDAPPasswordIdentityProvider:
			idp = &osinv1.LDAPPasswordIdentityProvider{}

		case *legacyconfigv1.KeystonePasswordIdentityProvider:
			idp = &osinv1.KeystonePasswordIdentityProvider{}

		case *legacyconfigv1.OpenIDIdentityProvider:
			idp = &osinv1.OpenIDIdentityProvider{}

		case *legacyconfigv1.GitHubIdentityProvider:
			idp = &osinv1.GitHubIdentityProvider{}

		case *legacyconfigv1.GitLabIdentityProvider:
			idp = &osinv1.GitLabIdentityProvider{}

		case *legacyconfigv1.GoogleIdentityProvider:
			idp = &osinv1.GoogleIdentityProvider{}

		default:
			return fmt.Errorf("unknown IDP: %#v", identityProvider.Provider.Object)
		}
		if err := json.Unmarshal(identityProvider.Provider.Raw, idp); err != nil {
			return err
		}
		out.IdentityProviders[i].Provider.Object = idp
	}
	return nil
}
