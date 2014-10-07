package buildlog

import (
	"fmt"
	"net/url"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/build/registry/build"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	BuildRegistry build.Registry
	PodClient     client.PodInterface
	proxyPrefix   string
}

// NewREST creates a new REST for BuildLog
// Takes build registry and pod client to get neccessary attibutes to assamble
// URL to which the request shall be redirected in order to get build logs.
func NewREST(b build.Registry, c client.PodInterface, p string) apiserver.RESTStorage {
	return &REST{
		BuildRegistry: b,
		PodClient:     c,
		proxyPrefix:   p,
	}
}

// Redirector implementation
func (r *REST) ResourceLocation(ctx kubeapi.Context, id string) (string, error) {
	build, err := r.BuildRegistry.GetBuild(id)
	if err != nil {
		return "", fmt.Errorf("No such build")
	}

	pod, err := r.PodClient.GetPod(kubeapi.NewContext(), build.PodID)
	if err != nil {
		return "", fmt.Errorf("No such pod")
	}
	buildPodID := build.PodID
	buildHost := pod.CurrentState.Host
	// Build will take place only in one container
	buildContainerName := pod.DesiredState.Manifest.Containers[0].Name
	location := &url.URL{
		Path: r.proxyPrefix + "/" + buildHost + "/containerLogs/" + buildPodID + "/" + buildContainerName,
	}
	if err != nil {
		return "", err
	}
	return location.String(), nil
}

func (r *REST) Get(ctx kubeapi.Context, id string) (runtime.Object, error) {
	return nil, fmt.Errorf("BuildLog can't be retrieved")
}

func (r *REST) New() runtime.Object {
	return nil
}

func (r *REST) List(ctx kubeapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	return nil, fmt.Errorf("BuildLog can't be listed")
}

func (r *REST) Delete(ctx kubeapi.Context, id string) (<-chan runtime.Object, error) {
	return nil, fmt.Errorf("BuildLog can't be deleted")
}

func (r *REST) Create(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	return nil, fmt.Errorf("BuildLog can't be created")
}

func (r *REST) Update(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	return nil, fmt.Errorf("BuildLog can't be updated")
}
