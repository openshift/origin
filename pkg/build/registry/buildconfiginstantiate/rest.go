package buildconfiginstantiate

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	knet "k8s.io/apimachinery/pkg/util/net"
	kubeletremotecommand "k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/apimachinery/pkg/util/wait"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/tools/remotecommand"
	kapi "k8s.io/kubernetes/pkg/api"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/registry/core/pod"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/generator"
	"github.com/openshift/origin/pkg/build/registry"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

var (
	cancelPollInterval = 500 * time.Millisecond
	cancelPollDuration = 30 * time.Second
)

// NewStorage creates a new storage object for build generation
func NewStorage(generator *generator.BuildGenerator) *InstantiateREST {
	return &InstantiateREST{generator: generator}
}

// InstantiateREST is a RESTStorage implementation for a BuildGenerator which supports only
// the Create operation (as the generator has no underlying storage object).
type InstantiateREST struct {
	generator *generator.BuildGenerator
}

// New creates a new build generation request
func (s *InstantiateREST) New() runtime.Object {
	return &buildapi.BuildRequest{}
}

// Create instantiates a new build from a build configuration
func (s *InstantiateREST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}

	request := obj.(*buildapi.BuildRequest)
	if request.TriggeredBy == nil {
		buildTriggerCauses := []buildapi.BuildTriggerCause{}
		request.TriggeredBy = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseManualMsg,
			},
		)
	}
	return s.generator.Instantiate(ctx, request)
}

func NewBinaryStorage(generator *generator.BuildGenerator, watcher rest.Watcher, podClient kcoreclient.PodsGetter, info kubeletclient.ConnectionInfoGetter) *BinaryInstantiateREST {
	return &BinaryInstantiateREST{
		Generator:      generator,
		Watcher:        watcher,
		PodGetter:      &podGetter{podClient},
		ConnectionInfo: info,
		Timeout:        5 * time.Minute,
	}
}

type BinaryInstantiateREST struct {
	Generator      *generator.BuildGenerator
	Watcher        rest.Watcher
	PodGetter      pod.ResourceGetter
	ConnectionInfo kubeletclient.ConnectionInfoGetter
	Timeout        time.Duration
}

// New creates a new build generation request
func (s *BinaryInstantiateREST) New() runtime.Object {
	return &buildapi.BinaryBuildRequestOptions{}
}

// Connect returns a ConnectHandler that will handle the request/response for a request
func (r *BinaryInstantiateREST) Connect(ctx apirequest.Context, name string, options runtime.Object, responder rest.Responder) (http.Handler, error) {
	return &binaryInstantiateHandler{
		r:         r,
		responder: responder,
		ctx:       ctx,
		name:      name,
		options:   options.(*buildapi.BinaryBuildRequestOptions),
	}, nil
}

// NewConnectOptions prepares a binary build request.
func (r *BinaryInstantiateREST) NewConnectOptions() (runtime.Object, bool, string) {
	return &buildapi.BinaryBuildRequestOptions{}, false, ""
}

// ConnectMethods returns POST, the only supported binary method.
func (r *BinaryInstantiateREST) ConnectMethods() []string {
	return []string{"POST"}
}

// binaryInstantiateHandler responds to upload requests
type binaryInstantiateHandler struct {
	r *BinaryInstantiateREST

	responder rest.Responder
	ctx       apirequest.Context
	name      string
	options   *buildapi.BinaryBuildRequestOptions
}

var _ http.Handler = &binaryInstantiateHandler{}

func (h *binaryInstantiateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	build, err := h.handle(r.Body)
	if err != nil {
		h.responder.Error(err)
		return
	}
	h.responder.Object(http.StatusCreated, build)
}

func (h *binaryInstantiateHandler) handle(r io.Reader) (runtime.Object, error) {
	h.options.Name = h.name
	if err := rest.BeforeCreate(BinaryStrategy, h.ctx, h.options); err != nil {
		glog.Infof("failed to validate binary: %#v", h.options)
		return nil, err
	}

	request := &buildapi.BuildRequest{}
	request.Name = h.name
	if len(h.options.Commit) > 0 {
		request.Revision = &buildapi.SourceRevision{
			Git: &buildapi.GitSourceRevision{
				Committer: buildapi.SourceControlUser{
					Name:  h.options.CommitterName,
					Email: h.options.CommitterEmail,
				},
				Author: buildapi.SourceControlUser{
					Name:  h.options.AuthorName,
					Email: h.options.AuthorEmail,
				},
				Message: h.options.Message,
				Commit:  h.options.Commit,
			},
		}
	}
	request.Binary = &buildapi.BinaryBuildSource{
		AsFile: h.options.AsFile,
	}

	var build *buildapi.Build
	start := time.Now()
	if err := wait.Poll(time.Second, h.r.Timeout, func() (bool, error) {
		result, err := h.r.Generator.Instantiate(h.ctx, request)
		if err != nil {
			if errors.IsNotFound(err) {
				if s, ok := err.(errors.APIStatus); ok {
					if s.Status().Kind == "imagestreamtags" {
						return false, nil
					}
				}
			}
			glog.V(2).Infof("failed to instantiate: %#v", request)
			return false, err
		}
		build = result
		return true, nil
	}); err != nil {
		return nil, err
	}
	remaining := h.r.Timeout - time.Now().Sub(start)

	// Attempt to cancel the build if it did not start running
	// before we gave up.
	cancel := true
	defer func() {
		if !cancel {
			return
		}
		h.cancelBuild(build)
	}()

	latest, ok, err := registry.WaitForRunningBuild(h.r.Watcher, h.ctx, build, remaining)

	switch {
	case latest.Status.Phase == buildapi.BuildPhaseError:
		// don't cancel the build if it reached a terminal state on its own
		cancel = false
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s encountered an error: %s", build.Name, buildutil.NoBuildLogsMessage))
	case latest.Status.Phase == buildapi.BuildPhaseFailed:
		// don't cancel the build if it reached a terminal state on its own
		cancel = false
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s failed: %s: %s", build.Name, build.Status.Reason, build.Status.Message))
	case latest.Status.Phase == buildapi.BuildPhaseCancelled:
		// don't cancel the build if it reached a terminal state on its own
		cancel = false
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s was cancelled: %s", build.Name, buildutil.NoBuildLogsMessage))
	case latest.Status.Phase != buildapi.BuildPhaseRunning:
		return nil, errors.NewBadRequest(fmt.Sprintf("cannot upload file to build %s with status %s", build.Name, latest.Status.Phase))
	case err == registry.ErrBuildDeleted:
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s was deleted before it started: %s", build.Name, buildutil.NoBuildLogsMessage))
	case err != nil:
		return nil, errors.NewBadRequest(fmt.Sprintf("unable to wait for build %s to run: %v", build.Name, err))
	case !ok:
		return nil, errors.NewTimeoutError(fmt.Sprintf("timed out waiting for build %s to start after %s", build.Name, h.r.Timeout), 0)
	}

	// The container should be the default build container, so setting it to blank
	buildPodName := buildapi.GetBuildPodName(build)
	opts := &kapi.PodAttachOptions{
		Stdin: true,
		// TODO remove Stdout and Stderr once https://github.com/kubernetes/kubernetes/issues/44448 is
		// fixed
		Stdout: true,
		Stderr: true,
	}
	location, transport, err := pod.AttachLocation(h.r.PodGetter, h.r.ConnectionInfo, h.ctx, buildPodName, opts)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFound(kapi.Resource("pod"), buildPodName)
		}
		return nil, errors.NewBadRequest(err.Error())
	}
	tlsClientConfig, err := knet.TLSClientConfig(transport)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("unable to connect to node, could not retrieve TLS client config: %v", err))
	}
	upgrader := spdy.NewRoundTripper(tlsClientConfig, false)
	exec, err := remotecommand.NewStreamExecutor(upgrader, nil, "POST", location)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("unable to connect to server: %v", err))
	}
	streamOptions := remotecommand.StreamOptions{
		SupportedProtocols: kubeletremotecommand.SupportedStreamingProtocols,
		Stdin:              r,
		// TODO remove Stdout and Stderr once https://github.com/kubernetes/kubernetes/issues/44448 is
		// fixed
		Stdout: ioutil.Discard,
		Stderr: ioutil.Discard,
	}
	if err := exec.Stream(streamOptions); err != nil {
		return nil, errors.NewInternalError(err)
	}
	cancel = false
	return latest, nil
}

// cancelBuild will mark a build for cancellation unless
// cancel is false in which case it is a no-op.
func (h *binaryInstantiateHandler) cancelBuild(build *buildapi.Build) {
	build.Status.Cancelled = true
	h.r.Generator.Client.UpdateBuild(h.ctx, build)
	wait.Poll(cancelPollInterval, cancelPollDuration, func() (bool, error) {
		build.Status.Cancelled = true
		err := h.r.Generator.Client.UpdateBuild(h.ctx, build)
		switch {
		case err != nil && errors.IsConflict(err):
			build, err = h.r.Generator.Client.GetBuild(h.ctx, build.Name, &metav1.GetOptions{})
			return false, err
		default:
			return true, err
		}
	})
}

type podGetter struct {
	podsNamespacer kcoreclient.PodsGetter
}

func (g *podGetter) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("namespace parameter required.")
	}
	return g.podsNamespacer.Pods(ns).Get(name, *options)
}
