package install

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/conversion"

	// we have a strong dependency on kube objects for deployments and scale
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/authentication/install"
	_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	_ "k8s.io/kubernetes/pkg/apis/batch/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"

	_ "github.com/openshift/origin/pkg/authorization/api/install"
	_ "github.com/openshift/origin/pkg/build/api/install"
	_ "github.com/openshift/origin/pkg/cmd/server/api/install"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	_ "github.com/openshift/origin/pkg/image/api/install"
	_ "github.com/openshift/origin/pkg/oauth/api/install"
	_ "github.com/openshift/origin/pkg/project/api/install"
	_ "github.com/openshift/origin/pkg/quota/api/install"
	_ "github.com/openshift/origin/pkg/route/api/install"
	_ "github.com/openshift/origin/pkg/sdn/api/install"
	_ "github.com/openshift/origin/pkg/security/api/install"
	_ "github.com/openshift/origin/pkg/template/api/install"
	_ "github.com/openshift/origin/pkg/user/api/install"

	kv1 "k8s.io/kubernetes/pkg/api/v1"

	watchapi "k8s.io/kubernetes/pkg/watch"
	watchv1 "k8s.io/kubernetes/pkg/watch/versioned"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"

	authorizationv1 "github.com/openshift/origin/pkg/authorization/api/v1"
	buildv1 "github.com/openshift/origin/pkg/build/api/v1"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	imagev1 "github.com/openshift/origin/pkg/image/api/v1"
	oauthv1 "github.com/openshift/origin/pkg/oauth/api/v1"
	projectv1 "github.com/openshift/origin/pkg/project/api/v1"
	routev1 "github.com/openshift/origin/pkg/route/api/v1"
	templatev1 "github.com/openshift/origin/pkg/template/api/v1"
	userv1 "github.com/openshift/origin/pkg/user/api/v1"
)

func init() {
	// This is a "fast-path" that avoids reflection for common types. It focuses on the objects that are
	// converted the most in the cluster.
	// TODO: generate one of these for every external API group - this is to prove the impact
	kapi.Scheme.AddGenericConversionFunc(func(objA, objB interface{}, s conversion.Scope) (bool, error) {
		switch a := objA.(type) {

		case *watchv1.Event:
			switch b := objB.(type) {
			case *watchv1.InternalEvent:
				return true, watchv1.Convert_versioned_Event_to_versioned_InternalEvent(a, b, s)
			case *watchapi.Event:
				return true, watchv1.Convert_versioned_Event_to_watch_Event(a, b, s)
			}
		case *watchv1.InternalEvent:
			switch b := objB.(type) {
			case *watchv1.Event:
				return true, watchv1.Convert_versioned_InternalEvent_to_versioned_Event(a, b, s)
			}
		case *watchapi.Event:
			switch b := objB.(type) {
			case *watchv1.Event:
				return true, watchv1.Convert_watch_Event_to_versioned_Event(a, b, s)
			}

		case *kapi.ListOptions:
			switch b := objB.(type) {
			case *kv1.ListOptions:
				return true, kv1.Convert_api_ListOptions_To_v1_ListOptions(a, b, s)
			}
		case *kv1.ListOptions:
			switch b := objB.(type) {
			case *kapi.ListOptions:
				return true, kv1.Convert_v1_ListOptions_To_api_ListOptions(a, b, s)
			}

		case *kv1.ServiceAccount:
			switch b := objB.(type) {
			case *kapi.ServiceAccount:
				return true, kv1.Convert_v1_ServiceAccount_To_api_ServiceAccount(a, b, s)
			}
		case *kapi.ServiceAccount:
			switch b := objB.(type) {
			case *kv1.ServiceAccount:
				return true, kv1.Convert_api_ServiceAccount_To_v1_ServiceAccount(a, b, s)
			}

		case *kv1.SecretList:
			switch b := objB.(type) {
			case *kapi.SecretList:
				return true, kv1.Convert_v1_SecretList_To_api_SecretList(a, b, s)
			}
		case *kapi.SecretList:
			switch b := objB.(type) {
			case *kv1.SecretList:
				return true, kv1.Convert_api_SecretList_To_v1_SecretList(a, b, s)
			}

		case *kv1.Secret:
			switch b := objB.(type) {
			case *kapi.Secret:
				return true, kv1.Convert_v1_Secret_To_api_Secret(a, b, s)
			}
		case *kapi.Secret:
			switch b := objB.(type) {
			case *kv1.Secret:
				return true, kv1.Convert_api_Secret_To_v1_Secret(a, b, s)
			}

		case *routev1.RouteList:
			switch b := objB.(type) {
			case *routeapi.RouteList:
				return true, routev1.Convert_v1_RouteList_To_api_RouteList(a, b, s)
			}
		case *routeapi.RouteList:
			switch b := objB.(type) {
			case *routev1.RouteList:
				return true, routev1.Convert_api_RouteList_To_v1_RouteList(a, b, s)
			}

		case *routev1.Route:
			switch b := objB.(type) {
			case *routeapi.Route:
				return true, routev1.Convert_v1_Route_To_api_Route(a, b, s)
			}
		case *routeapi.Route:
			switch b := objB.(type) {
			case *routev1.Route:
				return true, routev1.Convert_api_Route_To_v1_Route(a, b, s)
			}

		case *buildv1.BuildList:
			switch b := objB.(type) {
			case *buildapi.BuildList:
				return true, buildv1.Convert_v1_BuildList_To_api_BuildList(a, b, s)
			}
		case *buildapi.BuildList:
			switch b := objB.(type) {
			case *buildv1.BuildList:
				return true, buildv1.Convert_api_BuildList_To_v1_BuildList(a, b, s)
			}

		case *buildv1.BuildConfigList:
			switch b := objB.(type) {
			case *buildapi.BuildConfigList:
				return true, buildv1.Convert_v1_BuildConfigList_To_api_BuildConfigList(a, b, s)
			}
		case *buildapi.BuildConfigList:
			switch b := objB.(type) {
			case *buildv1.BuildConfigList:
				return true, buildv1.Convert_api_BuildConfigList_To_v1_BuildConfigList(a, b, s)
			}

		case *buildv1.BuildConfig:
			switch b := objB.(type) {
			case *buildapi.BuildConfig:
				return true, buildv1.Convert_v1_BuildConfig_To_api_BuildConfig(a, b, s)
			}
		case *buildapi.BuildConfig:
			switch b := objB.(type) {
			case *buildv1.BuildConfig:
				return true, buildv1.Convert_api_BuildConfig_To_v1_BuildConfig(a, b, s)
			}

		case *buildv1.Build:
			switch b := objB.(type) {
			case *buildapi.Build:
				return true, buildv1.Convert_v1_Build_To_api_Build(a, b, s)
			}
		case *buildapi.Build:
			switch b := objB.(type) {
			case *buildv1.Build:
				return true, buildv1.Convert_api_Build_To_v1_Build(a, b, s)
			}
		case *oauthv1.OAuthAuthorizeToken:
			switch b := objB.(type) {
			case *oauthapi.OAuthAuthorizeToken:
				return true, oauthv1.Convert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(a, b, s)
			}
		case *oauthapi.OAuthAuthorizeToken:
			switch b := objB.(type) {
			case *oauthv1.OAuthAuthorizeToken:
				return true, oauthv1.Convert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(a, b, s)
			}

		case *oauthv1.OAuthAccessToken:
			switch b := objB.(type) {
			case *oauthapi.OAuthAccessToken:
				return true, oauthv1.Convert_v1_OAuthAccessToken_To_api_OAuthAccessToken(a, b, s)
			}
		case *oauthapi.OAuthAccessToken:
			switch b := objB.(type) {
			case *oauthv1.OAuthAccessToken:
				return true, oauthv1.Convert_api_OAuthAccessToken_To_v1_OAuthAccessToken(a, b, s)
			}

		case *projectv1.Project:
			switch b := objB.(type) {
			case *projectapi.Project:
				return true, projectv1.Convert_v1_Project_To_api_Project(a, b, s)
			}
		case *projectapi.Project:
			switch b := objB.(type) {
			case *projectv1.Project:
				return true, projectv1.Convert_api_Project_To_v1_Project(a, b, s)
			}

		case *projectv1.ProjectList:
			switch b := objB.(type) {
			case *projectapi.ProjectList:
				return true, projectv1.Convert_v1_ProjectList_To_api_ProjectList(a, b, s)
			}
		case *projectapi.ProjectList:
			switch b := objB.(type) {
			case *projectv1.ProjectList:
				return true, projectv1.Convert_api_ProjectList_To_v1_ProjectList(a, b, s)
			}

		case *templatev1.Template:
			switch b := objB.(type) {
			case *templateapi.Template:
				return true, templatev1.Convert_v1_Template_To_api_Template(a, b, s)
			}
		case *templateapi.Template:
			switch b := objB.(type) {
			case *templatev1.Template:
				return true, templatev1.Convert_api_Template_To_v1_Template(a, b, s)
			}

		case *templatev1.TemplateList:
			switch b := objB.(type) {
			case *templateapi.TemplateList:
				return true, templatev1.Convert_v1_TemplateList_To_api_TemplateList(a, b, s)
			}
		case *templateapi.TemplateList:
			switch b := objB.(type) {
			case *templatev1.TemplateList:
				return true, templatev1.Convert_api_TemplateList_To_v1_TemplateList(a, b, s)
			}

		case *deployv1.DeploymentConfig:
			switch b := objB.(type) {
			case *deployapi.DeploymentConfig:
				return true, deployv1.Convert_v1_DeploymentConfig_To_api_DeploymentConfig(a, b, s)
			}
		case *deployapi.DeploymentConfig:
			switch b := objB.(type) {
			case *deployv1.DeploymentConfig:
				return true, deployv1.Convert_api_DeploymentConfig_To_v1_DeploymentConfig(a, b, s)
			}

		case *imagev1.ImageStream:
			switch b := objB.(type) {
			case *imageapi.ImageStream:
				return true, imagev1.Convert_v1_ImageStream_To_api_ImageStream(a, b, s)
			}
		case *imageapi.ImageStream:
			switch b := objB.(type) {
			case *imagev1.ImageStream:
				return true, imagev1.Convert_api_ImageStream_To_v1_ImageStream(a, b, s)
			}

		case *imagev1.Image:
			switch b := objB.(type) {
			case *imageapi.Image:
				return true, imagev1.Convert_v1_Image_To_api_Image(a, b, s)
			}
		case *imageapi.Image:
			switch b := objB.(type) {
			case *imagev1.Image:
				return true, imagev1.Convert_api_Image_To_v1_Image(a, b, s)
			}

		case *imagev1.ImageSignature:
			switch b := objB.(type) {
			case *imageapi.ImageSignature:
				return true, imagev1.Convert_v1_ImageSignature_To_api_ImageSignature(a, b, s)
			}
		case *imageapi.ImageSignature:
			switch b := objB.(type) {
			case *imagev1.ImageSignature:
				return true, imagev1.Convert_api_ImageSignature_To_v1_ImageSignature(a, b, s)
			}

		case *imagev1.ImageStreamImport:
			switch b := objB.(type) {
			case *imageapi.ImageStreamImport:
				return true, imagev1.Convert_v1_ImageStreamImport_To_api_ImageStreamImport(a, b, s)
			}
		case *imageapi.ImageStreamImport:
			switch b := objB.(type) {
			case *imagev1.ImageStreamImport:
				return true, imagev1.Convert_api_ImageStreamImport_To_v1_ImageStreamImport(a, b, s)
			}

		case *imagev1.ImageStreamList:
			switch b := objB.(type) {
			case *imageapi.ImageStreamList:
				return true, imagev1.Convert_v1_ImageStreamList_To_api_ImageStreamList(a, b, s)
			}
		case *imageapi.ImageStreamList:
			switch b := objB.(type) {
			case *imagev1.ImageStreamList:
				return true, imagev1.Convert_api_ImageStreamList_To_v1_ImageStreamList(a, b, s)
			}

		case *imagev1.ImageStreamImage:
			switch b := objB.(type) {
			case *imageapi.ImageStreamImage:
				return true, imagev1.Convert_v1_ImageStreamImage_To_api_ImageStreamImage(a, b, s)
			}
		case *imageapi.ImageStreamImage:
			switch b := objB.(type) {
			case *imagev1.ImageStreamImage:
				return true, imagev1.Convert_api_ImageStreamImage_To_v1_ImageStreamImage(a, b, s)
			}

		case *imagev1.ImageStreamTag:
			switch b := objB.(type) {
			case *imageapi.ImageStreamTag:
				return true, imagev1.Convert_v1_ImageStreamTag_To_api_ImageStreamTag(a, b, s)
			}
		case *imageapi.ImageStreamTag:
			switch b := objB.(type) {
			case *imagev1.ImageStreamTag:
				return true, imagev1.Convert_api_ImageStreamTag_To_v1_ImageStreamTag(a, b, s)
			}

		case *imagev1.ImageStreamMapping:
			switch b := objB.(type) {
			case *imageapi.ImageStreamMapping:
				return true, imagev1.Convert_v1_ImageStreamMapping_To_api_ImageStreamMapping(a, b, s)
			}
		case *imageapi.ImageStreamMapping:
			switch b := objB.(type) {
			case *imagev1.ImageStreamMapping:
				return true, imagev1.Convert_api_ImageStreamMapping_To_v1_ImageStreamMapping(a, b, s)
			}

		case *authorizationv1.ClusterPolicyBinding:
			switch b := objB.(type) {
			case *authorizationapi.ClusterPolicyBinding:
				return true, authorizationv1.Convert_v1_ClusterPolicyBinding_To_api_ClusterPolicyBinding(a, b, s)
			}
		case *authorizationapi.ClusterPolicyBinding:
			switch b := objB.(type) {
			case *authorizationv1.ClusterPolicyBinding:
				return true, authorizationv1.Convert_api_ClusterPolicyBinding_To_v1_ClusterPolicyBinding(a, b, s)
			}

		case *authorizationv1.PolicyBinding:
			switch b := objB.(type) {
			case *authorizationapi.PolicyBinding:
				return true, authorizationv1.Convert_v1_PolicyBinding_To_api_PolicyBinding(a, b, s)
			}
		case *authorizationapi.PolicyBinding:
			switch b := objB.(type) {
			case *authorizationv1.PolicyBinding:
				return true, authorizationv1.Convert_api_PolicyBinding_To_v1_PolicyBinding(a, b, s)
			}

		case *authorizationv1.ClusterPolicy:
			switch b := objB.(type) {
			case *authorizationapi.ClusterPolicy:
				return true, authorizationv1.Convert_v1_ClusterPolicy_To_api_ClusterPolicy(a, b, s)
			}
		case *authorizationapi.ClusterPolicy:
			switch b := objB.(type) {
			case *authorizationv1.ClusterPolicy:
				return true, authorizationv1.Convert_api_ClusterPolicy_To_v1_ClusterPolicy(a, b, s)
			}

		case *authorizationv1.Policy:
			switch b := objB.(type) {
			case *authorizationapi.Policy:
				return true, authorizationv1.Convert_v1_Policy_To_api_Policy(a, b, s)
			}
		case *authorizationapi.Policy:
			switch b := objB.(type) {
			case *authorizationv1.Policy:
				return true, authorizationv1.Convert_api_Policy_To_v1_Policy(a, b, s)
			}

		case *authorizationv1.ClusterRole:
			switch b := objB.(type) {
			case *authorizationapi.ClusterRole:
				return true, authorizationv1.Convert_v1_ClusterRole_To_api_ClusterRole(a, b, s)
			}
		case *authorizationapi.ClusterRole:
			switch b := objB.(type) {
			case *authorizationv1.ClusterRole:
				return true, authorizationv1.Convert_api_ClusterRole_To_v1_ClusterRole(a, b, s)
			}

		case *authorizationv1.Role:
			switch b := objB.(type) {
			case *authorizationapi.Role:
				return true, authorizationv1.Convert_v1_Role_To_api_Role(a, b, s)
			}
		case *authorizationapi.Role:
			switch b := objB.(type) {
			case *authorizationv1.Role:
				return true, authorizationv1.Convert_api_Role_To_v1_Role(a, b, s)
			}

		case *authorizationv1.ClusterRoleBinding:
			switch b := objB.(type) {
			case *authorizationapi.ClusterRoleBinding:
				return true, authorizationv1.Convert_v1_ClusterRoleBinding_To_api_ClusterRoleBinding(a, b, s)
			}
		case *authorizationapi.ClusterRoleBinding:
			switch b := objB.(type) {
			case *authorizationv1.ClusterRoleBinding:
				return true, authorizationv1.Convert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding(a, b, s)
			}

		case *authorizationv1.RoleBinding:
			switch b := objB.(type) {
			case *authorizationapi.RoleBinding:
				return true, authorizationv1.Convert_v1_RoleBinding_To_api_RoleBinding(a, b, s)
			}
		case *authorizationapi.RoleBinding:
			switch b := objB.(type) {
			case *authorizationv1.RoleBinding:
				return true, authorizationv1.Convert_api_RoleBinding_To_v1_RoleBinding(a, b, s)
			}

		case *authorizationv1.IsPersonalSubjectAccessReview:
			switch b := objB.(type) {
			case *authorizationapi.IsPersonalSubjectAccessReview:
				return true, authorizationv1.Convert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(a, b, s)
			}
		case *authorizationapi.IsPersonalSubjectAccessReview:
			switch b := objB.(type) {
			case *authorizationv1.IsPersonalSubjectAccessReview:
				return true, authorizationv1.Convert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(a, b, s)
			}

		case *userv1.User:
			switch b := objB.(type) {
			case *userapi.User:
				return true, userv1.Convert_v1_User_To_api_User(a, b, s)
			}
		case *userapi.User:
			switch b := objB.(type) {
			case *userv1.User:
				return true, userv1.Convert_api_User_To_v1_User(a, b, s)
			}

		case *userv1.UserList:
			switch b := objB.(type) {
			case *userapi.UserList:
				return true, userv1.Convert_v1_UserList_To_api_UserList(a, b, s)
			}
		case *userapi.UserList:
			switch b := objB.(type) {
			case *userv1.UserList:
				return true, userv1.Convert_api_UserList_To_v1_UserList(a, b, s)
			}

		case *userv1.UserIdentityMapping:
			switch b := objB.(type) {
			case *userapi.UserIdentityMapping:
				return true, userv1.Convert_v1_UserIdentityMapping_To_api_UserIdentityMapping(a, b, s)
			}
		case *userapi.UserIdentityMapping:
			switch b := objB.(type) {
			case *userv1.UserIdentityMapping:
				return true, userv1.Convert_api_UserIdentityMapping_To_v1_UserIdentityMapping(a, b, s)
			}

		case *userv1.Identity:
			switch b := objB.(type) {
			case *userapi.Identity:
				return true, userv1.Convert_v1_Identity_To_api_Identity(a, b, s)
			}
		case *userapi.Identity:
			switch b := objB.(type) {
			case *userv1.Identity:
				return true, userv1.Convert_api_Identity_To_v1_Identity(a, b, s)
			}

		case *userv1.GroupList:
			switch b := objB.(type) {
			case *userapi.GroupList:
				return true, userv1.Convert_v1_GroupList_To_api_GroupList(a, b, s)
			}
		case *userapi.GroupList:
			switch b := objB.(type) {
			case *userv1.GroupList:
				return true, userv1.Convert_api_GroupList_To_v1_GroupList(a, b, s)
			}

		case *userv1.Group:
			switch b := objB.(type) {
			case *userapi.Group:
				return true, userv1.Convert_v1_Group_To_api_Group(a, b, s)
			}
		case *userapi.Group:
			switch b := objB.(type) {
			case *userv1.Group:
				return true, userv1.Convert_api_Group_To_v1_Group(a, b, s)
			}

		}
		return false, nil
	})
}
