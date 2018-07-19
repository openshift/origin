package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/openshift/api/oauth/v1"
)

func addFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(v1.GroupVersion.String(), "OAuthAccessToken", oauthAccessTokenFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(v1.GroupVersion.String(), "OAuthAuthorizeToken", oauthAuthorizeTokenFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	if err := scheme.AddFieldLabelConversionFunc(v1.GroupVersion.String(), "OAuthClientAuthorization", oauthClientAuthorizationFieldSelectorKeyConversionFunc); err != nil {
		return err
	}
	return nil
}

// because field selectors can vary in support by version they are exposed under, we have one function for each
// groupVersion we're registering for

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
