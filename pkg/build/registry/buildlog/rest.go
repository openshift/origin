package buildlog

import (
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/util/wait"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericrest "k8s.io/apiserver/pkg/registry/generic/rest"
	"k8s.io/apiserver/pkg/registry/rest"

	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/registry/core/pod"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/apis/build/validation"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildtypedclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	"github.com/openshift/origin/pkg/build/registry"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	BuildClient    buildtypedclient.BuildsGetter
	PodGetter      pod.ResourceGetter
	ConnectionInfo kubeletclient.ConnectionInfoGetter
	Timeout        time.Duration
}

// TODO these wrapers shouldb e removed
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
func NewREST(buildClient buildtypedclient.BuildsGetter, pn kcoreclient.PodsGetter, connectionInfo kubeletclient.ConnectionInfoGetter) *REST {
	return &REST{
		BuildClient:    buildClient,
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
	build, err := r.BuildClient.Builds(apirequest.NamespaceValue(ctx)).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if buildLogOpts.Previous {
		version := buildutil.VersionForBuild(build)
		// Use the previous version
		version--
		previousBuildName := buildutil.BuildNameForConfigVersion(buildutil.ConfigNameForBuild(build), version)
		previous, err := r.BuildClient.Builds(apirequest.NamespaceValue(ctx)).Get(previousBuildName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		build = previous
	}
	switch build.Status.Phase {
	// Build has not launched, wait until it runs
	case buildapi.BuildPhaseNew, buildapi.BuildPhasePending:
		if buildLogOpts.NoWait {
			glog.V(4).Infof("Build %s/%s is in %s state. No logs to retrieve yet.", build.Namespace, build.Name, build.Status.Phase)
			// return empty content if not waiting for build
			return &genericrest.LocationStreamer{}, nil
		}
		glog.V(4).Infof("Build %s/%s is in %s state, waiting for Build to start", build.Namespace, build.Name, build.Status.Phase)
		latest, ok, err := registry.WaitForRunningBuild(r.BuildClient, build, r.Timeout)
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

	// if we can't at least get the build pod, we're not going to get very far, so
	// error out now.
	obj, err := r.PodGetter.Get(ctx, buildPodName, &metav1.GetOptions{})
	if err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}

	// check for old style builds with a single container/no initcontainers
	// and handle them w/ the old logging code.
	buildPod := obj.(*kapi.Pod)
	if len(buildPod.Spec.InitContainers) == 0 {
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

	// new style builds w/ init containers from here out.

	// we'll funnel all the initcontainer+main container logs into this single stream
	reader, writer := io.Pipe()
	pipeStreamer := PipeStreamer{
		In:          writer,
		Out:         reader,
		Flush:       buildLogOpts.Follow,
		ContentType: "text/plain",
	}

	// background thread will poll the init containers until they are running/terminated
	// and then stream the logs from them into the pipe, one by one, before streaming
	// the primary container logs into the pipe.  Any errors that occur will result
	// in a premature return and aborted log stream.
	go func() {
		defer pipeStreamer.In.Close()

		// containers that we've successfully streamed the logs for and don't need
		// to worry about it anymore.
		doneWithContainer := map[string]bool{}
		// check all the init containers for logs at least once.
		waitForInitContainers := true
		// once we see an init container that failed, stop iterating the containers
		// because no additional init containers will run.
		initFailed := false
		// sleep in between rounds of iterating the containers unless we successfully
		// streamed something in which case it makes sense to immediately proceed to
		// checking if another init container is ready (or we're done with all initcontainers)
		sleep := true
		// If we are following the logs, keep iterating through the init containers
		// until they have all run, unless we see one of them fail.
		// If we aren't following the logs, we will run through all the init containers exactly once.
		for waitForInitContainers {
			select {
			case <-ctx.Done():
				glog.V(4).Infof("timed out while iterating on build init containers for build pod %s/%s", build.Namespace, buildPodName)
				return
			default:
			}
			glog.V(4).Infof("iterating through build init containers for build pod %s/%s", build.Namespace, buildPodName)

			// assume we are not going to need to iterate again until proven otherwise
			waitForInitContainers = false
			// Get the latest version of the pod so we can check init container statuses
			obj, err = r.PodGetter.Get(ctx, buildPodName, &metav1.GetOptions{})
			if err != nil {
				s := fmt.Sprintf("error retrieving build pod %s/%s : %v", build.Namespace, buildPodName, err.Error())
				// we're sending the error message as the log output so the user at least sees some indication of why
				// they didn't get the logs they expected.
				pipeStreamer.In.Write([]byte(s))
				return
			}
			buildPod = obj.(*kapi.Pod)

			// Walk all the initcontainers in order and dump/stream their logs.  The initcontainers
			// are defined in order of execution, so that's the order we'll dump their logs in.
			for _, status := range buildPod.Status.InitContainerStatuses {
				glog.V(4).Infof("processing build pod: %s/%s container: %s in state %#v", build.Namespace, buildPodName, status.Name, status.State)

				if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
					initFailed = true
					// if we see a failed init container, don't continue waiting for additional init containers
					// as they will never run, but we do need to dump/stream the logs for this container so
					// we won't break out of the loop here, we just won't re-enter the loop.
					waitForInitContainers = false

					// if we've already dealt with the logs for this container we are done.
					// We might have streamed the logs for it last time around, but we wouldn't see that it
					// terminated with a failure until now.
					if doneWithContainer[status.Name] {
						break
					}
				}

				// if we've already dumped the logs for this init container, ignore it.
				if doneWithContainer[status.Name] {
					continue
				}

				// ignore containers in a waiting state(they have no logs yet), but
				// flag that we need to keep iterating while we wait for it to run.
				if status.State.Waiting != nil {
					waitForInitContainers = true
					continue
				}

				// get the container logstream for this init container
				containerLogOpts := buildapi.BuildToPodLogOptions(buildLogOpts)
				containerLogOpts.Container = status.Name
				// never "follow" logs for terminated containers, it causes latency in streaming the result
				// and there's no point since the log is complete already.
				if status.State.Terminated != nil {
					containerLogOpts.Follow = false
				}

				if err := r.pipeLogs(ctx, build.Namespace, buildPodName, containerLogOpts, pipeStreamer.In); err != nil {
					glog.Errorf("error: failed to stream logs for build pod: %s/%s container: %s, due to: %v", build.Namespace, buildPodName, status.Name, err)
					return
				}

				// if we successfully streamed anything, don't wait before the next iteration
				// of init container checking/streaming.
				sleep = false

				// we are done with this container, we can ignore on future iterations.
				doneWithContainer[status.Name] = true

				// no point in processing more init containers once we've seen one that failed,
				// no additional init containers will run.
				if initFailed {
					break
				}
			} // loop over all the initcontainers

			// if we're not in log follow mode, don't keep waiting on container logs once
			// we've iterated all the init containers once.
			if !buildLogOpts.Follow {
				break
			}
			// don't iterate too quickly waiting for the next init container to run unless
			// we did some log streaming during this iteration.
			if sleep {
				time.Sleep(time.Second)
			}

			// loop over the pod until we've seen all the initcontainers enter the running state and
		} // streamed their logs, or seen a failed initcontainer, or we weren't in follow mode.

		// done handling init container logs, get the main container logs, unless
		// an init container failed, in which case there will be no main container logs and we're done.
		if !initFailed {

			// Wait for the main container to be running, this can take a second after the initcontainers
			// finish so we have to poll.
			err := wait.PollImmediate(time.Second, 10*time.Minute, func() (bool, error) {
				obj, err = r.PodGetter.Get(ctx, buildPodName, &metav1.GetOptions{})
				if err != nil {
					s := fmt.Sprintf("error while getting build logs, could not retrieve build pod %s/%s : %v", build.Namespace, buildPodName, err.Error())
					pipeStreamer.In.Write([]byte(s))
					return false, err
				}
				buildPod = obj.(*kapi.Pod)
				// we can get logs from a pod in any state other than pending.
				if buildPod.Status.Phase != kapi.PodPending {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				glog.Errorf("error: failed to stream logs for build pod: %s/%s due to: %v", build.Namespace, buildPodName, err)
				return
			}

			containerLogOpts := buildapi.BuildToPodLogOptions(buildLogOpts)
			containerLogOpts.Container = selectBuilderContainer(buildPod.Spec.Containers)
			if containerLogOpts.Container == "" {
				glog.Errorf("error: failed to select a container in build pod: %s/%s", build.Namespace, buildPodName)
			}

			// never follow logs for terminated pods, it just causes latency in streaming the result.
			if buildPod.Status.Phase == kapi.PodFailed || buildPod.Status.Phase == kapi.PodSucceeded {
				containerLogOpts.Follow = false
			}

			if err := r.pipeLogs(ctx, build.Namespace, buildPodName, containerLogOpts, pipeStreamer.In); err != nil {
				glog.Errorf("error: failed to stream logs for build pod: %s/%s due to: %v", build.Namespace, buildPodName, err)
				return
			}
		}
	}()

	return &pipeStreamer, nil
}

// NewGetOptions returns a new options object for build logs
func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &buildapi.BuildLogOptions{}, false, ""
}

// New creates an empty BuildLog resource
func (r *REST) New() runtime.Object {
	return &buildapi.BuildLog{}
}

// pipeLogs retrieves the logs for a particular container and streams them into the provided writer.
func (r *REST) pipeLogs(ctx apirequest.Context, namespace, buildPodName string, containerLogOpts *kapi.PodLogOptions, writer io.Writer) error {
	glog.V(4).Infof("pulling build pod logs for %s/%s, container %s", namespace, buildPodName, containerLogOpts.Container)
	location, transport, err := pod.LogLocation(r.PodGetter, r.ConnectionInfo, ctx, buildPodName, containerLogOpts)
	if err != nil {
		s := fmt.Sprintf("error retrieving logs for build pod: %s/%s container: %s, %v", namespace, buildPodName, containerLogOpts.Container, err.Error())
		_, err2 := writer.Write([]byte(s))
		if err2 != nil {
			glog.Errorf("error: could not write build log error %q to stream due to: %v", s, err2)
		}
		return err
	}
	locationStreamer := genericrest.LocationStreamer{
		Location:        location,
		Transport:       transport,
		ContentType:     "text/plain",
		Flush:           true,
		ResponseChecker: genericrest.NewGenericHttpResponseChecker(kapi.Resource("pod"), buildPodName),
	}
	out, _, _, err := locationStreamer.InputStream("", "")
	if err != nil {
		s := fmt.Sprintf("error streaming logs from build pod: %s/%s container: %s, %v", namespace, buildPodName, containerLogOpts.Container, err.Error())
		_, err2 := writer.Write([]byte(s))
		if err2 != nil {
			glog.Errorf("error: could not write build log error %q to stream due to: %v", s, err2)
		}
		return err
	}
	if out == nil {
		glog.Warningf("warning: logs for build pod: %s/%s container: %s returned a nil stream", namespace, buildPodName, containerLogOpts.Container)
		return nil
	}
	glog.V(4).Infof("retrieved logs for build pod: %s/%s container: %s", namespace, buildPodName, containerLogOpts.Container)
	// dump all container logs from the log stream into a single output stream that we'll send back to the client.
	_, err = io.Copy(writer, out)
	return err
}

// 3rd party tools, such as istio auto-inject, may add sidecar containers to
// the build pod. We are interested in logs from the build container only
func selectBuilderContainer(containers []kapi.Container) string {
	for _, c := range containers {
		for _, bcName := range buildstrategy.BuildContainerNames {
			if c.Name == bcName {
				return bcName
			}
		}
	}
	return ""
}
