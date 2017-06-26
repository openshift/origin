package buildlog

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericrest "k8s.io/apiserver/pkg/registry/generic/rest"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/registry/core/pod"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/apis/build/validation"
	"github.com/openshift/origin/pkg/build/registry"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	Getter         rest.Getter
	Watcher        rest.Watcher
	PodGetter      pod.ResourceGetter
	ConnectionInfo kubeletclient.ConnectionInfoGetter
	Timeout        time.Duration
}

type podGetter struct {
	kcoreclient.PodsGetter
}

func (g *podGetter) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("namespace parameter required.")
	}
	return g.Pods(ns).Get(name, *options)
}

const defaultTimeout time.Duration = 10 * time.Second

// NewREST creates a new REST for BuildLog
// Takes build registry and pod client to get necessary attributes to assemble
// URL to which the request shall be redirected in order to get build logs.
func NewREST(getter rest.Getter, watcher rest.Watcher, pn kcoreclient.PodsGetter, connectionInfo kubeletclient.ConnectionInfoGetter) *REST {
	return &REST{
		Getter:         getter,
		Watcher:        watcher,
		PodGetter:      &podGetter{pn},
		ConnectionInfo: connectionInfo,
		Timeout:        defaultTimeout,
	}
}

var _ = rest.GetterWithOptions(&REST{})

// Get returns a streamer resource with the contents of the build log
func (r *REST) Get(ctx apirequest.Context, name string, opts runtime.Object) (runtime.Object, error) {
	buildLogOpts, ok := opts.(*buildapi.BuildLogOptions)
	if !ok {
		return nil, errors.NewBadRequest("did not get an expected options.")
	}
	if errs := validation.ValidateBuildLogOptions(buildLogOpts); len(errs) > 0 {
		return nil, errors.NewInvalid(buildapi.Kind("BuildLogOptions"), "", errs)
	}
	obj, err := r.Getter.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	build := obj.(*buildapi.Build)
	if buildLogOpts.Previous {
		version := buildutil.VersionForBuild(build)
		// Use the previous version
		version--
		previousBuildName := buildutil.BuildNameForConfigVersion(buildutil.ConfigNameForBuild(build), version)
		previous, err := r.Getter.Get(ctx, previousBuildName, &metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		build = previous.(*buildapi.Build)
	}
	switch build.Status.Phase {
	// Build has not launched, wait til it runs
	case buildapi.BuildPhaseNew, buildapi.BuildPhasePending:
		if buildLogOpts.NoWait {
			glog.V(4).Infof("Build %s/%s is in %s state. No logs to retrieve yet.", build.Namespace, build.Name, build.Status.Phase)
			// return empty content if not waiting for build
			return &genericrest.LocationStreamer{}, nil
		}
		glog.V(4).Infof("Build %s/%s is in %s state, waiting for Build to start", build.Namespace, build.Name, build.Status.Phase)
		latest, ok, err := registry.WaitForRunningBuild(r.Watcher, ctx, build, r.Timeout)
		if err != nil {
			return nil, errors.NewBadRequest(fmt.Sprintf("unable to wait for build %s to run: %v", build.Name, err))
		}
		switch latest.Status.Phase {
		case buildapi.BuildPhaseError:
			return nil, errors.NewBadRequest(fmt.Sprintf("build %s encountered an error: %s", build.Name, buildutil.NoBuildLogsMessage))
		case buildapi.BuildPhaseCancelled:
			return nil, errors.NewBadRequest(fmt.Sprintf("build %s was cancelled: %s", build.Name, buildutil.NoBuildLogsMessage))
		}
		if !ok {
			return nil, errors.NewTimeoutError(fmt.Sprintf("timed out waiting for build %s to start after %s", build.Name, r.Timeout), 1)
		}

	// The build was cancelled
	case buildapi.BuildPhaseCancelled:
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s was cancelled. %s", build.Name, buildutil.NoBuildLogsMessage))

	// An error occurred launching the build, return an error
	case buildapi.BuildPhaseError:
		return nil, errors.NewBadRequest(fmt.Sprintf("build %s is in an error state. %s", build.Name, buildutil.NoBuildLogsMessage))
	}
	// The container should be the default build container, so setting it to blank
	buildPodName := buildapi.GetBuildPodName(build)
	logOpts := buildapi.BuildToPodLogOptions(buildLogOpts)
	location, transport, err := pod.LogLocation(r.PodGetter, r.ConnectionInfo, ctx, buildPodName, logOpts)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFound(kapi.Resource("pod"), buildPodName)
		}
		return nil, errors.NewBadRequest(err.Error())
	}
	return &genericrest.LocationStreamer{
		Location:        location,
		Transport:       transport,
		ContentType:     "text/plain",
		Flush:           buildLogOpts.Follow,
		ResponseChecker: genericrest.NewGenericHttpResponseChecker(kapi.Resource("pod"), buildPodName),
	}, nil
}

// NewGetOptions returns a new options object for build logs
func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &buildapi.BuildLogOptions{}, false, ""
}

// New creates an empty BuildLog resource
func (r *REST) New() runtime.Object {
	return &buildapi.BuildLog{}
}
