package delegated

import (
	"errors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	utilerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectrequestregistry "github.com/openshift/origin/pkg/project/registry/projectrequest"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

type REST struct {
	message            string
	openshiftNamespace string
	templateName       string

	openshiftClient *client.Client
}

func NewREST(message, openshiftNamespace, templateName string, openshiftClient *client.Client) *REST {
	return &REST{
		message:            message,
		openshiftNamespace: openshiftNamespace,
		templateName:       templateName,
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

	if _, err := r.openshiftClient.Projects().Get(projectRequest.Name); err == nil {
		return nil, kapierror.NewAlreadyExists("project", projectRequest.Name)
	}

	projectName := projectRequest.Name
	projectDisplayName := projectName
	projectDescription := projectDisplayName
	projectAdmin := ""

	if len(projectRequest.DisplayName) > 0 {
		projectDisplayName = projectRequest.DisplayName
	}

	if len(projectRequest.Annotations["description"]) > 0 {
		projectDescription = projectRequest.Annotations["description"]
	}
	if userInfo, exists := kapi.UserFrom(ctx); exists {
		projectAdmin = userInfo.GetName()
	}

	template, err := r.getTemplate()
	if err != nil {
		return nil, err
	}

	for i := range template.Parameters {
		switch template.Parameters[i].Name {
		case ProjectAdminUserParam:
			template.Parameters[i].Value = projectAdmin
		case ProjectDescriptionParam:
			template.Parameters[i].Value = projectDescription
		case ProjectDisplayNameParam:
			template.Parameters[i].Value = projectDisplayName
		case ProjectNameParam:
			template.Parameters[i].Value = projectName
		}
	}

	list, err := r.openshiftClient.TemplateConfigs(kapi.NamespaceDefault).Create(template)
	if err != nil {
		return nil, err
	}
	if err := utilerrors.NewAggregate(runtime.DecodeList(list.Objects, kapi.Scheme)); err != nil {
		return nil, err
	}

	bulk := configcmd.Bulk{
		Mapper: latest.RESTMapper,
		Typer:  kapi.Scheme,
		RESTClientFactory: func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
			return r.openshiftClient, nil
		},
	}
	if err := utilerrors.NewAggregate(bulk.Create(&kapi.List{Items: list.Objects}, projectName)); err != nil {
		return nil, err
	}

	return r.openshiftClient.Projects().Get(projectName)
}

func (r *REST) getTemplate() (*templateapi.Template, error) {
	if len(r.openshiftNamespace) == 0 || len(r.templateName) == 0 {
		return DefaultTemplate(), nil
	}

	return r.openshiftClient.Templates(r.openshiftNamespace).Get(r.templateName)
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

	forbiddenError, _ := kapierror.NewForbidden("ProjectRequest", "", errors.New("you may not request a new project via this API.")).(*kapierror.StatusError)
	if len(r.message) > 0 {
		forbiddenError.ErrStatus.Message = r.message
		forbiddenError.ErrStatus.Details = &kapi.StatusDetails{
			Kind: "ProjectRequest",
			Causes: []kapi.StatusCause{
				{Message: r.message},
			},
		}
	} else {
		forbiddenError.ErrStatus.Message = "You may not request a new project via this API."
	}
	return nil, forbiddenError
}
