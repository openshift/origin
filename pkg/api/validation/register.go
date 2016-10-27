package validation

import (
	authorizationvalidation "github.com/openshift/origin/pkg/authorization/api/validation"
	buildvalidation "github.com/openshift/origin/pkg/build/api/validation"
	deployvalidation "github.com/openshift/origin/pkg/deploy/api/validation"
	imagevalidation "github.com/openshift/origin/pkg/image/api/validation"
	oauthvalidation "github.com/openshift/origin/pkg/oauth/api/validation"
	projectvalidation "github.com/openshift/origin/pkg/project/api/validation"
	quotavalidation "github.com/openshift/origin/pkg/quota/api/validation"
	routevalidation "github.com/openshift/origin/pkg/route/api/validation"
	sdnvalidation "github.com/openshift/origin/pkg/sdn/api/validation"
	securityvalidation "github.com/openshift/origin/pkg/security/api/validation"
	templatevalidation "github.com/openshift/origin/pkg/template/api/validation"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
	extvalidation "k8s.io/kubernetes/pkg/apis/extensions/validation"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	securityapi "github.com/openshift/origin/pkg/security/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	"k8s.io/kubernetes/pkg/apis/extensions"

	// required to be loaded before we register
	_ "github.com/openshift/origin/pkg/api/install"
)

func init() {
	registerAll()
}

func registerAll() {
	Validator.MustRegister(&authorizationapi.SelfSubjectRulesReview{}, authorizationvalidation.ValidateSelfSubjectRulesReview, nil)
	Validator.MustRegister(&authorizationapi.SubjectAccessReview{}, authorizationvalidation.ValidateSubjectAccessReview, nil)
	Validator.MustRegister(&authorizationapi.SubjectRulesReview{}, authorizationvalidation.ValidateSubjectRulesReview, nil)
	Validator.MustRegister(&authorizationapi.ResourceAccessReview{}, authorizationvalidation.ValidateResourceAccessReview, nil)
	Validator.MustRegister(&authorizationapi.LocalSubjectAccessReview{}, authorizationvalidation.ValidateLocalSubjectAccessReview, nil)
	Validator.MustRegister(&authorizationapi.LocalResourceAccessReview{}, authorizationvalidation.ValidateLocalResourceAccessReview, nil)

	Validator.MustRegister(&authorizationapi.Policy{}, authorizationvalidation.ValidateLocalPolicy, authorizationvalidation.ValidateLocalPolicyUpdate)
	Validator.MustRegister(&authorizationapi.PolicyBinding{}, authorizationvalidation.ValidateLocalPolicyBinding, authorizationvalidation.ValidateLocalPolicyBindingUpdate)
	Validator.MustRegister(&authorizationapi.Role{}, authorizationvalidation.ValidateLocalRole, authorizationvalidation.ValidateLocalRoleUpdate)
	Validator.MustRegister(&authorizationapi.RoleBinding{}, authorizationvalidation.ValidateLocalRoleBinding, authorizationvalidation.ValidateLocalRoleBindingUpdate)

	Validator.MustRegister(&authorizationapi.ClusterPolicy{}, authorizationvalidation.ValidateClusterPolicy, authorizationvalidation.ValidateClusterPolicyUpdate)
	Validator.MustRegister(&authorizationapi.ClusterPolicyBinding{}, authorizationvalidation.ValidateClusterPolicyBinding, authorizationvalidation.ValidateClusterPolicyBindingUpdate)
	Validator.MustRegister(&authorizationapi.ClusterRole{}, authorizationvalidation.ValidateClusterRole, authorizationvalidation.ValidateClusterRoleUpdate)
	Validator.MustRegister(&authorizationapi.ClusterRoleBinding{}, authorizationvalidation.ValidateClusterRoleBinding, authorizationvalidation.ValidateClusterRoleBindingUpdate)

	Validator.MustRegister(&buildapi.Build{}, buildvalidation.ValidateBuild, buildvalidation.ValidateBuildUpdate)
	Validator.MustRegister(&buildapi.BuildConfig{}, buildvalidation.ValidateBuildConfig, buildvalidation.ValidateBuildConfigUpdate)
	Validator.MustRegister(&buildapi.BuildRequest{}, buildvalidation.ValidateBuildRequest, nil)
	Validator.MustRegister(&buildapi.BuildLogOptions{}, buildvalidation.ValidateBuildLogOptions, nil)

	Validator.MustRegister(&deployapi.DeploymentConfig{}, deployvalidation.ValidateDeploymentConfig, deployvalidation.ValidateDeploymentConfigUpdate)
	Validator.MustRegister(&deployapi.DeploymentConfigRollback{}, deployvalidation.ValidateDeploymentConfigRollback, nil)
	Validator.MustRegister(&deployapi.DeploymentLogOptions{}, deployvalidation.ValidateDeploymentLogOptions, nil)
	Validator.MustRegister(&deployapi.DeploymentRequest{}, deployvalidation.ValidateDeploymentRequest, nil)
	Validator.MustRegister(&extensions.Scale{}, extvalidation.ValidateScale, nil)

	Validator.MustRegister(&imageapi.Image{}, imagevalidation.ValidateImage, imagevalidation.ValidateImageUpdate)
	Validator.MustRegister(&imageapi.ImageSignature{}, imagevalidation.ValidateImageSignature, imagevalidation.ValidateImageSignatureUpdate)
	Validator.MustRegister(&imageapi.ImageStream{}, imagevalidation.ValidateImageStream, imagevalidation.ValidateImageStreamUpdate)
	Validator.MustRegister(&imageapi.ImageStreamImport{}, imagevalidation.ValidateImageStreamImport, nil)
	Validator.MustRegister(&imageapi.ImageStreamMapping{}, imagevalidation.ValidateImageStreamMapping, nil)
	Validator.MustRegister(&imageapi.ImageStreamTag{}, imagevalidation.ValidateImageStreamTag, imagevalidation.ValidateImageStreamTagUpdate)

	Validator.MustRegister(&oauthapi.OAuthAccessToken{}, oauthvalidation.ValidateAccessToken, oauthvalidation.ValidateAccessTokenUpdate)
	Validator.MustRegister(&oauthapi.OAuthAuthorizeToken{}, oauthvalidation.ValidateAuthorizeToken, oauthvalidation.ValidateAuthorizeTokenUpdate)
	Validator.MustRegister(&oauthapi.OAuthClient{}, oauthvalidation.ValidateClient, oauthvalidation.ValidateClientUpdate)
	Validator.MustRegister(&oauthapi.OAuthClientAuthorization{}, oauthvalidation.ValidateClientAuthorization, oauthvalidation.ValidateClientAuthorizationUpdate)
	Validator.MustRegister(&oauthapi.OAuthRedirectReference{}, oauthvalidation.ValidateOAuthRedirectReference, nil)

	Validator.MustRegister(&projectapi.Project{}, projectvalidation.ValidateProject, projectvalidation.ValidateProjectUpdate)
	Validator.MustRegister(&projectapi.ProjectRequest{}, projectvalidation.ValidateProjectRequest, nil)

	Validator.MustRegister(&routeapi.Route{}, routevalidation.ValidateRoute, routevalidation.ValidateRouteUpdate)

	Validator.MustRegister(&sdnapi.ClusterNetwork{}, sdnvalidation.ValidateClusterNetwork, sdnvalidation.ValidateClusterNetworkUpdate)
	Validator.MustRegister(&sdnapi.HostSubnet{}, sdnvalidation.ValidateHostSubnet, sdnvalidation.ValidateHostSubnetUpdate)
	Validator.MustRegister(&sdnapi.NetNamespace{}, sdnvalidation.ValidateNetNamespace, sdnvalidation.ValidateNetNamespaceUpdate)
	Validator.MustRegister(&sdnapi.EgressNetworkPolicy{}, sdnvalidation.ValidateEgressNetworkPolicy, sdnvalidation.ValidateEgressNetworkPolicyUpdate)

	Validator.MustRegister(&templateapi.Template{}, templatevalidation.ValidateTemplate, templatevalidation.ValidateTemplateUpdate)

	Validator.MustRegister(&userapi.User{}, uservalidation.ValidateUser, uservalidation.ValidateUserUpdate)
	Validator.MustRegister(&userapi.Identity{}, uservalidation.ValidateIdentity, uservalidation.ValidateIdentityUpdate)
	Validator.MustRegister(&userapi.UserIdentityMapping{}, uservalidation.ValidateUserIdentityMapping, uservalidation.ValidateUserIdentityMappingUpdate)
	Validator.MustRegister(&userapi.Group{}, uservalidation.ValidateGroup, uservalidation.ValidateGroupUpdate)

	Validator.MustRegister(&securityapi.PodSecurityPolicySubjectReview{}, securityvalidation.ValidatePodSecurityPolicySubjectReview, nil)
	Validator.MustRegister(&securityapi.PodSecurityPolicySelfSubjectReview{}, securityvalidation.ValidatePodSecurityPolicySelfSubjectReview, nil)
	Validator.MustRegister(&securityapi.PodSecurityPolicyReview{}, securityvalidation.ValidatePodSecurityPolicyReview, nil)

	Validator.MustRegister(&quotaapi.ClusterResourceQuota{}, quotavalidation.ValidateClusterResourceQuota, quotavalidation.ValidateClusterResourceQuotaUpdate)
}
