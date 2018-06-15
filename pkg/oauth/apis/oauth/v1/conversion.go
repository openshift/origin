package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	return nil
}

func addLegacyFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(LegacySchemeGroupVersion.String(), "OAuthAccessToken", legacyOAuthAccessTokenFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(LegacySchemeGroupVersion.String(), "OAuthAuthorizeToken", legacyOAuthAuthorizeTokenFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(LegacySchemeGroupVersion.String(), "OAuthClientAuthorization", legacyOAuthClientAuthorizationFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	return nil
}

func addFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "OAuthAccessToken", oauthAccessTokenFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "OAuthAuthorizeToken", oauthAuthorizeTokenFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "OAuthClientAuthorization", oauthClientAuthorizationFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	return nil
}

// because field selectors can vary in support by version they are exposed under, we have one function for each
// groupVersion we're registering for

func legacyOAuthAccessTokenFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "clientName",
		"userName",
		"userUID",
		"authorizeToken":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}

func oauthAccessTokenFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "clientName",
		"userName",
		"userUID",
		"authorizeToken":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}

func legacyOAuthAuthorizeTokenFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "clientName",
		"userName",
		"userUID":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}

func oauthAuthorizeTokenFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "clientName",
		"userName",
		"userUID":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}

func legacyOAuthClientAuthorizationFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "clientName",
		"userName",
		"userUID":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}

func oauthClientAuthorizationFieldSelectorKeyConversionFunc(label, value string) (internalLabel, internalValue string, err error) {
	switch label {
	case "clientName",
		"userName",
		"userUID":
		return label, value, nil
	default:
		return runtime.DefaultMetaV1FieldSelectorConversion(label, value)
	}
}
