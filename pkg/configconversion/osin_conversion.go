package configconversion

import (
	"reflect"

	"k8s.io/apimachinery/pkg/conversion"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	osinv1 "github.com/openshift/api/osin/v1"
)

func Convert_legacyconfigv1_OAuthConfig_to_osinv1_OAuthConfig(in *legacyconfigv1.OAuthConfig, out *osinv1.OAuthConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}

func Convert_osinv1_OAuthConfig_to_legacyconfigv1_OAuthConfig(in *osinv1.OAuthConfig, out *legacyconfigv1.OAuthConfig, s conversion.Scope) error {
	converter := conversion.NewConverter(conversion.DefaultNameFunc)
	_, meta := converter.DefaultMeta(reflect.TypeOf(in))
	return converter.DefaultConvert(in, out, conversion.AllowDifferentFieldTypeNames, meta)
}
