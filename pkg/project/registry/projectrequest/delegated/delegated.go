package delegated

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/rolebinding"

	projectapi "github.com/openshift/origin/pkg/project/api"
	projectstorage "github.com/openshift/origin/pkg/project/registry/project/proxy"
	projectrequestregistry "github.com/openshift/origin/pkg/project/registry/projectrequest"
)

type REST struct {
	masterNamespace    string
	roleBindingStorage rolebinding.Storage

	projectStorage projectstorage.REST
}

func NewREST(masterNamespace string, roleBindingStorage rolebinding.Storage, projectStorage projectstorage.REST) *REST {
	return &REST{
		masterNamespace:    masterNamespace,
		roleBindingStorage: roleBindingStorage,
		projectStorage:     projectStorage,
	}
}

func (r *REST) New() runtime.Object {
	return &projectapi.ProjectRequest{}
}

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	if err := rest.BeforeCreate(projectrequestregistry.Strategy, ctx, obj); err != nil {
		return nil, err
	}

	projectRequest := obj.(*projectapi.ProjectRequest)

	project := &projectapi.Project{}
	project.ObjectMeta = projectRequest.ObjectMeta
	project.Annotations["displayName"] = projectRequest.DisplayName

	projectObj, err := r.projectStorage.Create(ctx, project)
	if err != nil {
		return nil, err
	}
	realizedProject := projectObj.(*projectapi.Project)

	adminBinding := &authorizationapi.RoleBinding{}
	adminBinding.Namespace = realizedProject.Name
	adminBinding.Name = "admins"
	adminBinding.RoleRef = kapi.ObjectReference{Namespace: r.masterNamespace, Name: "admin"}
	if userInfo, exists := kapi.UserFrom(ctx); exists {
		adminBinding.Users = util.NewStringSet(userInfo.GetName())
	}

	projectContext := kapi.WithNamespace(ctx, realizedProject.Name)
	if _, err := r.roleBindingStorage.CreateRoleBindingWithEscalation(projectContext, adminBinding); err != nil {
		return nil, err
	}

	return realizedProject, nil
}
