package buildlog

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/build"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	BuildRegistry  build.Registry
	PodControl     PodControlInterface
	ConnectionInfo kclient.ConnectionInfoGetter
}

type PodControlInterface interface {
	getPod(namespace, name string) (*kapi.Pod, error)
}

type RealPodControl struct {
	podsNamspacer kclient.PodsNamespacer
}

func (r RealPodControl) getPod(namespace, name string) (*kapi.Pod, error) {
	return r.podsNamspacer.Pods(namespace).Get(name)
}

// NewREST creates a new REST for BuildLog
// Takes build registry and pod client to get necessary attributes to assemble
// URL to which the request shall be redirected in order to get build logs.
func NewREST(b build.Registry, pn kclient.PodsNamespacer, connectionInfo kclient.ConnectionInfoGetter) *REST {
	return &REST{
		BuildRegistry:  b,
		PodControl:     RealPodControl{pn},
		ConnectionInfo: connectionInfo,
	}
}

var _ = rest.Redirector(&REST{})

// Redirector implementation
func (r *REST) ResourceLocation(ctx kapi.Context, id string) (*url.URL, http.RoundTripper, error) {
	build, err := r.BuildRegistry.GetBuild(ctx, id)
	if err != nil {
		return nil, nil, fielderrors.NewFieldNotFound("Build", id)
	}

	// TODO: these must be status errors, not field errors
	// TODO: choose a more appropriate "try again later" status code, like 202
	buildPodName := buildutil.GetBuildPodName(build)
	pod, err := r.PodControl.getPod(build.Namespace, buildPodName)
	if err != nil {
		return nil, nil, fielderrors.NewFieldNotFound("Pod.Name", buildPodName)
	}

	buildPodHost := pod.Status.Host
	buildPodNamespace := pod.Namespace
	// Build will take place only in one container
	buildContainerName := pod.Spec.Containers[0].Name

	scheme, port, transport, err := r.ConnectionInfo.GetConnectionInfo(buildPodHost)
	if err != nil {
		return nil, nil, err
	}

	location := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(buildPodHost, strconv.FormatUint(uint64(port), 10)),
		Path:   fmt.Sprintf("/containerLogs/%s/%s/%s", buildPodNamespace, buildPodName, buildContainerName),
	}

	// Pod in which build take place can't be in the Pending or Unknown phase,
	// cause no containers are present in the Pod in those phases.
	if pod.Status.Phase == kapi.PodPending || pod.Status.Phase == kapi.PodUnknown {
		return nil, nil, fielderrors.NewFieldInvalid("Pod.Status", pod.Status.Phase, "must be Running, Succeeded or Failed")
	}

	switch build.Status {
	case api.BuildStatusRunning:
		location.RawQuery = "follow=1"
	case api.BuildStatusComplete, api.BuildStatusFailed:
		// Do not follow the Complete and Failed logs as the streaming already finished.
	default:
		return nil, nil, fielderrors.NewFieldInvalid("build.Status", build.Status, "must be Running, Complete or Failed")
	}

	return location, transport, nil
}

func (r *REST) New() runtime.Object {
	return &api.BuildLog{}
}
