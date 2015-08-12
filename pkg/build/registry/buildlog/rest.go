package buildlog

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	genericrest "k8s.io/kubernetes/pkg/registry/generic/rest"
	"k8s.io/kubernetes/pkg/registry/pod"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/build"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	BuildRegistry  build.Registry
	PodGetter      pod.ResourceGetter
	ConnectionInfo kclient.ConnectionInfoGetter
	Timeout        time.Duration
}

type podGetter struct {
	podsNamespacer kclient.PodsNamespacer
}

func (g *podGetter) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ns, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("namespace parameter required.")
	}
	return g.podsNamespacer.Pods(ns).Get(name)
}

const defaultTimeout time.Duration = 10 * time.Second

// NewREST creates a new REST for BuildLog
// Takes build registry and pod client to get necessary attributes to assemble
// URL to which the request shall be redirected in order to get build logs.
func NewREST(b build.Registry, pn kclient.PodsNamespacer, connectionInfo kclient.ConnectionInfoGetter) *REST {
	return &REST{
		BuildRegistry:  b,
		PodGetter:      &podGetter{pn},
		ConnectionInfo: connectionInfo,
		Timeout:        defaultTimeout,
	}
}

var _ = rest.GetterWithOptions(&REST{})

// Get returns a streamer resource with the contents of the build log
func (r *REST) Get(ctx kapi.Context, name string, opts runtime.Object) (runtime.Object, error) {
	buildLogOpts, ok := opts.(*api.BuildLogOptions)
	if !ok {
		return nil, errors.NewBadRequest("did not get an expected options.")
	}
	build, err := r.BuildRegistry.GetBuild(ctx, name)
	if err != nil {
		return nil, errors.NewNotFound("build", name)
	}
	switch build.Status.Phase {
	// Build has not launched, wait til it runs
	case api.BuildPhaseNew, api.BuildPhasePending:
		if buildLogOpts.NoWait {
			glog.V(4).Infof("Build %s/%s is in %s state. No logs to retrieve yet.", build.Namespace, name, build.Status.Phase)
			// return empty content if not waiting for build
			return &genericrest.LocationStreamer{}, nil
		}
		glog.V(4).Infof("Build %s/%s is in %s state, waiting for Build to start", build.Namespace, name, build.Status.Phase)
		err := r.waitForBuild(ctx, build)
		if err != nil {
			return nil, err
		}

	// The build was cancelled
	case api.BuildPhaseCancelled:
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s/%s was cancelled. %s", build.Namespace, build.Name, buildutil.NoBuildLogsMessage))

	// An error occurred launching the build, return an error
	case api.BuildPhaseError:
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s/%s is in an error state. %s", build.Namespace, build.Name, buildutil.NoBuildLogsMessage))
	}
	// The container should be the default build container, so setting it to blank
	buildPodName := buildutil.GetBuildPodName(build)
	logOpts := &kapi.PodLogOptions{
		Follow: buildLogOpts.Follow,
	}
	location, transport, err := pod.LogLocation(r.PodGetter, r.ConnectionInfo, ctx, buildPodName, logOpts)
	if err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}
	return &genericrest.LocationStreamer{
		Location:    location,
		Transport:   transport,
		ContentType: "text/plain",
		Flush:       buildLogOpts.Follow,
	}, nil
}

func (r *REST) waitForBuild(ctx kapi.Context, build *api.Build) error {
	fieldSelector := fields.Set{"metadata.name": build.Name}.AsSelector()
	w, err := r.BuildRegistry.WatchBuilds(ctx, labels.Everything(), fieldSelector, build.ResourceVersion)
	if err != nil {
		return err
	}
	defer w.Stop()
	done := make(chan struct{})
	errchan := make(chan error)
	go func(ch <-chan watch.Event) {
		for event := range ch {
			obj, ok := event.Object.(*api.Build)
			if !ok {
				errchan <- fmt.Errorf("event object is not a Build: %#v", event.Object)
				break
			}
			switch obj.Status.Phase {
			case api.BuildPhaseCancelled:
				errchan <- fmt.Errorf("build %s/%s was cancelled. %s", build.Namespace, build.Name, buildutil.NoBuildLogsMessage)
				break
			case api.BuildPhaseError:
				errchan <- fmt.Errorf("build %s/%s is in an error state. %s", build.Namespace, build.Name, buildutil.NoBuildLogsMessage)
				break
			case api.BuildPhaseRunning, api.BuildPhaseComplete, api.BuildPhaseFailed:
				done <- struct{}{}
				break
			}
		}
	}(w.ResultChan())
	select {
	case err := <-errchan:
		return err
	case <-done:
		return nil
	case <-time.After(r.Timeout):
		return errors.NewTimeoutError(fmt.Sprintf("timed out waiting for Build %s/%s", build.Namespace, build.Name), 1)
	}
}

// NewGetOptions returns a new options object for build logs
func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &api.BuildLogOptions{}, false, ""
}

// New creates an empty BuildLog resource
func (r *REST) New() runtime.Object {
	return &api.BuildLog{}
}
