package delegated

import (
	"errors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	"github.com/openshift/origin/pkg/client"

	projectapi "github.com/openshift/origin/pkg/project/api"
	projectstorage "github.com/openshift/origin/pkg/project/registry/project/proxy"
	projectrequestregistry "github.com/openshift/origin/pkg/project/registry/projectrequest"
)

type REST struct {
	message            string
	masterNamespace    string
	roleBindingStorage rolebinding.Storage

	projectStorage  projectstorage.REST
	openshiftClient *client.Client
}

func NewREST(message, masterNamespace string, roleBindingStorage rolebinding.Storage, projectStorage projectstorage.REST, openshiftClient *client.Client) *REST {
	return &REST{
		message:            message,
		masterNamespace:    masterNamespace,
		roleBindingStorage: roleBindingStorage,
		projectStorage:     projectStorage,
		openshiftClient:    openshiftClient,
	}
}

func (r *REST) New() runtime.Object {
	return &projectapi.ProjectRequest{}
}

func (r *REST) NewList() runtime.Object {
	return &kapi.Status{}
}

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	if err := rest.BeforeCreate(projectrequestregistry.Strategy, ctx, obj); err != nil {
		return nil, err
	}

	projectRequest := obj.(*projectapi.ProjectRequest)

	project := &projectapi.Project{}
	project.ObjectMeta = projectRequest.ObjectMeta
	project.DisplayName = projectRequest.DisplayName

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

func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	userInfo, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, errors.New("a user must be provided")
	}

	// the caller might not have permission to run a subject access review (he has it by default, but it could have been removed).
	// So we'll escalate for the subject access review to determine rights
	accessReview := &authorizationapi.SubjectAccessReview{
		Verb:     "create",
		Resource: "projectrequests",
		User:     userInfo.GetName(),
		Groups:   util.NewStringSet(userInfo.GetGroups()...),
	}
	accessReviewResponse, err := r.openshiftClient.ClusterSubjectAccessReviews().Create(accessReview)
	if err != nil {
		return nil, err
	}
	if accessReviewResponse.Allowed {
		return &kapi.Status{Status: kapi.StatusSuccess}, nil
	}

	forbiddenError, _ := kapierror.NewForbidden("ProjectRequest", "", errors.New("You may not request a new project via this API.")).(*kapierror.StatusError)
	if len(r.message) > 0 {
		forbiddenError.ErrStatus.Message = r.message
	}
	return nil, forbiddenError
}
