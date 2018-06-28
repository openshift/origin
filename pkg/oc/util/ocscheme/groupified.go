package ocscheme

import (
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "github.com/openshift/api/apps/v1"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	networkv1 "github.com/openshift/api/network/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	projectv1 "github.com/openshift/api/project/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	templatev1 "github.com/openshift/api/template/v1"
	userv1 "github.com/openshift/api/user/v1"
)

func AddOpenShiftExternalToScheme(scheme *runtime.Scheme) error {
	if err := appsv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := authorizationv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := buildv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := imagev1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := networkv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := oauthv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := projectv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := quotav1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := routev1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := securityv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := templatev1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := userv1.AddToScheme(scheme); err != nil {
		return err
	}

	return nil
}
