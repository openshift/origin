package validation

import (
	authorizationvalidation "github.com/openshift/origin/pkg/authorization/api/validation"
	buildvalidation "github.com/openshift/origin/pkg/build/api/validation"
	deployvalidation "github.com/openshift/origin/pkg/deploy/api/validation"
	imagevalidation "github.com/openshift/origin/pkg/image/api/validation"
	oauthvalidation "github.com/openshift/origin/pkg/oauth/api/validation"
	projectvalidation "github.com/openshift/origin/pkg/project/api/validation"
	routevalidation "github.com/openshift/origin/pkg/route/api/validation"
	sdnvalidation "github.com/openshift/origin/pkg/sdn/api/validation"
	templatevalidation "github.com/openshift/origin/pkg/template/api/validation"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

func init() {
	Validator.Register(&authorizationapi.Policy{}, authorizationvalidation.ValidateLocalPolicy, authorizationvalidation.ValidateLocalPolicyUpdate)
	Validator.Register(&authorizationapi.PolicyBinding{}, authorizationvalidation.ValidateLocalPolicyBinding, authorizationvalidation.ValidateLocalPolicyBindingUpdate)
	Validator.Register(&authorizationapi.Role{}, authorizationvalidation.ValidateLocalRole, authorizationvalidation.ValidateLocalRoleUpdate)
	Validator.Register(&authorizationapi.RoleBinding{}, authorizationvalidation.ValidateLocalRoleBinding, authorizationvalidation.ValidateLocalRoleBindingUpdate)

	Validator.Register(&authorizationapi.ClusterPolicy{}, authorizationvalidation.ValidateClusterPolicy, authorizationvalidation.ValidateClusterPolicyUpdate)
	Validator.Register(&authorizationapi.ClusterPolicyBinding{}, authorizationvalidation.ValidateClusterPolicyBinding, authorizationvalidation.ValidateClusterPolicyBindingUpdate)
	Validator.Register(&authorizationapi.ClusterRole{}, authorizationvalidation.ValidateClusterRole, authorizationvalidation.ValidateClusterRoleUpdate)
	Validator.Register(&authorizationapi.ClusterRoleBinding{}, authorizationvalidation.ValidateClusterRoleBinding, authorizationvalidation.ValidateClusterRoleBindingUpdate)

	Validator.Register(&buildapi.Build{}, buildvalidation.ValidateBuild, nil)
	Validator.Register(&buildapi.BuildConfig{}, buildvalidation.ValidateBuildConfig, nil)

	Validator.Register(&deployapi.DeploymentConfig{}, deployvalidation.ValidateDeploymentConfig, deployvalidation.ValidateDeploymentConfigUpdate)
	Validator.Register(&deployapi.DeploymentConfigRollback{}, deployvalidation.ValidateDeploymentConfigRollback, nil)

	Validator.Register(&imageapi.Image{}, imagevalidation.ValidateImage, nil)
	Validator.Register(&imageapi.ImageStream{}, imagevalidation.ValidateImageStream, imagevalidation.ValidateImageStreamUpdate)

	Validator.Register(&oauthapi.OAuthAccessToken{}, oauthvalidation.ValidateAccessToken, nil)
	Validator.Register(&oauthapi.OAuthAuthorizeToken{}, oauthvalidation.ValidateAuthorizeToken, nil)
	Validator.Register(&oauthapi.OAuthClient{}, oauthvalidation.ValidateClient, oauthvalidation.ValidateClientUpdate)
	Validator.Register(&oauthapi.OAuthClientAuthorization{}, oauthvalidation.ValidateClientAuthorization, oauthvalidation.ValidateClientAuthorizationUpdate)

	Validator.Register(&projectapi.Project{}, projectvalidation.ValidateProject, projectvalidation.ValidateProjectUpdate)
	Validator.Register(&projectapi.ProjectRequest{}, projectvalidation.ValidateProjectRequest, nil)

	Validator.Register(&routeapi.Route{}, routevalidation.ValidateRoute, nil)

	Validator.Register(&sdnapi.ClusterNetwork{}, sdnvalidation.ValidateClusterNetwork, sdnvalidation.ValidateClusterNetworkUpdate)
	Validator.Register(&sdnapi.HostSubnet{}, sdnvalidation.ValidateHostSubnet, sdnvalidation.ValidateHostSubnetUpdate)

	Validator.Register(&templateapi.Template{}, templatevalidation.ValidateTemplate, templatevalidation.ValidateTemplateUpdate)

	Validator.Register(&userapi.User{}, uservalidation.ValidateUser, uservalidation.ValidateUserUpdate)
	Validator.Register(&userapi.Identity{}, uservalidation.ValidateIdentity, uservalidation.ValidateIdentityUpdate)
	Validator.Register(&userapi.UserIdentityMapping{}, uservalidation.ValidateUserIdentityMapping, uservalidation.ValidateUserIdentityMappingUpdate)
}
