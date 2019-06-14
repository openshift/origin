package main

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	appsv1 "github.com/openshift/api/apps/v1"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	networkv1 "github.com/openshift/api/network/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	projectv1 "github.com/openshift/api/project/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	securityv1 "github.com/openshift/api/security/v1"
	templatev1 "github.com/openshift/api/template/v1"
	userv1 "github.com/openshift/api/user/v1"
)

func InstallExternalLegacyAll(scheme *runtime.Scheme) {
	utilruntime.Must(appsv1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(authorizationv1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(buildv1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(imagev1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(networkv1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(oauthv1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(projectv1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(quotav1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(securityv1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(templatev1.DeprecatedInstallWithoutGroup(scheme))
	utilruntime.Must(userv1.DeprecatedInstallWithoutGroup(scheme))
}
