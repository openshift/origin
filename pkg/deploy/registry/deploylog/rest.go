package deploylog

import (
	"fmt"
	"sort"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/labels"
	genericrest "k8s.io/kubernetes/pkg/registry/generic/rest"
	"k8s.io/kubernetes/pkg/registry/pod"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
	"github.com/openshift/origin/pkg/deploy/registry"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

const (
	// defaultTimeout is the default time to wait for the logs of a deployment.
	defaultTimeout time.Duration = 60 * time.Second
	// defaultInterval is the default interval for polling a not found deployment.
	defaultInterval time.Duration = 1 * time.Second
)

// podGetter implements the ResourceGetter interface. Used by LogLocation to
// retrieve the deployer pod
type podGetter struct {
	pn unversioned.PodsNamespacer
}

// Get is responsible for retrieving the deployer pod
func (g *podGetter) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	namespace, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("namespace parameter required.")
	}
	return g.pn.Pods(namespace).Get(name)
}

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	dn       client.DeploymentConfigsNamespacer
	rn       unversioned.ReplicationControllersNamespacer
	pn       unversioned.PodsNamespacer
	connInfo kubeletclient.ConnectionInfoGetter
	timeout  time.Duration
	interval time.Duration
}

// REST implements GetterWithOptions
var _ = rest.GetterWithOptions(&REST{})

// NewREST creates a new REST for DeploymentLogs. It uses three clients: one for configs,
// one for deployments (replication controllers) and one for pods to get the necessary
// attributes to assemble the URL to which the request shall be redirected in order to
// get the deployment logs.
func NewREST(dn client.DeploymentConfigsNamespacer, rn unversioned.ReplicationControllersNamespacer, pn unversioned.PodsNamespacer, connectionInfo kubeletclient.ConnectionInfoGetter) *REST {
	return &REST{
		dn:       dn,
		rn:       rn,
		pn:       pn,
		connInfo: connectionInfo,
		timeout:  defaultTimeout,
		interval: defaultInterval,
	}
}

// NewGetOptions returns a new options object for deployment logs
func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &deployapi.DeploymentLogOptions{}, false, ""
}

// New creates an empty DeploymentLog resource
func (r *REST) New() runtime.Object {
	return &deployapi.DeploymentLog{}
}

// Get returns a streamer resource with the contents of the deployment log
func (r *REST) Get(ctx kapi.Context, name string, opts runtime.Object) (runtime.Object, error) {
	// Ensure we have a namespace in the context
	namespace, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("namespace parameter required.")
	}

	// Validate DeploymentLogOptions
	deployLogOpts, ok := opts.(*deployapi.DeploymentLogOptions)
	if !ok {
		return nil, errors.NewBadRequest("did not get an expected options.")
	}
	if errs := validation.ValidateDeploymentLogOptions(deployLogOpts); len(errs) > 0 {
		return nil, errors.NewInvalid(deployapi.Kind("DeploymentLogOptions"), "", errs)
	}

	// Fetch deploymentConfig and check latest version; if 0, there are no deployments
	// for this config
	config, err := r.dn.DeploymentConfigs(namespace).Get(name)
	if err != nil {
		return nil, errors.NewNotFound(deployapi.Resource("deploymentconfig"), name)
	}
	desiredVersion := config.Status.LatestVersion
	if desiredVersion == 0 {
		return nil, errors.NewBadRequest(fmt.Sprintf("no deployment exists for deploymentConfig %q", config.Name))
	}

	// Support retrieving logs for older deployments
	switch {
	case deployLogOpts.Version == nil:
		// Latest or previous
		if deployLogOpts.Previous {
			desiredVersion--
			if desiredVersion < 1 {
				return nil, errors.NewBadRequest(fmt.Sprintf("no previous deployment exists for deploymentConfig %q", config.Name))
			}
		}
	case *deployLogOpts.Version <= 0 || *deployLogOpts.Version > config.Status.LatestVersion:
		// Invalid version
		return nil, errors.NewBadRequest(fmt.Sprintf("invalid version for deploymentConfig %q: %d", config.Name, *deployLogOpts.Version))
	default:
		desiredVersion = *deployLogOpts.Version
	}

	// Get desired deployment
	targetName := deployutil.DeploymentNameForConfigVersion(config.Name, desiredVersion)
	target, err := r.waitForExistingDeployment(namespace, targetName)
	if err != nil {
		return nil, err
	}
	podName := deployutil.DeployerPodNameForDeployment(target.Name)

	// Check for deployment status; if it is new or pending, we will wait for it. If it is complete,
	// the deployment completed successfully and the deployer pod will be deleted so we will return a
	// success message. If it is running or failed, retrieve the log from the deployer pod.
	status := deployutil.DeploymentStatusFor(target)
	switch status {
	case deployapi.DeploymentStatusNew, deployapi.DeploymentStatusPending:
		if deployLogOpts.NoWait {
			glog.V(4).Infof("Deployment %s is in %s state. No logs to retrieve yet.", deployutil.LabelForDeployment(target), status)
			return &genericrest.LocationStreamer{}, nil
		}
		glog.V(4).Infof("Deployment %s is in %s state, waiting for it to start...", deployutil.LabelForDeployment(target), status)

		if err := deployutil.WaitForRunningDeployerPod(r.pn, target, r.timeout); err != nil {
			return nil, errors.NewBadRequest(fmt.Sprintf("failed to run deployer pod %s: %v", podName, err))
		}

		latest, ok, err := registry.WaitForRunningDeployment(r.rn, target, r.timeout)
		if err != nil {
			return nil, errors.NewBadRequest(fmt.Sprintf("unable to wait for deployment %s to run: %v", deployutil.LabelForDeployment(target), err))
		}
		if !ok {
			return nil, errors.NewServerTimeout(kapi.Resource("ReplicationController"), "get", 2)
		}
		if deployutil.IsCompleteDeployment(latest) {
			podName, err = r.returnApplicationPodName(target)
			if err != nil {
				return nil, err
			}
		}
	case deployapi.DeploymentStatusComplete:
		podName, err = r.returnApplicationPodName(target)
		if err != nil {
			return nil, err
		}
	}

	logOpts := deployapi.DeploymentToPodLogOptions(deployLogOpts)
	location, transport, err := pod.LogLocation(&podGetter{r.pn}, r.connInfo, ctx, podName, logOpts)
	if err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}

	return &genericrest.LocationStreamer{
		Location:        location,
		Transport:       transport,
		ContentType:     "text/plain",
		Flush:           deployLogOpts.Follow,
		ResponseChecker: genericrest.NewGenericHttpResponseChecker(kapi.Resource("pod"), podName),
	}, nil
}

// waitForExistingDeployment will use the timeout to wait for a deployment to appear.
func (r *REST) waitForExistingDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	var (
		target *kapi.ReplicationController
		err    error
	)

	condition := func() (bool, error) {
		target, err = r.rn.ReplicationControllers(namespace).Get(name)
		switch {
		case errors.IsNotFound(err):
			return false, nil
		case err != nil:
			return false, err
		}
		return true, nil
	}

	err = wait.PollImmediate(r.interval, r.timeout, condition)
	if err == wait.ErrWaitTimeout {
		err = errors.NewNotFound(kapi.Resource("replicationcontrollers"), name)
	}
	return target, err
}

// returnApplicationPodName returns the best candidate pod for the target deployment in order to
// view its logs.
func (r *REST) returnApplicationPodName(target *kapi.ReplicationController) (string, error) {
	selector := labels.Set(target.Spec.Selector).AsSelector()
	sortBy := func(pods []*kapi.Pod) sort.Interface { return controller.ByLogging(pods) }

	pod, _, err := kcmdutil.GetFirstPod(r.pn, target.Namespace, selector, r.timeout, sortBy)
	if err != nil {
		return "", errors.NewInternalError(err)
	}
	return pod.Name, nil
}
