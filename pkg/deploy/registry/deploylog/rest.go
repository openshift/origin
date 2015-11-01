package deploylog

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	genericrest "k8s.io/kubernetes/pkg/registry/generic/rest"
	"k8s.io/kubernetes/pkg/registry/pod"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
	"github.com/openshift/origin/pkg/deploy/registry"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// defaultTimeout is the default time to wait for the logs of a deployment
const defaultTimeout time.Duration = 10 * time.Second

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	ConfigGetter     client.DeploymentConfigsNamespacer
	DeploymentGetter kclient.ReplicationControllersNamespacer
	PodGetter        pod.ResourceGetter
	ConnectionInfo   kclient.ConnectionInfoGetter
	Timeout          time.Duration
}

// REST implements GetterWithOptions
var _ = rest.GetterWithOptions(&REST{})

// NewREST creates a new REST for DeploymentLogs. It uses three clients: one for configs,
// one for deployments (replication controllers) and one for pods to get the necessary
// attributes to assemble the URL to which the request shall be redirected in order to
// get the deployment logs.
func NewREST(dn client.DeploymentConfigsNamespacer, rn kclient.ReplicationControllersNamespacer, pn kclient.PodsNamespacer, connectionInfo kclient.ConnectionInfoGetter) *REST {
	return &REST{
		ConfigGetter:     dn,
		DeploymentGetter: rn,
		PodGetter:        &podGetter{pn},
		ConnectionInfo:   connectionInfo,
		Timeout:          defaultTimeout,
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
		return nil, errors.NewInvalid("deploymentLogOptions", "", errs)
	}

	// Fetch deploymentConfig and check latest version; if 0, there are no deployments
	// for this config
	config, err := r.ConfigGetter.DeploymentConfigs(namespace).Get(name)
	if err != nil {
		return nil, errors.NewNotFound("deploymentConfig", name)
	}
	desiredVersion := config.LatestVersion
	if desiredVersion == 0 {
		return nil, errors.NewBadRequest(fmt.Sprintf("no deployment exists for deploymentConfig %q", config.Name))
	}

	// Support retrieving logs for older deployments
	switch {
	case deployLogOpts.Version == nil:
		// Latest
	case *deployLogOpts.Version <= 0 || int(*deployLogOpts.Version) > config.LatestVersion:
		// Invalid version
		return nil, errors.NewBadRequest(fmt.Sprintf("invalid version for deploymentConfig %q: %d", config.Name, *deployLogOpts.Version))
	default:
		desiredVersion = int(*deployLogOpts.Version)
	}

	// Get desired deployment
	targetName := deployutil.DeploymentNameForConfigVersion(config.Name, desiredVersion)
	target, err := r.DeploymentGetter.ReplicationControllers(namespace).Get(targetName)
	if err != nil {
		return nil, err
	}

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

		latest, ok, err := registry.WaitForRunningDeployment(r.DeploymentGetter, target, r.Timeout)
		if err != nil {
			return nil, errors.NewBadRequest(fmt.Sprintf("unable to wait for deployment %s to run: %v", deployutil.LabelForDeployment(target), err))
		}
		if !ok {
			return nil, errors.NewTimeoutError(fmt.Sprintf("timed out waiting for deployment %s to start after %s", deployutil.LabelForDeployment(target), r.Timeout), 1)
		}
		if deployutil.DeploymentStatusFor(latest) == deployapi.DeploymentStatusComplete {
			// Deployer pod has been deleted, no logs to retrieve
			glog.V(4).Infof("Deployment %s was successful so the deployer pod is deleted. No logs to retrieve.", deployutil.LabelForDeployment(target))
			return &genericrest.LocationStreamer{}, nil
		}
	case deployapi.DeploymentStatusComplete:
		// Deployer pod has been deleted, no logs to retrieve
		glog.V(4).Infof("Deployment %s was successful so the deployer pod is deleted. No logs to retrieve.", deployutil.LabelForDeployment(target))
		return &genericrest.LocationStreamer{}, nil
	}

	// Setup url of the deployer pod
	deployPodName := deployutil.DeployerPodNameForDeployment(target.Name)
	logOpts := deployapi.DeploymentToPodLogOptions(deployLogOpts)
	location, transport, err := pod.LogLocation(r.PodGetter, r.ConnectionInfo, ctx, deployPodName, logOpts)
	if err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}

	return &genericrest.LocationStreamer{
		Location:    location,
		Transport:   transport,
		ContentType: "text/plain",
		Flush:       deployLogOpts.Follow,
	}, nil
}

// podGetter implements the ResourceGetter interface. Used by LogLocation to
// retrieve the deployer pod
type podGetter struct {
	podsNamespacer kclient.PodsNamespacer
}

// Get is responsible for retrieving the deployer pod
func (g *podGetter) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	namespace, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, errors.NewBadRequest("namespace parameter required.")
	}
	return g.podsNamespacer.Pods(namespace).Get(name)
}
