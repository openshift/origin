package delegated

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog"

	authorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/util/retry"

	"github.com/openshift/api/project"
	projectv1 "github.com/openshift/api/project/v1"
	templatev1 "github.com/openshift/api/template/v1"
	projectv1typedclient "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned"
	"github.com/openshift/library-go/pkg/authorization/authorizationutil"
	"github.com/openshift/library-go/pkg/template/templateprocessingclient"
	osauthorizationapi "github.com/openshift/openshift-apiserver/pkg/authorization/apis/authorization"
	projectapi "github.com/openshift/openshift-apiserver/pkg/project/apis/project"
	projectrequestregistry "github.com/openshift/openshift-apiserver/pkg/project/apiserver/registry/projectrequest"
)

type REST struct {
	message           string
	templateNamespace string
	templateName      string

	sarClient      authorizationclient.SubjectAccessReviewInterface
	projectGetter  projectv1typedclient.ProjectsGetter
	templateClient templatev1client.Interface
	client         dynamic.Interface
	restMapper     meta.RESTMapper

	// policyBindings is an auth cache that is shared with the authorizer for the API server.
	// we use this cache to detect when the authorizer has observed the change for the auth rules
	roleBindings rbacv1listers.RoleBindingLister
}

var _ rest.Lister = &REST{}
var _ rest.Creater = &REST{}
var _ rest.Scoper = &REST{}

func NewREST(message, templateNamespace, templateName string,
	projectClient projectv1typedclient.ProjectsGetter,
	templateClient templatev1client.Interface,
	sarClient authorizationclient.SubjectAccessReviewInterface,
	client dynamic.Interface,
	restMapper meta.RESTMapper,
	roleBindings rbacv1listers.RoleBindingLister) *REST {
	return &REST{
		message:           message,
		templateNamespace: templateNamespace,
		templateName:      templateName,
		projectGetter:     projectClient,
		templateClient:    templateClient,
		sarClient:         sarClient,
		client:            client,
		restMapper:        restMapper,
		roleBindings:      roleBindings,
	}
}

func (r *REST) New() runtime.Object {
	return &projectapi.ProjectRequest{}
}

func (r *REST) NewList() runtime.Object {
	return &metav1.Status{}
}

func (s *REST) NamespaceScoped() bool {
	return false
}

var _ = rest.Creater(&REST{})

var (
	legacyGroupName   = ""
	ForbiddenNames    = []string{"openshift", "kubernetes", "kube"}
	ForbiddenPrefixes = []string{"openshift-", "kubernetes-", "kube-"}

	defaultRoleBindingNames = sets.NewString("system:image-pullers", "system:image-builders", "system:deployers")
	roleBindingGroups       = sets.NewString(legacyGroupName, osauthorizationapi.GroupName, rbacv1.GroupName)
	roleBindingKind         = "RoleBinding"
)

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	if err := rest.BeforeCreate(projectrequestregistry.Strategy, ctx, obj); err != nil {
		return nil, err
	}
	if err := createValidation(obj); err != nil {
		return nil, err
	}

	projectRequest := obj.(*projectapi.ProjectRequest)
	for _, s := range ForbiddenNames {
		if projectRequest.Name == s {
			return nil, apierror.NewForbidden(project.Resource("project"), projectRequest.Name, fmt.Errorf("cannot request a project with the name %q", s))
		}
	}
	for _, s := range ForbiddenPrefixes {
		if strings.HasPrefix(projectRequest.Name, s) {
			return nil, apierror.NewForbidden(project.Resource("project"), projectRequest.Name, fmt.Errorf("cannot request a project starting with %q", s))
		}
	}

	if _, err := r.projectGetter.Projects().Get(projectRequest.Name, metav1.GetOptions{}); err == nil {
		return nil, apierror.NewAlreadyExists(project.Resource("project"), projectRequest.Name)
	}

	projectName := projectRequest.Name
	projectAdmin := ""
	projectRequester := ""
	if userInfo, exists := apirequest.UserFrom(ctx); exists {
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

	processedList, err := templateprocessingclient.NewDynamicTemplateProcessor(r.client).ProcessToList(template)
	if err != nil {
		return nil, err
	}

	// one of the items in this list should be the project.  We are going to locate it, remove it from the list, create it separately
	var projectFromTemplate *projectv1.Project
	lastRoleBindingName := ""
	objectsToCreate := []*unstructured.Unstructured{}
	for i := range processedList.Items {
		item := processedList.Items[i]
		switch item.GroupVersionKind().GroupKind() {
		case schema.GroupKind{Group: "project.openshift.io", Kind: "Project"}:
			if projectFromTemplate != nil {
				return nil, apierror.NewInternalError(fmt.Errorf("the project template (%s/%s) is not correctly configured: must contain only one project resource", r.templateNamespace, r.templateName))
			}
			projectFromTemplate = &projectv1.Project{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, projectFromTemplate)
			if err != nil {
				return nil, err
			}
			// don't add this to the list to create.  We'll create the project separately.
			continue
		case schema.GroupKind{Group: "authorization.openshift.io", Kind: "RoleBinding"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding"}:
			lastRoleBindingName = item.GetName()
		default:
			// noop, we care only for special handling projects and roles
		}

		// use list.Objects[i] in append to avoid range memory address reuse
		objectsToCreate = append(objectsToCreate, &item)
	}
	if projectFromTemplate == nil {
		return nil, apierror.NewInternalError(fmt.Errorf("the project template (%s/%s) is not correctly configured: must contain a project resource", r.templateNamespace, r.templateName))
	}

	// we split out project creation separately so that in a case of racers for the same project, only one will win and create the rest of their template objects
	createdProject, err := r.projectGetter.Projects().Create(projectFromTemplate)
	if err != nil {
		// log errors other than AlreadyExists and Forbidden
		if !apierror.IsAlreadyExists(err) && !apierror.IsForbidden(err) {
			utilruntime.HandleError(fmt.Errorf("error creating requested project %#v: %v", projectFromTemplate, err))
		}
		return nil, err
	}

	// TODO, stop doing this crazy thing, but for now it's a very simple way to get the unstructured objects we need
	for _, toCreate := range objectsToCreate {
		var restMapping *meta.RESTMapping
		var mappingErr error
		for i := 0; i < 3; i++ {
			restMapping, mappingErr = r.restMapper.RESTMapping(toCreate.GroupVersionKind().GroupKind(), toCreate.GroupVersionKind().Version)
			if mappingErr == nil {
				break
			}
			// the refresh is every 10 seconds
			time.Sleep(11 * time.Second)
		}
		if mappingErr != nil {
			utilruntime.HandleError(fmt.Errorf("error creating items in requested project %q: %v", createdProject.Name, mappingErr))
			// We have to clean up the project if any part of the project request template fails
			if deleteErr := r.projectGetter.Projects().Delete(createdProject.Name, &metav1.DeleteOptions{}); deleteErr != nil {
				utilruntime.HandleError(fmt.Errorf("error cleaning up requested project %q: %v", createdProject.Name, deleteErr))
			}
			return nil, apierror.NewInternalError(mappingErr)
		}

		_, createErr := r.client.Resource(restMapping.Resource).Namespace(createdProject.Name).Create(toCreate, metav1.CreateOptions{})
		// if a default role binding already exists, we're probably racing the controller.  Don't die
		if gvk := restMapping.GroupVersionKind; apierror.IsAlreadyExists(createErr) &&
			gvk.Kind == roleBindingKind && roleBindingGroups.Has(gvk.Group) && defaultRoleBindingNames.Has(toCreate.GetName()) {
			continue
		}
		// it is safe to ignore all such errors since stopOnErr will only let these through for the default role bindings
		if apierror.IsAlreadyExists(createErr) {
			continue
		}
		if createErr != nil {
			utilruntime.HandleError(fmt.Errorf("error creating items in requested project %q: %v", createdProject.Name, createErr))
			// We have to clean up the project if any part of the project request template fails
			if deleteErr := r.projectGetter.Projects().Delete(createdProject.Name, &metav1.DeleteOptions{}); deleteErr != nil {
				utilruntime.HandleError(fmt.Errorf("error cleaning up requested project %q: %v", createdProject.Name, deleteErr))
			}
			return nil, apierror.NewInternalError(createErr)
		}
	}

	// wait for a rolebinding if we created one
	if len(lastRoleBindingName) != 0 {
		r.waitForRoleBinding(createdProject.Name, lastRoleBindingName)
	}

	return r.projectGetter.Projects().Get(createdProject.Name, metav1.GetOptions{})
}

func (r *REST) waitForRoleBinding(namespace, name string) {
	// we have a rolebinding, the we check the cache we have to see if its been updated with this rolebinding
	// if you share a cache with our authorizer (you should), then this will let you know when the authorizer is ready.
	// doesn't matter if this failed.  When the call returns, return.  If we have access great.  If not, oh well.
	backoff := retry.DefaultBackoff
	backoff.Steps = 6 // this effectively waits for 6-ish seconds
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		_, err := r.roleBindings.RoleBindings(namespace).Get(name)
		return err == nil, nil
	})

	if err != nil {
		klog.V(4).Infof("authorization cache failed to update for %v %v: %v", namespace, name, err)
	}
}

func (r *REST) getTemplate() (*templatev1.Template, error) {
	if len(r.templateNamespace) == 0 || len(r.templateName) == 0 {
		return DefaultTemplate(), nil
	}

	return r.templateClient.TemplateV1().Templates(r.templateNamespace).Get(r.templateName, metav1.GetOptions{})
}

var _ = rest.Lister(&REST{})

func (r *REST) List(ctx context.Context, options *metainternal.ListOptions) (runtime.Object, error) {
	userInfo, exists := apirequest.UserFrom(ctx)
	if !exists {
		return nil, errors.New("a user must be provided")
	}

	// the caller might not have permission to run a subject access review (he has it by default, but it could have been removed).
	// So we'll escalate for the subject access review to determine rights
	accessReview := authorizationutil.AddUserToSAR(userInfo, &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:     "create",
				Group:    projectv1.GroupName,
				Resource: "projectrequests",
			},
		},
	})
	accessReviewResponse, err := r.sarClient.Create(accessReview)
	if err != nil {
		return nil, err
	}
	if accessReviewResponse.Status.Allowed {
		return &metav1.Status{Status: metav1.StatusSuccess}, nil
	}

	forbiddenError := apierror.NewForbidden(project.Resource("projectrequest"), "", errors.New("you may not request a new project via this API."))
	if len(r.message) > 0 {
		forbiddenError.ErrStatus.Message = r.message
		forbiddenError.ErrStatus.Details = &metav1.StatusDetails{
			Kind: "ProjectRequest",
			Causes: []metav1.StatusCause{
				{Message: r.message},
			},
		}
	} else {
		forbiddenError.ErrStatus.Message = "You may not request a new project via this API."
	}
	return nil, forbiddenError
}
