package delegated

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/api/v1"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectapiv1 "github.com/openshift/origin/pkg/project/api/v1"
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

	project := &projectapi.Project{}
	project.Name = ns
	project.Annotations = map[string]string{
		oapi.OpenShiftDescription:   "${" + ProjectDescriptionParam + "}",
		oapi.OpenShiftDisplayName:   "${" + ProjectDisplayNameParam + "}",
		projectapi.ProjectRequester: "${" + ProjectRequesterParam + "}",
	}
	if err := templateapi.AddObjectsToTemplate(ret, []runtime.Object{project}, projectapiv1.SchemeGroupVersion); err != nil {
		panic(err)
	}

	serviceAccountRoleBindings := bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(ns)
	for i := range serviceAccountRoleBindings {
		if err := templateapi.AddObjectsToTemplate(ret, []runtime.Object{&serviceAccountRoleBindings[i]}, authorizationapiv1.SchemeGroupVersion); err != nil {
			panic(err)
		}
	}

	binding := &authorizationapi.RoleBinding{}
	binding.Name = bootstrappolicy.AdminRoleName
	binding.Namespace = ns
	binding.Subjects = []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "${" + ProjectAdminUserParam + "}"}}
	binding.RoleRef.Name = bootstrappolicy.AdminRoleName
	if err := templateapi.AddObjectsToTemplate(ret, []runtime.Object{binding}, authorizationapiv1.SchemeGroupVersion); err != nil {
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
