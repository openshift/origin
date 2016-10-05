package delegated

import (
	"errors"
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"

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

	// policyBindings is an auth cache that is shared with the authorizer for the API server.
	// we use this cache to detect when the authorizer has observed the change for the auth rules
	policyBindings client.PolicyBindingsListerNamespacer
}

func NewREST(message, templateNamespace, templateName string, openshiftClient *client.Client, kubeClient *kclient.Client, policyBindingCache client.PolicyBindingsListerNamespacer) *REST {
	return &REST{
		message:           message,
		templateNamespace: templateNamespace,
		templateName:      templateName,
		openshiftClient:   openshiftClient,
		kubeClient:        kubeClient,
		policyBindings:    policyBindingCache,
	}
}

func (r *REST) New() runtime.Object {
	return &projectapi.ProjectRequest{}
}

func (r *REST) NewList() runtime.Object {
	return &unversioned.Status{}
}

var _ = rest.Creater(&REST{})

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {

	if err := rest.BeforeCreate(projectrequestregistry.Strategy, ctx, obj); err != nil {
		return nil, err
	}

	projectRequest := obj.(*projectapi.ProjectRequest)

	if _, err := r.openshiftClient.Projects().Get(projectRequest.Name); err == nil {
		return nil, kapierror.NewAlreadyExists(projectapi.Resource("project"), projectRequest.Name)
	}

	projectName := projectRequest.Name
	projectAdmin := ""
	projectRequester := ""
	if userInfo, exists := kapi.UserFrom(ctx); exists {
		projectAdmin = userInfo.GetName()
		projectRequester = userInfo.GetName()
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
		case ProjectRequesterParam:
			template.Parameters[i].Value = projectRequester
		}
	}

	list, err := r.openshiftClient.TemplateConfigs(kapi.NamespaceDefault).Create(template)
	if err != nil {
		return nil, err
	}
	if err := utilerrors.NewAggregate(runtime.DecodeList(list.Objects, kapi.Codecs.UniversalDecoder())); err != nil {
		return nil, kapierror.NewInternalError(err)
	}

	// one of the items in this list should be the project.  We are going to locate it, remove it from the list, create it separately
	var projectFromTemplate *projectapi.Project
	var lastRoleBinding *authorizationapi.RoleBinding
	objectsToCreate := &kapi.List{}
	for i := range list.Objects {
		if templateProject, ok := list.Objects[i].(*projectapi.Project); ok {
			projectFromTemplate = templateProject
			// don't add this to the list to create.  We'll create the project separately.
			continue
		}

		if roleBinding, ok := list.Objects[i].(*authorizationapi.RoleBinding); ok {
			// keep track of the rolebinding, but still add it to the list
			lastRoleBinding = roleBinding
		}

		objectsToCreate.Items = append(objectsToCreate.Items, list.Objects[i])
	}
	if projectFromTemplate == nil {
		return nil, kapierror.NewInternalError(fmt.Errorf("the project template (%s/%s) is not correctly configured: must contain a project resource", r.templateNamespace, r.templateName))
	}

	// we split out project creation separately so that in a case of racers for the same project, only one will win and create the rest of their template objects
	createdProject, err := r.openshiftClient.Projects().Create(projectFromTemplate)
	if err != nil {
		// log errors other than AlreadyExists and Forbidden
		if !kapierror.IsAlreadyExists(err) && !kapierror.IsForbidden(err) {
			utilruntime.HandleError(fmt.Errorf("error creating requested project %#v: %v", projectFromTemplate, err))
		}
		return nil, err
	}

	// Stop on the first error, since we have to delete the whole project if any item in the template fails
	stopOnErr := configcmd.AfterFunc(func(_ *resource.Info, err error) bool {
		return err != nil
	})

	bulk := configcmd.Bulk{
		Mapper: &resource.Mapper{
			RESTMapper:  client.DefaultMultiRESTMapper(),
			ObjectTyper: kapi.Scheme,
			ClientMapper: resource.ClientMapperFunc(func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
				if latest.OriginKind(mapping.GroupVersionKind) {
					return r.openshiftClient, nil
				}
				return r.kubeClient, nil
			}),
		},
		After: stopOnErr,
		Op:    configcmd.Create,
	}
	if err := utilerrors.NewAggregate(bulk.Run(objectsToCreate, projectName)); err != nil {
		utilruntime.HandleError(fmt.Errorf("error creating items in requested project %q: %v", createdProject.Name, err))
		// We have to clean up the project if any part of the project request template fails
		if deleteErr := r.openshiftClient.Projects().Delete(createdProject.Name); deleteErr != nil {
			utilruntime.HandleError(fmt.Errorf("error cleaning up requested project %q: %v", createdProject.Name, deleteErr))
		}
		return nil, kapierror.NewInternalError(err)
	}

	// wait for a rolebinding if we created one
	if lastRoleBinding != nil {
		r.waitForRoleBinding(projectName, lastRoleBinding.Name)
	}

	return r.openshiftClient.Projects().Get(projectName)
}

func (r *REST) waitForRoleBinding(namespace, name string) {
	// we have a rolebinding, the we check the cache we have to see if its been updated with this rolebinding
	// if you share a cache with our authorizer (you should), then this will let you know when the authorizer is ready.
	// doesn't matter if this failed.  When the call returns, return.  If we have access great.  If not, oh well.
	backoff := kclient.DefaultBackoff
	backoff.Steps = 6 // this effectively waits for 6-ish seconds
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		policyBindingList, _ := r.policyBindings.PolicyBindings(namespace).List(kapi.ListOptions{})
		for _, policyBinding := range policyBindingList.Items {
			for roleBindingName := range policyBinding.RoleBindings {
				if roleBindingName == name {
					return true, nil
				}
			}
		}

		return false, nil
	})

	if err != nil {
		glog.V(4).Infof("authorization cache failed to update for %v %v: %v", namespace, name, err)
	}
}

func (r *REST) getTemplate() (*templateapi.Template, error) {
	if len(r.templateNamespace) == 0 || len(r.templateName) == 0 {
		return DefaultTemplate(), nil
	}

	return r.openshiftClient.Templates(r.templateNamespace).Get(r.templateName)
}

var _ = rest.Lister(&REST{})

func (r *REST) List(ctx kapi.Context, options *kapi.ListOptions) (runtime.Object, error) {
	userInfo, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, errors.New("a user must be provided")
	}

	// the caller might not have permission to run a subject access review (he has it by default, but it could have been removed).
	// So we'll escalate for the subject access review to determine rights
	accessReview := authorizationapi.AddUserToSAR(userInfo,
		&authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "create",
				Group:    projectapi.GroupName,
				Resource: "projectrequests",
			},
		})
	accessReviewResponse, err := r.openshiftClient.SubjectAccessReviews().Create(accessReview)
	if err != nil {
		return nil, err
	}
	if accessReviewResponse.Allowed {
		return &unversioned.Status{Status: unversioned.StatusSuccess}, nil
	}

	forbiddenError := kapierror.NewForbidden(projectapi.Resource("projectrequest"), "", errors.New("you may not request a new project via this API."))
	if len(r.message) > 0 {
		forbiddenError.ErrStatus.Message = r.message
		forbiddenError.ErrStatus.Details = &unversioned.StatusDetails{
			Kind: "ProjectRequest",
			Causes: []unversioned.StatusCause{
				{Message: r.message},
			},
		}
	} else {
		forbiddenError.ErrStatus.Message = "You may not request a new project via this API."
	}
	return nil, forbiddenError
}
