package validation

import (
	appsvalidation "github.com/openshift/origin/pkg/apps/apis/apps/validation"
	authorizationvalidation "github.com/openshift/origin/pkg/authorization/apis/authorization/validation"
	buildvalidation "github.com/openshift/origin/pkg/build/apis/build/validation"
	imagevalidation "github.com/openshift/origin/pkg/image/apis/image/validation"
	sdnvalidation "github.com/openshift/origin/pkg/network/apis/network/validation"
	oauthvalidation "github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
	projectvalidation "github.com/openshift/origin/pkg/project/apis/project/validation"
	quotavalidation "github.com/openshift/origin/pkg/quota/apis/quota/validation"
	routevalidation "github.com/openshift/origin/pkg/route/apis/route/validation"
	securityvalidation "github.com/openshift/origin/pkg/security/apis/security/validation"
	templatevalidation "github.com/openshift/origin/pkg/template/apis/template/validation"
	uservalidation "github.com/openshift/origin/pkg/user/apis/user/validation"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	userapi "github.com/openshift/origin/pkg/user/apis/user"

	// required to be loaded before we register
	_ "github.com/openshift/origin/pkg/api/install"
)

func init() {
	registerAll()
}

func registerAll() {
	Validator.MustRegister(&authorizationapi.SelfSubjectRulesReview{}, true, authorizationvalidation.ValidateSelfSubjectRulesReview, nil)
	Validator.MustRegister(&authorizationapi.SubjectAccessReview{}, false, authorizationvalidation.ValidateSubjectAccessReview, nil)
	Validator.MustRegister(&authorizationapi.SubjectRulesReview{}, true, authorizationvalidation.ValidateSubjectRulesReview, nil)
	Validator.MustRegister(&authorizationapi.ResourceAccessReview{}, false, authorizationvalidation.ValidateResourceAccessReview, nil)
	Validator.MustRegister(&authorizationapi.LocalSubjectAccessReview{}, true, authorizationvalidation.ValidateLocalSubjectAccessReview, nil)
	Validator.MustRegister(&authorizationapi.LocalResourceAccessReview{}, true, authorizationvalidation.ValidateLocalResourceAccessReview, nil)

	Validator.MustRegister(&authorizationapi.Role{}, true, authorizationvalidation.ValidateLocalRole, authorizationvalidation.ValidateLocalRoleUpdate)
	Validator.MustRegister(&authorizationapi.RoleBinding{}, true, authorizationvalidation.ValidateLocalRoleBinding, authorizationvalidation.ValidateLocalRoleBindingUpdate)
	Validator.MustRegister(&authorizationapi.RoleBindingRestriction{}, true, authorizationvalidation.ValidateRoleBindingRestriction, authorizationvalidation.ValidateRoleBindingRestrictionUpdate)
	Validator.MustRegister(&authorizationapi.ClusterRole{}, false, authorizationvalidation.ValidateClusterRole, authorizationvalidation.ValidateClusterRoleUpdate)
	Validator.MustRegister(&authorizationapi.ClusterRoleBinding{}, false, authorizationvalidation.ValidateClusterRoleBinding, authorizationvalidation.ValidateClusterRoleBindingUpdate)

	Validator.MustRegister(&buildapi.Build{}, true, buildvalidation.ValidateBuild, buildvalidation.ValidateBuildUpdate)
	Validator.MustRegister(&buildapi.BuildConfig{}, true, buildvalidation.ValidateBuildConfig, buildvalidation.ValidateBuildConfigUpdate)
	Validator.MustRegister(&buildapi.BuildRequest{}, true, buildvalidation.ValidateBuildRequest, nil)
	Validator.MustRegister(&buildapi.BuildLogOptions{}, true, buildvalidation.ValidateBuildLogOptions, nil)

	Validator.MustRegister(&appsapi.DeploymentConfig{}, true, appsvalidation.ValidateDeploymentConfig, appsvalidation.ValidateDeploymentConfigUpdate)
	Validator.MustRegister(&appsapi.DeploymentConfigRollback{}, true, appsvalidation.ValidateDeploymentConfigRollback, nil)
	Validator.MustRegister(&appsapi.DeploymentLogOptions{}, true, appsvalidation.ValidateDeploymentLogOptions, nil)
	Validator.MustRegister(&appsapi.DeploymentRequest{}, true, appsvalidation.ValidateDeploymentRequest, nil)

	Validator.MustRegister(&imageapi.Image{}, false, imagevalidation.ValidateImage, imagevalidation.ValidateImageUpdate)
	Validator.MustRegister(&imageapi.ImageSignature{}, false, imagevalidation.ValidateImageSignature, imagevalidation.ValidateImageSignatureUpdate)
	Validator.MustRegister(&imageapi.ImageStream{}, true, imagevalidation.ValidateImageStream, imagevalidation.ValidateImageStreamUpdate)
	Validator.MustRegister(&imageapi.ImageStreamImport{}, true, imagevalidation.ValidateImageStreamImport, nil)
	Validator.MustRegister(&imageapi.ImageStreamMapping{}, true, imagevalidation.ValidateImageStreamMapping, nil)
	Validator.MustRegister(&imageapi.ImageStreamTag{}, true, imagevalidation.ValidateImageStreamTag, imagevalidation.ValidateImageStreamTagUpdate)

	Validator.MustRegister(&oauthapi.OAuthAccessToken{}, false, oauthvalidation.ValidateAccessToken, oauthvalidation.ValidateAccessTokenUpdate)
	Validator.MustRegister(&oauthapi.OAuthAuthorizeToken{}, false, oauthvalidation.ValidateAuthorizeToken, oauthvalidation.ValidateAuthorizeTokenUpdate)
	Validator.MustRegister(&oauthapi.OAuthClient{}, false, oauthvalidation.ValidateClient, oauthvalidation.ValidateClientUpdate)
	Validator.MustRegister(&oauthapi.OAuthClientAuthorization{}, false, oauthvalidation.ValidateClientAuthorization, oauthvalidation.ValidateClientAuthorizationUpdate)
	Validator.MustRegister(&oauthapi.OAuthRedirectReference{}, true, oauthvalidation.ValidateOAuthRedirectReference, nil)

	Validator.MustRegister(&projectapi.Project{}, false, projectvalidation.ValidateProject, projectvalidation.ValidateProjectUpdate)
	Validator.MustRegister(&projectapi.ProjectRequest{}, false, projectvalidation.ValidateProjectRequest, nil)

	Validator.MustRegister(&routeapi.Route{}, true, routevalidation.ValidateRoute, routevalidation.ValidateRouteUpdate)

	Validator.MustRegister(&networkapi.ClusterNetwork{}, false, sdnvalidation.ValidateClusterNetwork, sdnvalidation.ValidateClusterNetworkUpdate)
	Validator.MustRegister(&networkapi.HostSubnet{}, false, sdnvalidation.ValidateHostSubnet, sdnvalidation.ValidateHostSubnetUpdate)
	Validator.MustRegister(&networkapi.NetNamespace{}, false, sdnvalidation.ValidateNetNamespace, sdnvalidation.ValidateNetNamespaceUpdate)
	Validator.MustRegister(&networkapi.EgressNetworkPolicy{}, true, sdnvalidation.ValidateEgressNetworkPolicy, sdnvalidation.ValidateEgressNetworkPolicyUpdate)

	Validator.MustRegister(&templateapi.Template{}, true, templatevalidation.ValidateTemplate, templatevalidation.ValidateTemplateUpdate)
	Validator.MustRegister(&templateapi.TemplateInstance{}, true, templatevalidation.ValidateTemplateInstance, templatevalidation.ValidateTemplateInstanceUpdate)
	Validator.MustRegister(&templateapi.BrokerTemplateInstance{}, false, templatevalidation.ValidateBrokerTemplateInstance, templatevalidation.ValidateBrokerTemplateInstanceUpdate)

	Validator.MustRegister(&userapi.User{}, false, uservalidation.ValidateUser, uservalidation.ValidateUserUpdate)
	Validator.MustRegister(&userapi.Identity{}, false, uservalidation.ValidateIdentity, uservalidation.ValidateIdentityUpdate)
	Validator.MustRegister(&userapi.UserIdentityMapping{}, false, uservalidation.ValidateUserIdentityMapping, uservalidation.ValidateUserIdentityMappingUpdate)
	Validator.MustRegister(&userapi.Group{}, false, uservalidation.ValidateGroup, uservalidation.ValidateGroupUpdate)

	Validator.MustRegister(&securityapi.SecurityContextConstraints{}, false, securityvalidation.ValidateSecurityContextConstraints, securityvalidation.ValidateSecurityContextConstraintsUpdate)
	Validator.MustRegister(&securityapi.PodSecurityPolicySubjectReview{}, true, securityvalidation.ValidatePodSecurityPolicySubjectReview, nil)
	Validator.MustRegister(&securityapi.PodSecurityPolicySelfSubjectReview{}, true, securityvalidation.ValidatePodSecurityPolicySelfSubjectReview, nil)
	Validator.MustRegister(&securityapi.PodSecurityPolicyReview{}, true, securityvalidation.ValidatePodSecurityPolicyReview, nil)

	Validator.MustRegister(&quotaapi.ClusterResourceQuota{}, false, quotavalidation.ValidateClusterResourceQuota, quotavalidation.ValidateClusterResourceQuotaUpdate)
}
