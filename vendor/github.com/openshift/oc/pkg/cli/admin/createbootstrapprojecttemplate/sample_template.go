package createbootstrapprojecttemplate

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"

	"github.com/openshift/api/annotations"
	projectv1 "github.com/openshift/api/project/v1"
	templatev1 "github.com/openshift/api/template/v1"
)

const (
	DefaultTemplateName = "project-request"

	AdminRoleName = "admin"

	ProjectNameParam        = "PROJECT_NAME"
	ProjectDisplayNameParam = "PROJECT_DISPLAYNAME"
	ProjectDescriptionParam = "PROJECT_DESCRIPTION"
	ProjectAdminUserParam   = "PROJECT_ADMIN_USER"
	ProjectRequesterParam   = "PROJECT_REQUESTING_USER"

	ProjectRequester = "openshift.io/requester"
)

var (
	parameters = []string{ProjectNameParam, ProjectDisplayNameParam, ProjectDescriptionParam, ProjectAdminUserParam, ProjectRequesterParam}
)

func DefaultTemplate() *templatev1.Template {
	scheme := runtime.NewScheme()
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(projectv1.Install(scheme))
	utilruntime.Must(templatev1.Install(scheme))
	codec := serializer.NewCodecFactory(scheme).LegacyCodec(scheme.PrioritizedVersionsAllGroups()...)

	ret := &templatev1.Template{}
	ret.Name = DefaultTemplateName

	ns := "${" + ProjectNameParam + "}"

	project := &projectv1.Project{}
	project.Name = ns
	project.Annotations = map[string]string{
		annotations.OpenShiftDescription: "${" + ProjectDescriptionParam + "}",
		annotations.OpenShiftDisplayName: "${" + ProjectDisplayNameParam + "}",
		ProjectRequester:                 "${" + ProjectRequesterParam + "}",
	}
	objBytes, err := runtime.Encode(codec, project)
	if err != nil {
		panic(err)
	}
	ret.Objects = append(ret.Objects, runtime.RawExtension{Raw: objBytes})

	binding := rbacv1helpers.NewRoleBindingForClusterRole(AdminRoleName, ns).Users("${" + ProjectAdminUserParam + "}").BindingOrDie()
	objBytes, err = runtime.Encode(codec, &binding)
	if err != nil {
		panic(err)
	}
	ret.Objects = append(ret.Objects, runtime.RawExtension{Raw: objBytes})

	for _, parameterName := range parameters {
		parameter := templatev1.Parameter{}
		parameter.Name = parameterName
		ret.Parameters = append(ret.Parameters, parameter)
	}

	return ret
}
