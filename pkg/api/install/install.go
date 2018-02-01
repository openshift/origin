package install

import (
	kv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"

	// we have a strong dependency on kube objects for deployments and scale
	_ "k8s.io/kubernetes/pkg/apis/apps/install"
	_ "k8s.io/kubernetes/pkg/apis/authentication/install"
	_ "k8s.io/kubernetes/pkg/apis/authorization/install"
	_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	_ "k8s.io/kubernetes/pkg/apis/batch/install"
	_ "k8s.io/kubernetes/pkg/apis/certificates/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
	_ "k8s.io/kubernetes/pkg/apis/policy/install"
	_ "k8s.io/kubernetes/pkg/apis/rbac/install"
	_ "k8s.io/kubernetes/pkg/apis/settings/install"
	_ "k8s.io/kubernetes/pkg/apis/storage/install"

	_ "github.com/openshift/origin/pkg/cmd/server/apis/config/install"

	apps "github.com/openshift/origin/pkg/apps/apis/apps/install"
	authz "github.com/openshift/origin/pkg/authorization/apis/authorization/install"
	build "github.com/openshift/origin/pkg/build/apis/build/install"
	image "github.com/openshift/origin/pkg/image/apis/image/install"
	network "github.com/openshift/origin/pkg/network/apis/network/install"
	oauth "github.com/openshift/origin/pkg/oauth/apis/oauth/install"
	project "github.com/openshift/origin/pkg/project/apis/project/install"
	quota "github.com/openshift/origin/pkg/quota/apis/quota/install"
	route "github.com/openshift/origin/pkg/route/apis/route/install"
	security "github.com/openshift/origin/pkg/security/apis/security/install"
	template "github.com/openshift/origin/pkg/template/apis/template/install"
	user "github.com/openshift/origin/pkg/user/apis/user/install"

	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchapi "k8s.io/apimachinery/pkg/watch"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	userapi "github.com/openshift/origin/pkg/user/apis/user"

	appsv1 "github.com/openshift/api/apps/v1"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	projectv1 "github.com/openshift/api/project/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	userv1 "github.com/openshift/api/user/v1"

	appsconversionv1 "github.com/openshift/origin/pkg/apps/apis/apps/v1"
	authorizationconversionv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	buildconversionv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	imageconversionv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	oauthconversionv1 "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
	projectconversionv1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	routeconversionv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
	templateconversionv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	userconversionv1 "github.com/openshift/origin/pkg/user/apis/user/v1"
)

func init() {
	// This is a "fast-path" that avoids reflection for common types. It focuses on the objects that are
	// converted the most in the cluster.
	// TODO: generate one of these for every external API group - this is to prove the impact
	legacyscheme.Scheme.AddGenericConversionFunc(func(objA, objB interface{}, s conversion.Scope) (bool, error) {
		switch a := objA.(type) {
		case *metav1.WatchEvent:
			switch b := objB.(type) {
			case *metav1.InternalEvent:
				return true, metav1.Convert_versioned_Event_to_versioned_InternalEvent(a, b, s)
			case *watchapi.Event:
				return true, metav1.Convert_versioned_Event_to_watch_Event(a, b, s)
			}
		case *metav1.InternalEvent:
			switch b := objB.(type) {
			case *metav1.WatchEvent:
				return true, metav1.Convert_versioned_InternalEvent_to_versioned_Event(a, b, s)
			}
		case *watchapi.Event:
			switch b := objB.(type) {
			case *metav1.WatchEvent:
				return true, metav1.Convert_watch_Event_to_versioned_Event(a, b, s)
			}

		case *metainternal.ListOptions:
			switch b := objB.(type) {
			case *metav1.ListOptions:
				return true, metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(a, b, s)
			}
		case *metav1.ListOptions:
			switch b := objB.(type) {
			case *metainternal.ListOptions:
				return true, metainternal.Convert_v1_ListOptions_To_internalversion_ListOptions(a, b, s)
			}

		case *kv1.ServiceAccount:
			switch b := objB.(type) {
			case *kapi.ServiceAccount:
				return true, kapiv1.Convert_v1_ServiceAccount_To_core_ServiceAccount(a, b, s)
			}
		case *kapi.ServiceAccount:
			switch b := objB.(type) {
			case *kv1.ServiceAccount:
				return true, kapiv1.Convert_core_ServiceAccount_To_v1_ServiceAccount(a, b, s)
			}

		case *kv1.SecretList:
			switch b := objB.(type) {
			case *kapi.SecretList:
				return true, kapiv1.Convert_v1_SecretList_To_core_SecretList(a, b, s)
			}
		case *kapi.SecretList:
			switch b := objB.(type) {
			case *kv1.SecretList:
				return true, kapiv1.Convert_core_SecretList_To_v1_SecretList(a, b, s)
			}

		case *kv1.Secret:
			switch b := objB.(type) {
			case *kapi.Secret:
				return true, kapiv1.Convert_v1_Secret_To_core_Secret(a, b, s)
			}
		case *kapi.Secret:
			switch b := objB.(type) {
			case *kv1.Secret:
				return true, kapiv1.Convert_core_Secret_To_v1_Secret(a, b, s)
			}

		case *routev1.RouteList:
			switch b := objB.(type) {
			case *routeapi.RouteList:
				return true, routeconversionv1.Convert_v1_RouteList_To_route_RouteList(a, b, s)
			}
		case *routeapi.RouteList:
			switch b := objB.(type) {
			case *routev1.RouteList:
				return true, routeconversionv1.Convert_route_RouteList_To_v1_RouteList(a, b, s)
			}

		case *routev1.Route:
			switch b := objB.(type) {
			case *routeapi.Route:
				return true, routeconversionv1.Convert_v1_Route_To_route_Route(a, b, s)
			}
		case *routeapi.Route:
			switch b := objB.(type) {
			case *routev1.Route:
				return true, routeconversionv1.Convert_route_Route_To_v1_Route(a, b, s)
			}

		case *buildv1.BuildList:
			switch b := objB.(type) {
			case *buildapi.BuildList:
				return true, buildconversionv1.Convert_v1_BuildList_To_build_BuildList(a, b, s)
			}
		case *buildapi.BuildList:
			switch b := objB.(type) {
			case *buildv1.BuildList:
				return true, buildconversionv1.Convert_build_BuildList_To_v1_BuildList(a, b, s)
			}

		case *buildv1.BuildConfigList:
			switch b := objB.(type) {
			case *buildapi.BuildConfigList:
				return true, buildconversionv1.Convert_v1_BuildConfigList_To_build_BuildConfigList(a, b, s)
			}
		case *buildapi.BuildConfigList:
			switch b := objB.(type) {
			case *buildv1.BuildConfigList:
				return true, buildconversionv1.Convert_build_BuildConfigList_To_v1_BuildConfigList(a, b, s)
			}

		case *buildv1.BuildConfig:
			switch b := objB.(type) {
			case *buildapi.BuildConfig:
				return true, buildconversionv1.Convert_v1_BuildConfig_To_build_BuildConfig(a, b, s)
			}
		case *buildapi.BuildConfig:
			switch b := objB.(type) {
			case *buildv1.BuildConfig:
				return true, buildconversionv1.Convert_build_BuildConfig_To_v1_BuildConfig(a, b, s)
			}

		case *buildv1.Build:
			switch b := objB.(type) {
			case *buildapi.Build:
				return true, buildconversionv1.Convert_v1_Build_To_build_Build(a, b, s)
			}
		case *buildapi.Build:
			switch b := objB.(type) {
			case *buildv1.Build:
				return true, buildconversionv1.Convert_build_Build_To_v1_Build(a, b, s)
			}
		case *oauthv1.OAuthAuthorizeToken:
			switch b := objB.(type) {
			case *oauthapi.OAuthAuthorizeToken:
				return true, oauthconversionv1.Convert_v1_OAuthAuthorizeToken_To_oauth_OAuthAuthorizeToken(a, b, s)
			}
		case *oauthapi.OAuthAuthorizeToken:
			switch b := objB.(type) {
			case *oauthv1.OAuthAuthorizeToken:
				return true, oauthconversionv1.Convert_oauth_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(a, b, s)
			}

		case *oauthv1.OAuthAccessToken:
			switch b := objB.(type) {
			case *oauthapi.OAuthAccessToken:
				return true, oauthconversionv1.Convert_v1_OAuthAccessToken_To_oauth_OAuthAccessToken(a, b, s)
			}
		case *oauthapi.OAuthAccessToken:
			switch b := objB.(type) {
			case *oauthv1.OAuthAccessToken:
				return true, oauthconversionv1.Convert_oauth_OAuthAccessToken_To_v1_OAuthAccessToken(a, b, s)
			}

		case *projectv1.Project:
			switch b := objB.(type) {
			case *projectapi.Project:
				return true, projectconversionv1.Convert_v1_Project_To_project_Project(a, b, s)
			}
		case *projectapi.Project:
			switch b := objB.(type) {
			case *projectv1.Project:
				return true, projectconversionv1.Convert_project_Project_To_v1_Project(a, b, s)
			}

		case *projectv1.ProjectList:
			switch b := objB.(type) {
			case *projectapi.ProjectList:
				return true, projectconversionv1.Convert_v1_ProjectList_To_project_ProjectList(a, b, s)
			}
		case *projectapi.ProjectList:
			switch b := objB.(type) {
			case *projectv1.ProjectList:
				return true, projectconversionv1.Convert_project_ProjectList_To_v1_ProjectList(a, b, s)
			}

		case *templatev1.Template:
			switch b := objB.(type) {
			case *templateapi.Template:
				return true, templateconversionv1.Convert_v1_Template_To_template_Template(a, b, s)
			}
		case *templateapi.Template:
			switch b := objB.(type) {
			case *templatev1.Template:
				return true, templateconversionv1.Convert_template_Template_To_v1_Template(a, b, s)
			}

		case *templatev1.TemplateList:
			switch b := objB.(type) {
			case *templateapi.TemplateList:
				return true, templateconversionv1.Convert_v1_TemplateList_To_template_TemplateList(a, b, s)
			}
		case *templateapi.TemplateList:
			switch b := objB.(type) {
			case *templatev1.TemplateList:
				return true, templateconversionv1.Convert_template_TemplateList_To_v1_TemplateList(a, b, s)
			}

		case *templatev1.TemplateInstance:
			switch b := objB.(type) {
			case *templateapi.TemplateInstance:
				return true, templateconversionv1.Convert_v1_TemplateInstance_To_template_TemplateInstance(a, b, s)
			}
		case *templateapi.TemplateInstance:
			switch b := objB.(type) {
			case *templatev1.TemplateInstance:
				return true, templateconversionv1.Convert_template_TemplateInstance_To_v1_TemplateInstance(a, b, s)
			}

		case *templatev1.TemplateInstanceList:
			switch b := objB.(type) {
			case *templateapi.TemplateInstanceList:
				return true, templateconversionv1.Convert_v1_TemplateInstanceList_To_template_TemplateInstanceList(a, b, s)
			}
		case *templateapi.TemplateInstanceList:
			switch b := objB.(type) {
			case *templatev1.TemplateInstanceList:
				return true, templateconversionv1.Convert_template_TemplateInstanceList_To_v1_TemplateInstanceList(a, b, s)
			}

		case *templatev1.BrokerTemplateInstance:
			switch b := objB.(type) {
			case *templateapi.BrokerTemplateInstance:
				return true, templateconversionv1.Convert_v1_BrokerTemplateInstance_To_template_BrokerTemplateInstance(a, b, s)
			}
		case *templateapi.BrokerTemplateInstance:
			switch b := objB.(type) {
			case *templatev1.BrokerTemplateInstance:
				return true, templateconversionv1.Convert_template_BrokerTemplateInstance_To_v1_BrokerTemplateInstance(a, b, s)
			}

		case *templatev1.BrokerTemplateInstanceList:
			switch b := objB.(type) {
			case *templateapi.BrokerTemplateInstanceList:
				return true, templateconversionv1.Convert_v1_BrokerTemplateInstanceList_To_template_BrokerTemplateInstanceList(a, b, s)
			}
		case *templateapi.BrokerTemplateInstanceList:
			switch b := objB.(type) {
			case *templatev1.BrokerTemplateInstanceList:
				return true, templateconversionv1.Convert_template_BrokerTemplateInstanceList_To_v1_BrokerTemplateInstanceList(a, b, s)
			}

		case *appsv1.DeploymentConfig:
			switch b := objB.(type) {
			case *appsapi.DeploymentConfig:
				return true, appsconversionv1.Convert_v1_DeploymentConfig_To_apps_DeploymentConfig(a, b, s)
			}
		case *appsapi.DeploymentConfig:
			switch b := objB.(type) {
			case *appsv1.DeploymentConfig:
				return true, appsconversionv1.Convert_apps_DeploymentConfig_To_v1_DeploymentConfig(a, b, s)
			}

		case *imagev1.ImageStream:
			switch b := objB.(type) {
			case *imageapi.ImageStream:
				return true, imageconversionv1.Convert_v1_ImageStream_To_image_ImageStream(a, b, s)
			}
		case *imageapi.ImageStream:
			switch b := objB.(type) {
			case *imagev1.ImageStream:
				return true, imageconversionv1.Convert_image_ImageStream_To_v1_ImageStream(a, b, s)
			}

		case *imagev1.Image:
			switch b := objB.(type) {
			case *imageapi.Image:
				return true, imageconversionv1.Convert_v1_Image_To_image_Image(a, b, s)
			}
		case *imageapi.Image:
			switch b := objB.(type) {
			case *imagev1.Image:
				return true, imageconversionv1.Convert_image_Image_To_v1_Image(a, b, s)
			}

		case *imagev1.ImageSignature:
			switch b := objB.(type) {
			case *imageapi.ImageSignature:
				return true, imageconversionv1.Convert_v1_ImageSignature_To_image_ImageSignature(a, b, s)
			}
		case *imageapi.ImageSignature:
			switch b := objB.(type) {
			case *imagev1.ImageSignature:
				return true, imageconversionv1.Convert_image_ImageSignature_To_v1_ImageSignature(a, b, s)
			}

		case *imagev1.ImageStreamImport:
			switch b := objB.(type) {
			case *imageapi.ImageStreamImport:
				return true, imageconversionv1.Convert_v1_ImageStreamImport_To_image_ImageStreamImport(a, b, s)
			}
		case *imageapi.ImageStreamImport:
			switch b := objB.(type) {
			case *imagev1.ImageStreamImport:
				return true, imageconversionv1.Convert_image_ImageStreamImport_To_v1_ImageStreamImport(a, b, s)
			}

		case *imagev1.ImageStreamList:
			switch b := objB.(type) {
			case *imageapi.ImageStreamList:
				return true, imageconversionv1.Convert_v1_ImageStreamList_To_image_ImageStreamList(a, b, s)
			}
		case *imageapi.ImageStreamList:
			switch b := objB.(type) {
			case *imagev1.ImageStreamList:
				return true, imageconversionv1.Convert_image_ImageStreamList_To_v1_ImageStreamList(a, b, s)
			}

		case *imagev1.ImageStreamImage:
			switch b := objB.(type) {
			case *imageapi.ImageStreamImage:
				return true, imageconversionv1.Convert_v1_ImageStreamImage_To_image_ImageStreamImage(a, b, s)
			}
		case *imageapi.ImageStreamImage:
			switch b := objB.(type) {
			case *imagev1.ImageStreamImage:
				return true, imageconversionv1.Convert_image_ImageStreamImage_To_v1_ImageStreamImage(a, b, s)
			}

		case *imagev1.ImageStreamTag:
			switch b := objB.(type) {
			case *imageapi.ImageStreamTag:
				return true, imageconversionv1.Convert_v1_ImageStreamTag_To_image_ImageStreamTag(a, b, s)
			}
		case *imageapi.ImageStreamTag:
			switch b := objB.(type) {
			case *imagev1.ImageStreamTag:
				return true, imageconversionv1.Convert_image_ImageStreamTag_To_v1_ImageStreamTag(a, b, s)
			}

		case *imagev1.ImageStreamMapping:
			switch b := objB.(type) {
			case *imageapi.ImageStreamMapping:
				return true, imageconversionv1.Convert_v1_ImageStreamMapping_To_image_ImageStreamMapping(a, b, s)
			}
		case *imageapi.ImageStreamMapping:
			switch b := objB.(type) {
			case *imagev1.ImageStreamMapping:
				return true, imageconversionv1.Convert_image_ImageStreamMapping_To_v1_ImageStreamMapping(a, b, s)
			}

		case *authorizationv1.ClusterPolicyBinding:
			switch b := objB.(type) {
			case *authorizationapi.ClusterPolicyBinding:
				return true, authorizationconversionv1.Convert_v1_ClusterPolicyBinding_To_authorization_ClusterPolicyBinding(a, b, s)
			}
		case *authorizationapi.ClusterPolicyBinding:
			switch b := objB.(type) {
			case *authorizationv1.ClusterPolicyBinding:
				return true, authorizationconversionv1.Convert_authorization_ClusterPolicyBinding_To_v1_ClusterPolicyBinding(a, b, s)
			}

		case *authorizationv1.PolicyBinding:
			switch b := objB.(type) {
			case *authorizationapi.PolicyBinding:
				return true, authorizationconversionv1.Convert_v1_PolicyBinding_To_authorization_PolicyBinding(a, b, s)
			}
		case *authorizationapi.PolicyBinding:
			switch b := objB.(type) {
			case *authorizationv1.PolicyBinding:
				return true, authorizationconversionv1.Convert_authorization_PolicyBinding_To_v1_PolicyBinding(a, b, s)
			}

		case *authorizationv1.ClusterPolicy:
			switch b := objB.(type) {
			case *authorizationapi.ClusterPolicy:
				return true, authorizationconversionv1.Convert_v1_ClusterPolicy_To_authorization_ClusterPolicy(a, b, s)
			}
		case *authorizationapi.ClusterPolicy:
			switch b := objB.(type) {
			case *authorizationv1.ClusterPolicy:
				return true, authorizationconversionv1.Convert_authorization_ClusterPolicy_To_v1_ClusterPolicy(a, b, s)
			}

		case *authorizationv1.Policy:
			switch b := objB.(type) {
			case *authorizationapi.Policy:
				return true, authorizationconversionv1.Convert_v1_Policy_To_authorization_Policy(a, b, s)
			}
		case *authorizationapi.Policy:
			switch b := objB.(type) {
			case *authorizationv1.Policy:
				return true, authorizationconversionv1.Convert_authorization_Policy_To_v1_Policy(a, b, s)
			}

		case *authorizationv1.ClusterRole:
			switch b := objB.(type) {
			case *authorizationapi.ClusterRole:
				return true, authorizationconversionv1.Convert_v1_ClusterRole_To_authorization_ClusterRole(a, b, s)
			}
		case *authorizationapi.ClusterRole:
			switch b := objB.(type) {
			case *authorizationv1.ClusterRole:
				return true, authorizationconversionv1.Convert_authorization_ClusterRole_To_v1_ClusterRole(a, b, s)
			}

		case *authorizationv1.Role:
			switch b := objB.(type) {
			case *authorizationapi.Role:
				return true, authorizationconversionv1.Convert_v1_Role_To_authorization_Role(a, b, s)
			}
		case *authorizationapi.Role:
			switch b := objB.(type) {
			case *authorizationv1.Role:
				return true, authorizationconversionv1.Convert_authorization_Role_To_v1_Role(a, b, s)
			}

		case *authorizationv1.ClusterRoleBinding:
			switch b := objB.(type) {
			case *authorizationapi.ClusterRoleBinding:
				return true, authorizationconversionv1.Convert_v1_ClusterRoleBinding_To_authorization_ClusterRoleBinding(a, b, s)
			}
		case *authorizationapi.ClusterRoleBinding:
			switch b := objB.(type) {
			case *authorizationv1.ClusterRoleBinding:
				return true, authorizationconversionv1.Convert_authorization_ClusterRoleBinding_To_v1_ClusterRoleBinding(a, b, s)
			}

		case *authorizationv1.RoleBinding:
			switch b := objB.(type) {
			case *authorizationapi.RoleBinding:
				return true, authorizationconversionv1.Convert_v1_RoleBinding_To_authorization_RoleBinding(a, b, s)
			}
		case *authorizationapi.RoleBinding:
			switch b := objB.(type) {
			case *authorizationv1.RoleBinding:
				return true, authorizationconversionv1.Convert_authorization_RoleBinding_To_v1_RoleBinding(a, b, s)
			}

		case *authorizationv1.IsPersonalSubjectAccessReview:
			switch b := objB.(type) {
			case *authorizationapi.IsPersonalSubjectAccessReview:
				return true, authorizationconversionv1.Convert_v1_IsPersonalSubjectAccessReview_To_authorization_IsPersonalSubjectAccessReview(a, b, s)
			}
		case *authorizationapi.IsPersonalSubjectAccessReview:
			switch b := objB.(type) {
			case *authorizationv1.IsPersonalSubjectAccessReview:
				return true, authorizationconversionv1.Convert_authorization_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(a, b, s)
			}

		case *authorizationv1.RoleBindingRestriction:
			switch b := objB.(type) {
			case *authorizationapi.RoleBindingRestriction:
				return true, authorizationconversionv1.Convert_v1_RoleBindingRestriction_To_authorization_RoleBindingRestriction(a, b, s)
			}
		case *authorizationapi.RoleBindingRestriction:
			switch b := objB.(type) {
			case *authorizationv1.RoleBindingRestriction:
				return true, authorizationconversionv1.Convert_authorization_RoleBindingRestriction_To_v1_RoleBindingRestriction(a, b, s)
			}

		case *userv1.User:
			switch b := objB.(type) {
			case *userapi.User:
				return true, userconversionv1.Convert_v1_User_To_user_User(a, b, s)
			}
		case *userapi.User:
			switch b := objB.(type) {
			case *userv1.User:
				return true, userconversionv1.Convert_user_User_To_v1_User(a, b, s)
			}

		case *userv1.UserList:
			switch b := objB.(type) {
			case *userapi.UserList:
				return true, userconversionv1.Convert_v1_UserList_To_user_UserList(a, b, s)
			}
		case *userapi.UserList:
			switch b := objB.(type) {
			case *userv1.UserList:
				return true, userconversionv1.Convert_user_UserList_To_v1_UserList(a, b, s)
			}

		case *userv1.UserIdentityMapping:
			switch b := objB.(type) {
			case *userapi.UserIdentityMapping:
				return true, userconversionv1.Convert_v1_UserIdentityMapping_To_user_UserIdentityMapping(a, b, s)
			}
		case *userapi.UserIdentityMapping:
			switch b := objB.(type) {
			case *userv1.UserIdentityMapping:
				return true, userconversionv1.Convert_user_UserIdentityMapping_To_v1_UserIdentityMapping(a, b, s)
			}

		case *userv1.Identity:
			switch b := objB.(type) {
			case *userapi.Identity:
				return true, userconversionv1.Convert_v1_Identity_To_user_Identity(a, b, s)
			}
		case *userapi.Identity:
			switch b := objB.(type) {
			case *userv1.Identity:
				return true, userconversionv1.Convert_user_Identity_To_v1_Identity(a, b, s)
			}

		case *userv1.GroupList:
			switch b := objB.(type) {
			case *userapi.GroupList:
				return true, userconversionv1.Convert_v1_GroupList_To_user_GroupList(a, b, s)
			}
		case *userapi.GroupList:
			switch b := objB.(type) {
			case *userv1.GroupList:
				return true, userconversionv1.Convert_user_GroupList_To_v1_GroupList(a, b, s)
			}

		case *userv1.Group:
			switch b := objB.(type) {
			case *userapi.Group:
				return true, userconversionv1.Convert_v1_Group_To_user_Group(a, b, s)
			}
		case *userapi.Group:
			switch b := objB.(type) {
			case *userv1.Group:
				return true, userconversionv1.Convert_user_Group_To_v1_Group(a, b, s)
			}

		}
		return false, nil
	})
}

func InstallAll(scheme *runtime.Scheme, groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager) {
	// add Origin types to the given scheme
	apps.Install(groupFactoryRegistry, registry, scheme)
	authz.Install(groupFactoryRegistry, registry, scheme)
	build.Install(groupFactoryRegistry, registry, scheme)
	image.Install(groupFactoryRegistry, registry, scheme)
	network.Install(groupFactoryRegistry, registry, scheme)
	oauth.Install(groupFactoryRegistry, registry, scheme)
	project.Install(groupFactoryRegistry, registry, scheme)
	quota.Install(groupFactoryRegistry, registry, scheme)
	route.Install(groupFactoryRegistry, registry, scheme)
	security.Install(groupFactoryRegistry, registry, scheme)
	template.Install(groupFactoryRegistry, registry, scheme)
	user.Install(groupFactoryRegistry, registry, scheme)
}
