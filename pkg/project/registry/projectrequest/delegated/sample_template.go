package delegated

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/rbac"

	projectapiv1 "github.com/openshift/api/project/v1"
	templateapi "github.com/openshift/api/template/v1"
	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
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
	objBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(projectapiv1.GroupVersion), project)
	if err != nil {
		panic(err)
	}
	ret.Objects = append(ret.Objects, runtime.RawExtension{Raw: objBytes})

	// TODO this should be removed in 3.11.  We need to keep it for new server, old controller cases in 3.10.
	serviceAccountRoleBindings := bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(ns)
	for i := range serviceAccountRoleBindings {
		objBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(rbacv1.SchemeGroupVersion), &serviceAccountRoleBindings[i])
		if err != nil {
			panic(err)
		}
		ret.Objects = append(ret.Objects, runtime.RawExtension{Raw: objBytes})
	}

	binding := rbac.NewRoleBindingForClusterRole(bootstrappolicy.AdminRoleName, ns).Users("${" + ProjectAdminUserParam + "}").BindingOrDie()
	objBytes, err = runtime.Encode(legacyscheme.Codecs.LegacyCodec(rbacv1.SchemeGroupVersion), &binding)
	if err != nil {
		panic(err)
	}
	ret.Objects = append(ret.Objects, runtime.RawExtension{Raw: objBytes})

	for _, parameterName := range parameters {
		parameter := templateapi.Parameter{}
		parameter.Name = parameterName
		ret.Parameters = append(ret.Parameters, parameter)
	}

	return ret
}
