package delegated

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	authorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"
	authorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"
	rbaclisters "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	osauthorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
	configcmd "github.com/openshift/origin/pkg/bulk"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectclientinternal "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	projectrequestregistry "github.com/openshift/origin/pkg/project/registry/projectrequest"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateinternalclient "github.com/openshift/origin/pkg/template/client/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	restutil "github.com/openshift/origin/pkg/util/rest"
)

type REST struct {
	message           string
	templateNamespace string
	templateName      string

	sarClient      authorizationclient.SubjectAccessReviewInterface
	projectGetter  projectclientinternal.ProjectsGetter
	templateClient templateclient.Interface
	restConfig     *restclient.Config

	// policyBindings is an auth cache that is shared with the authorizer for the API server.
	// we use this cache to detect when the authorizer has observed the change for the auth rules
	roleBindings rbaclisters.RoleBindingLister
}

var _ rest.Lister = &REST{}
var _ rest.Creater = &REST{}

func NewREST(message, templateNamespace, templateName string, projectClient projectclientinternal.ProjectsGetter, templateClient templateclient.Interface, sarClient authorizationclient.SubjectAccessReviewInterface, restConfig *restclient.Config, roleBindings rbaclisters.RoleBindingLister) *REST {
	return &REST{
		message:           message,
		templateNamespace: templateNamespace,
		templateName:      templateName,
		projectGetter:     projectClient,
		templateClient:    templateClient,
		sarClient:         sarClient,
		restConfig:        restConfig,
		roleBindings:      roleBindings,
	}
}

func (r *REST) New() runtime.Object {
	return &projectapi.ProjectRequest{}
}

func (r *REST) NewList() runtime.Object {
	return &metav1.Status{}
}

var _ = rest.Creater(&REST{})

var (
	ForbiddenNames    = []string{"openshift", "kubernetes", "kube"}
	ForbiddenPrefixes = []string{"openshift-", "kubernetes-", "kube-"}

	defaultRoleBindingNames = bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindingNames()
	roleBindingGroups       = sets.NewString(osauthorizationapi.LegacyGroupName, osauthorizationapi.GroupName, rbac.GroupName)
	roleBindingKind         = "RoleBinding"
)

func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, includeUninitialized bool) (runtime.Object, error) {
	if err := rest.BeforeCreate(projectrequestregistry.Strategy, ctx, obj); err != nil {
		return nil, err
	}
	if err := createValidation(obj); err != nil {
		return nil, err
	}

	projectRequest := obj.(*projectapi.ProjectRequest)
	for _, s := range ForbiddenNames {
		if projectRequest.Name == s {
			return nil, kapierror.NewForbidden(projectapi.Resource("project"), projectRequest.Name, fmt.Errorf("cannot request a project with the name %q", s))
		}
	}
	for _, s := range ForbiddenPrefixes {
		if strings.HasPrefix(projectRequest.Name, s) {
			return nil, kapierror.NewForbidden(projectapi.Resource("project"), projectRequest.Name, fmt.Errorf("cannot request a project starting with %q", s))
		}
	}

	if _, err := r.projectGetter.Projects().Get(projectRequest.Name, metav1.GetOptions{}); err == nil {
		return nil, kapierror.NewAlreadyExists(projectapi.Resource("project"), projectRequest.Name)
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

	tc := templateinternalclient.NewTemplateProcessorClient(r.templateClient.Template().RESTClient(), metav1.NamespaceDefault)
	list, err := tc.Process(template)
	if err != nil {
		return nil, err
	}
	if err := utilerrors.NewAggregate(runtime.DecodeList(list.Objects, legacyscheme.Codecs.UniversalDecoder())); err != nil {
		return nil, kapierror.NewInternalError(err)
	}

	// one of the items in this list should be the project.  We are going to locate it, remove it from the list, create it separately
	var projectFromTemplate *projectapi.Project
	lastRoleBindingName := ""
	objectsToCreate := &kapi.List{}
	for i := range list.Objects {
		switch t := list.Objects[i].(type) {
		case *projectapi.Project:
			if projectFromTemplate != nil {
				return nil, kapierror.NewInternalError(fmt.Errorf("the project template (%s/%s) is not correctly configured: must contain only one project resource", r.templateNamespace, r.templateName))
			}
			projectFromTemplate = t
			// don't add this to the list to create.  We'll create the project separately.
			continue
		case *rbac.RoleBinding:
			lastRoleBindingName = t.Name
		case *osauthorizationapi.RoleBinding:
			lastRoleBindingName = t.Name
		default:
			// noop, we care only for special handling projects and roles
		}

		// use list.Objects[i] in append to avoid range memory address reuse
		objectsToCreate.Items = append(objectsToCreate.Items, list.Objects[i])
	}
	if projectFromTemplate == nil {
		return nil, kapierror.NewInternalError(fmt.Errorf("the project template (%s/%s) is not correctly configured: must contain a project resource", r.templateNamespace, r.templateName))
	}

	// we split out project creation separately so that in a case of racers for the same project, only one will win and create the rest of their template objects
	createdProject, err := r.projectGetter.Projects().Create(projectFromTemplate)
	if err != nil {
		// log errors other than AlreadyExists and Forbidden
		if !kapierror.IsAlreadyExists(err) && !kapierror.IsForbidden(err) {
			utilruntime.HandleError(fmt.Errorf("error creating requested project %#v: %v", projectFromTemplate, err))
		}
		return nil, err
	}

	// Stop on the first error, since we have to delete the whole project if any item in the template fails
	stopOnErr := configcmd.AfterFunc(func(info *resource.Info, err error) bool {
		// if a default role binding already exists, we're probably racing the controller.  Don't die
		if gvk := info.Mapping.GroupVersionKind; kapierror.IsAlreadyExists(err) &&
			gvk.Kind == roleBindingKind && roleBindingGroups.Has(gvk.Group) && defaultRoleBindingNames.Has(info.Name) {
			return false
		}
		return err != nil
	})

	bulk := configcmd.Bulk{
		Mapper: &resource.Mapper{
			RESTMapper:   restutil.DefaultMultiRESTMapper(),
			ObjectTyper:  legacyscheme.Scheme,
			ClientMapper: configcmd.ClientMapperFromConfig(r.restConfig),
		},
		IgnoreError: func(err error) bool {
			// it is safe to ignore all such errors since stopOnErr will only let these through for the default role bindings
			return kapierror.IsAlreadyExists(err)
		},
		After: stopOnErr,
		Op:    configcmd.Create,
	}
	if err := utilerrors.NewAggregate(bulk.Run(objectsToCreate, createdProject.Name)); err != nil {
		utilruntime.HandleError(fmt.Errorf("error creating items in requested project %q: %v", createdProject.Name, err))
		// We have to clean up the project if any part of the project request template fails
		if deleteErr := r.projectGetter.Projects().Delete(createdProject.Name, &metav1.DeleteOptions{}); deleteErr != nil {
			utilruntime.HandleError(fmt.Errorf("error cleaning up requested project %q: %v", createdProject.Name, deleteErr))
		}
		return nil, kapierror.NewInternalError(err)
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
		glog.V(4).Infof("authorization cache failed to update for %v %v: %v", namespace, name, err)
	}
}

func (r *REST) getTemplate() (*templateapi.Template, error) {
	if len(r.templateNamespace) == 0 || len(r.templateName) == 0 {
		return DefaultTemplate(), nil
	}

	return r.templateClient.Template().Templates(r.templateNamespace).Get(r.templateName, metav1.GetOptions{})
}

var _ = rest.Lister(&REST{})

func (r *REST) List(ctx apirequest.Context, options *metainternal.ListOptions) (runtime.Object, error) {
	userInfo, exists := apirequest.UserFrom(ctx)
	if !exists {
		return nil, errors.New("a user must be provided")
	}

	// the caller might not have permission to run a subject access review (he has it by default, but it could have been removed).
	// So we'll escalate for the subject access review to determine rights
	accessReview := authorizationutil.AddUserToSAR(userInfo, &authorizationapi.SubjectAccessReview{
		Spec: authorizationapi.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationapi.ResourceAttributes{
				Verb:     "create",
				Group:    projectapi.GroupName,
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

	forbiddenError := kapierror.NewForbidden(projectapi.Resource("projectrequest"), "", errors.New("you may not request a new project via this API."))
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
