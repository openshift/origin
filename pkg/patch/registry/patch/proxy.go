package patch

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	authenticationapi "github.com/openshift/origin/pkg/auth/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	oclient "github.com/openshift/origin/pkg/client"
	patchapi "github.com/openshift/origin/pkg/patch/api"
)

type REST struct {
	restMapper   meta.RESTMapper
	kubeClient   *kclient.Client
	originClient *oclient.Client
}

func NewREST(restMapper meta.RESTMapper, kubeClient *kclient.Client, originClient *oclient.Client) *REST {
	return &REST{
		restMapper:   restMapper,
		kubeClient:   kubeClient,
		originClient: originClient,
	}
}

func (r *REST) New() runtime.Object {
	return &patchapi.Patch{}
}

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	patch, ok := obj.(*patchapi.Patch)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a patch: %#v", obj))
	}
	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("missing user"))
	}

	targetResource := unversioned.GroupVersionResource{Group: patch.Spec.TargetGroup, Version: patch.Spec.TargetVersion, Resource: patch.Spec.TargetResource}
	targetKind, err := r.restMapper.KindFor(targetResource)
	if err != nil {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("cannot find kind for resource %#v", targetResource))
	}

	// a truly generic generic client fixes this
	var restClient *restclient.RESTClient
	switch {
	case latest.OriginKind(targetKind):
		restClient = r.originClient.RESTClient
	case targetKind.Group == kapi.GroupName:
		restClient = r.kubeClient.RESTClient
	case targetKind.Group == "autoscaling":
		restClient = r.kubeClient.AutoscalingClient.RESTClient
	case targetKind.Group == "batch":
		restClient = r.kubeClient.BatchClient.RESTClient
	case targetKind.Group == "extensions":
		restClient = r.kubeClient.ExtensionsClient.RESTClient
	default:
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("cannot find client for resource %#v", targetResource))
	}

	mapping, err := r.restMapper.RESTMapping(targetKind.GroupKind(), targetKind.Version)
	if err != nil {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("cannot find mapping for resource %#v", targetResource))
	}

	patchRequest := restClient.Patch(kapi.StrategicMergePatchType).SetHeader(authenticationapi.ImpersonateUserHeader, user.GetName())
	for _, scope := range user.GetExtra()[authorizationapi.ScopesKey] {
		patchRequest = patchRequest.SetHeader(authenticationapi.ImpersonateUserScopeHeader, scope)
	}

	result, err := patchRequest.
		NamespaceIfScoped(patch.Spec.TargetNamespace, mapping.Scope.Name() == meta.RESTScopeNameNamespace).
		Resource(patch.Spec.TargetResource).
		Name(patch.Spec.TargetName).
		Body([]byte(patch.Spec.Patch)).
		Do().
		Get()

	status := patchapi.PatchStatus{}
	status.Result = result
	if err != nil {
		status.Error = err.Error()
	}
	patch.Status = status

	return patch, nil
}
