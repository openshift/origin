package buildconfiginstantiate

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/registry/pod"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/httpstream/spdy"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/generator"
	"github.com/openshift/origin/pkg/build/registry"
	buildutil "github.com/openshift/origin/pkg/build/util"
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
func (s *InstantiateREST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}

	return s.generator.Instantiate(ctx, obj.(*buildapi.BuildRequest))
}

func NewBinaryStorage(generator *generator.BuildGenerator, watcher rest.Watcher, podClient unversioned.PodsNamespacer, info kubeletclient.ConnectionInfoGetter) *BinaryInstantiateREST {
	return &BinaryInstantiateREST{
		Generator:      generator,
		Watcher:        watcher,
		PodGetter:      &podGetter{podClient},
		ConnectionInfo: info,
		Timeout:        time.Minute,
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
func (r *BinaryInstantiateREST) Connect(ctx kapi.Context, name string, options runtime.Object, responder rest.Responder) (http.Handler, error) {
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
	ctx       kapi.Context
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
	build, err := h.r.Generator.Instantiate(h.ctx, request)
	if err != nil {
		glog.Infof("failed to instantiate: %#v", request)
		return nil, err
	}

	latest, ok, err := registry.WaitForRunningBuild(h.r.Watcher, h.ctx, build, h.r.Timeout)
	if err != nil {
		switch latest.Status.Phase {
		case buildapi.BuildPhaseError:
			return nil, errors.NewBadRequest(fmt.Sprintf("build %s encountered an error: %s", build.Name, buildutil.NoBuildLogsMessage))
		case buildapi.BuildPhaseCancelled:
			return nil, errors.NewBadRequest(fmt.Sprintf("build %s was cancelled: %s", build.Name, buildutil.NoBuildLogsMessage))
		}
		return nil, errors.NewBadRequest(fmt.Sprintf("unable to wait for build %s to run: %v", build.Name, err))
	}
	if !ok {
		return nil, errors.NewTimeoutError(fmt.Sprintf("timed out waiting for build %s to start after %s", build.Name, h.r.Timeout), 0)
	}
	if latest.Status.Phase != buildapi.BuildPhaseRunning {
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s is no longer running, cannot upload file: %s", build.Name, build.Status.Phase))
	}

	// The container should be the default build container, so setting it to blank
	buildPodName := buildutil.GetBuildPodName(build)
	opts := &kapi.PodAttachOptions{
		Stdin: true,
	}
	location, transport, err := pod.AttachLocation(h.r.PodGetter, h.r.ConnectionInfo, h.ctx, buildPodName, opts)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFound(kapi.Resource("pod"), buildPodName)
		}
		return nil, errors.NewBadRequest(err.Error())
	}
	rawTransport, ok := transport.(*http.Transport)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("unable to connect to node, unrecognized type: %v", reflect.TypeOf(transport)))
	}
	upgrader := spdy.NewRoundTripper(rawTransport.TLSClientConfig)
	exec, err := remotecommand.NewStreamExecutor(upgrader, nil, "POST", location)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("unable to connect to server: %v", err))
	}
	if err := exec.Stream(r, nil, nil, false); err != nil {
		return nil, errors.NewInternalError(err)
	}
	return latest, nil
}

type podGetter struct {
	podsNamespacer unversioned.PodsNamespacer
}

func (g *podGetter) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ns, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("namespace parameter required.")
	}
	return g.podsNamespacer.Pods(ns).Get(name)
}
