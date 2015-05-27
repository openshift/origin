package delegated

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/Godeps/_workspace/src/github.com/GoogleCloudPlatform/kubernetes/pkg/serviceaccount"
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

	saViewersBinding := &authorizationapi.RoleBinding{}
	saViewersBinding.Name = "viewers"
	saViewersBinding.Namespace = ns
	saViewersBinding.Groups = util.NewStringSet(serviceaccount.MakeNamespaceGroupName(ns))
	saViewersBinding.RoleRef.Name = bootstrappolicy.ViewRoleName
	ret.Objects = append(ret.Objects, saViewersBinding)

	saImagePullBinding := &authorizationapi.RoleBinding{}
	saImagePullBinding.Name = "image-pullers"
	saImagePullBinding.Namespace = ns
	saImagePullBinding.Groups = util.NewStringSet(serviceaccount.MakeNamespaceGroupName(ns))
	saImagePullBinding.RoleRef.Name = bootstrappolicy.ImagePullerRoleName
	ret.Objects = append(ret.Objects, saImagePullBinding)

	saImageBuilderBinding := &authorizationapi.RoleBinding{}
	saImageBuilderBinding.Name = "image-builders"
	saImageBuilderBinding.Namespace = ns
	saImageBuilderBinding.Users = util.NewStringSet(serviceaccount.MakeUsername(ns, bootstrappolicy.BuilderServiceAccountName))
	saImageBuilderBinding.RoleRef.Name = bootstrappolicy.ImageBuilderRoleName
	ret.Objects = append(ret.Objects, saImageBuilderBinding)

	for _, parameterName := range parameters {
		parameter := templateapi.Parameter{}
		parameter.Name = parameterName
		ret.Parameters = append(ret.Parameters, parameter)
	}

	return ret
}
