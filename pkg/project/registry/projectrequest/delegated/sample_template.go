package delegated

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/serviceaccount"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	projectapi "github.com/openshift/origin/pkg/project/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

const (
	DefaultTemplateName = "project-request"

	ProjectNameParam        = "PROJECT_NAME"
	ProjectDisplayNameParam = "PROJECT_DISPLAYNAME"
	ProjectDescriptionParam = "PROJECT_DESCRIPTION"
	ProjectAdminUserParam   = "PROJECT_ADMIN_USER"
)

var (
	parameters = []string{ProjectNameParam, ProjectDisplayNameParam, ProjectDescriptionParam, ProjectAdminUserParam}
)

func DefaultTemplate() *templateapi.Template {
	ret := &templateapi.Template{}
	ret.Name = DefaultTemplateName

	ns := "${" + ProjectNameParam + "}"

	project := &projectapi.Project{}
	project.Name = ns
	project.Annotations = map[string]string{
		"description": "${" + ProjectDescriptionParam + "}",
		"displayName": "${" + ProjectDisplayNameParam + "}",
	}
	ret.Objects = append(ret.Objects, project)

	binding := &authorizationapi.RoleBinding{}
	binding.Name = "admins"
	binding.Namespace = ns
	binding.Users = util.NewStringSet("${" + ProjectAdminUserParam + "}")
	binding.RoleRef.Name = bootstrappolicy.AdminRoleName
	ret.Objects = append(ret.Objects, binding)

	serviceAccountsBinding := &authorizationapi.RoleBinding{}
	serviceAccountsBinding.Name = "image-pullers"
	serviceAccountsBinding.Namespace = ns
	serviceAccountsBinding.Groups = util.NewStringSet(serviceaccount.MakeNamespaceGroupName(ns))
	serviceAccountsBinding.RoleRef.Name = bootstrappolicy.ImagePullerRoleName
	ret.Objects = append(ret.Objects, serviceAccountsBinding)

	serviceAccountBuilderBinding := &authorizationapi.RoleBinding{}
	serviceAccountBuilderBinding.Name = "image-builders"
	serviceAccountBuilderBinding.Namespace = ns
	serviceAccountBuilderBinding.Users = util.NewStringSet(serviceaccount.MakeUsername(ns, bootstrappolicy.BuilderServiceAccountName))
	serviceAccountBuilderBinding.RoleRef.Name = bootstrappolicy.ImageBuilderRoleName
	ret.Objects = append(ret.Objects, serviceAccountBuilderBinding)

	for _, parameterName := range parameters {
		parameter := templateapi.Parameter{}
		parameter.Name = parameterName
		ret.Parameters = append(ret.Parameters, parameter)
	}

	return ret
}
