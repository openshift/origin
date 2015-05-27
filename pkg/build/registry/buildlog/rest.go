package buildlog

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	genericrest "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/pod"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

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
		return nil, errors.NewBadRequest("Namespace parameter required.")
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
	switch build.Status {
	// Build has not launched, wait til it runs
	case api.BuildStatusNew, api.BuildStatusPending:
		if buildLogOpts.NoWait {
			glog.V(4).Infof("Build %s/%s is in %s state, nothing to retrieve", build.Namespace, name, build.Status)
			// return empty content if not waiting for build
			return &genericrest.LocationStreamer{}, nil
		}
		glog.V(4).Infof("Build %s/%s is in %s state, waiting for Build to start", build.Namespace, name, build.Status)
		err := r.waitForBuild(ctx, build)
		if err != nil {
			return nil, err
		}

	// The build was cancelled
	case api.BuildStatusCancelled:
		return nil, fmt.Errorf("Build %s/%s was cancelled", build.Namespace, build.Name)

	// An error occurred launching the build, return an error
	case api.BuildStatusError:
		return nil, fmt.Errorf("Build %s/%s is in an error state", build.Namespace, build.Name)
	}
	// The container should be the default build container, so setting it to blank
	buildPodName := buildutil.GetBuildPodName(build)
	logOpts := &kapi.PodLogOptions{
		Follow: buildLogOpts.Follow,
	}
	location, transport, err := pod.LogLocation(r.PodGetter, r.ConnectionInfo, ctx, buildPodName, logOpts)
	if err != nil {
		return nil, err
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
			switch obj.Status {
			case api.BuildStatusCancelled:
				errchan <- fmt.Errorf("Build %s/%s was cancelled", build.Namespace, build.Name)
				break
			case api.BuildStatusError:
				errchan <- fmt.Errorf("Build %s/%s is in an error state", build.Namespace, build.Name)
				break
			case api.BuildStatusRunning, api.BuildStatusComplete, api.BuildStatusFailed:
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
		return fmt.Errorf("timed out waiting for Build %s/%s", build.Namespace, build.Name)
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
