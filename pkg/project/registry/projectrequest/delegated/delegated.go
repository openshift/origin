package delegated

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/rest"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectrequestregistry "github.com/openshift/origin/pkg/project/registry/projectrequest"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

type REST struct {
	message           string
	templateNamespace string
	templateName      string

	openshiftClient *client.Client
	kubeClient      *kclient.Client
}

func NewREST(message, templateNamespace, templateName string, openshiftClient *client.Client, kubeClient *kclient.Client) *REST {
	return &REST{
		message:           message,
		templateNamespace: templateNamespace,
		templateName:      templateName,
		openshiftClient:   openshiftClient,
		kubeClient:        kubeClient,
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
	projectAdmin := ""
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
			template.Parameters[i].Value = projectRequest.Description
		case ProjectDisplayNameParam:
			template.Parameters[i].Value = projectRequest.DisplayName
		case ProjectNameParam:
			template.Parameters[i].Value = projectName
		}
	}

	list, err := r.openshiftClient.TemplateConfigs(kapi.NamespaceDefault).Create(template)
	if err != nil {
		return nil, err
	}
	if err := utilerrors.NewAggregate(runtime.DecodeList(list.Objects, kapi.Scheme)); err != nil {
		return nil, kapierror.NewInternalError(err)
	}

	// one of the items in this list should be the project.  We are going to locate it, remove it from the list, create it separately
	var projectFromTemplate *projectapi.Project
	objectsToCreate := &kapi.List{}
	for i := range list.Objects {
		if templateProject, ok := list.Objects[i].(*projectapi.Project); ok {
			projectFromTemplate = templateProject

			if len(list.Objects) > (i + 1) {
				objectsToCreate.Items = append(objectsToCreate.Items, list.Objects[i+1:]...)
			}
			break
		}

		objectsToCreate.Items = append(objectsToCreate.Items, list.Objects[i])
	}
	if projectFromTemplate == nil {
		return nil, kapierror.NewInternalError(fmt.Errorf("the project template (%s/%s) is not correctly configured: must contain a project resource", r.templateNamespace, r.templateName))
	}

	// we split out project creation separately so that in a case of racers for the same project, only one will win and create the rest of their template objects
	if _, err := r.openshiftClient.Projects().Create(projectFromTemplate); err != nil {
		return nil, err
	}

	bulk := configcmd.Bulk{
		Mapper: latest.RESTMapper,
		Typer:  kapi.Scheme,
		RESTClientFactory: func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
			if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
				return r.openshiftClient, nil
			}
			return r.kubeClient, nil
		},
	}
	if err := utilerrors.NewAggregate(bulk.Create(objectsToCreate, projectName)); err != nil {
		return nil, kapierror.NewInternalError(err)
	}

	return r.openshiftClient.Projects().Get(projectName)
}

func (r *REST) getTemplate() (*templateapi.Template, error) {
	if len(r.templateNamespace) == 0 || len(r.templateName) == 0 {
		return DefaultTemplate(), nil
	}

	return r.openshiftClient.Templates(r.templateNamespace).Get(r.templateName)
}

func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	userInfo, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, errors.New("a user must be provided")
	}

	// the caller might not have permission to run a subject access review (he has it by default, but it could have been removed).
	// So we'll escalate for the subject access review to determine rights
	accessReview := &authorizationapi.SubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:     "create",
			Resource: "projectrequests",
		},
		User:   userInfo.GetName(),
		Groups: sets.NewString(userInfo.GetGroups()...),
	}
	accessReviewResponse, err := r.openshiftClient.SubjectAccessReviews().Create(accessReview)
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
