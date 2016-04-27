package delegated

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
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
	ProjectRequesterParam   = "PROJECT_REQUESTING_USER"
)

var (
	parameters = []string{ProjectNameParam, ProjectDisplayNameParam, ProjectDescriptionParam, ProjectAdminUserParam, ProjectRequesterParam}
)

func DefaultTemplate() *templateapi.Template {
	ret := &templateapi.Template{}
	ret.Name = DefaultTemplateName

	ns := "${" + ProjectNameParam + "}"

	templateContents := []runtime.Object{}

	project := &projectapi.Project{}
	project.Name = ns
	project.Annotations = map[string]string{
		projectapi.ProjectDescription: "${" + ProjectDescriptionParam + "}",
		projectapi.ProjectDisplayName: "${" + ProjectDisplayNameParam + "}",
		projectapi.ProjectRequester:   "${" + ProjectRequesterParam + "}",
	}
	templateContents = append(templateContents, project)

	serviceAccountRoleBindings := bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(ns)
	for i := range serviceAccountRoleBindings {
		templateContents = append(templateContents, &serviceAccountRoleBindings[i])
	}

	binding := &authorizationapi.RoleBinding{}
	binding.Name = bootstrappolicy.AdminRoleName
	binding.Namespace = ns
	binding.Subjects = []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "${" + ProjectAdminUserParam + "}"}}
	binding.RoleRef.Name = bootstrappolicy.AdminRoleName
	templateContents = append(templateContents, binding)

	if err := templateapi.AddObjectsToTemplate(ret, templateContents, latest.Version); err != nil {
		// this should never happen because we're tightly controlling what goes in.
		panic(err)
	}

	for _, parameterName := range parameters {
		parameter := templateapi.Parameter{}
		parameter.Name = parameterName
		ret.Parameters = append(ret.Parameters, parameter)
	}

	return ret
}
