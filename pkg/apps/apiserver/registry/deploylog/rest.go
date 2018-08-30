package deploylog

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericrest "k8s.io/apiserver/pkg/registry/generic/rest"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/openshift/api/apps"
	appsv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	apiserverrest "github.com/openshift/origin/pkg/apiserver/rest"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/apps/apis/apps/validation"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

const (
	// defaultTimeout is the default time to wait for the logs of a deployment.
	defaultTimeout = 60 * time.Second
	// defaultInterval is the default interval for polling a not found deployment.
	defaultInterval = 1 * time.Second
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	dcClient  appsclient.DeploymentConfigsGetter
	rcClient  corev1client.ReplicationControllersGetter
	podClient corev1client.PodsGetter
	timeout   time.Duration
	interval  time.Duration

	// for unit testing
	getLogsFn func(podNamespace, podName string, logOpts *corev1.PodLogOptions) (runtime.Object, error)
}

// REST implements GetterWithOptions
var _ = rest.GetterWithOptions(&REST{})

// NewREST creates a new REST for DeploymentLogs. It uses three clients: one for configs,
// one for deployments (replication controllers) and one for pods to get the necessary
// attributes to assemble the URL to which the request shall be redirected in order to
// get the deployment logs.
func NewREST(dcClient appsclient.DeploymentConfigsGetter, client kubernetes.Interface) *REST {
	r := &REST{
		dcClient:  dcClient,
		rcClient:  client.CoreV1(),
		podClient: client.CoreV1(),
		timeout:   defaultTimeout,
		interval:  defaultInterval,
	}
	r.getLogsFn = r.getLogs

	return r
}

// NewGetOptions returns a new options object for deployment logs
func (r *REST) NewGetOptions() (runtime.Object, bool, string) {
	return &appsapi.DeploymentLogOptions{}, false, ""
}

// New creates an empty DeploymentLog resource
func (r *REST) New() runtime.Object {
	return &appsapi.DeploymentLog{}
}

// Get returns a streamer resource with the contents of the deployment log
func (r *REST) Get(ctx context.Context, name string, opts runtime.Object) (runtime.Object, error) {
	// Ensure we have a namespace in the context
	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("namespace parameter required.")
	}

	// Validate DeploymentLogOptions
	deployLogOpts, ok := opts.(*appsapi.DeploymentLogOptions)
	if !ok {
		return nil, apierrors.NewBadRequest("did not get an expected options.")
	}
	if errs := validation.ValidateDeploymentLogOptions(deployLogOpts); len(errs) > 0 {
		return nil, apierrors.NewInvalid(apps.Kind("DeploymentLogOptions"), "", errs)
	}

	// Fetch deploymentConfig and check latest version; if 0, there are no deployments
	// for this config
	config, err := r.dcClient.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, apierrors.NewNotFound(apps.Resource("deploymentconfig"), name)
	}
	desiredVersion := config.Status.LatestVersion
	if desiredVersion == 0 {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("no deployment exists for deploymentConfig %q", config.Name))
	}

	// Support retrieving logs for older deployments
	switch {
	case deployLogOpts.Version == nil:
		// Latest or previous
		if deployLogOpts.Previous {
			desiredVersion--
			if desiredVersion < 1 {
				return nil, apierrors.NewBadRequest(fmt.Sprintf("no previous deployment exists for deploymentConfig %q", config.Name))
			}
		}
	case *deployLogOpts.Version <= 0 || *deployLogOpts.Version > config.Status.LatestVersion:
		// Invalid version
		return nil, apierrors.NewBadRequest(fmt.Sprintf("invalid version for deploymentConfig %q: %d", config.Name, *deployLogOpts.Version))
	default:
		desiredVersion = *deployLogOpts.Version
	}

	// Get desired deployment
	targetName := appsutil.DeploymentNameForConfigVersion(config.Name, desiredVersion)
	target, err := r.waitForExistingDeployment(namespace, targetName)
	if err != nil {
		return nil, err
	}
	podName := appsutil.DeployerPodNameForDeployment(target.Name)
	labelForDeployment := fmt.Sprintf("%s/%s", target.Namespace, target.Name)

	// Check for deployment status; if it is new or pending, we will wait for it. If it is complete,
	// the deployment completed successfully and the deployer pod will be deleted so we will return a
	// success message. If it is running or failed, retrieve the log from the deployer pod.
	status := appsutil.DeploymentStatusFor(target)
	switch status {
	case appsv1.DeploymentStatusNew, appsv1.DeploymentStatusPending:
		if deployLogOpts.NoWait {
			glog.V(4).Infof("Deployment %s is in %s state. No logs to retrieve yet.", labelForDeployment, status)
			return &genericrest.LocationStreamer{}, nil
		}
		glog.V(4).Infof("Deployment %s is in %s state, waiting for it to start...", labelForDeployment, status)

		if err := WaitForRunningDeployerPod(r.podClient, target, r.timeout); err != nil {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("failed to run deployer pod %s: %v", podName, err))
		}

		latest, ok, err := WaitForRunningDeployment(r.rcClient, target, r.timeout)
		if err != nil {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("unable to wait for deployment %s to run: %v", labelForDeployment, err))
		}
		if !ok {
			return nil, apierrors.NewServerTimeout(kapi.Resource("ReplicationController"), "get", 2)
		}
		if appsutil.IsCompleteDeployment(latest) {
			podName, err = r.returnApplicationPodName(target)
			if err != nil {
				return nil, err
			}
		}
	case appsv1.DeploymentStatusComplete:
		podName, err = r.returnApplicationPodName(target)
		if err != nil {
			return nil, err
		}
	}

	logOpts := DeploymentToPodLogOptions(deployLogOpts)
	return r.getLogsFn(namespace, podName, logOpts)
}

func (r *REST) getLogs(podNamespace, podName string, logOpts *corev1.PodLogOptions) (runtime.Object, error) {
	logRequest := r.podClient.Pods(podNamespace).GetLogs(podName, logOpts)

	readerCloser, err := logRequest.Stream()
	if err != nil {
		return nil, err
	}

	return &apiserverrest.PassThroughStreamer{
		In:          readerCloser,
		Flush:       logOpts.Follow,
		ContentType: "text/plain",
	}, nil
}

// waitForExistingDeployment will use the timeout to wait for a deployment to appear.
func (r *REST) waitForExistingDeployment(namespace, name string) (*corev1.ReplicationController, error) {
	var (
		target *corev1.ReplicationController
		err    error
	)

	condition := func() (bool, error) {
		target, err = r.rcClient.ReplicationControllers(namespace).Get(name, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			return false, nil
		case err != nil:
			return false, err
		}
		return true, nil
	}

	err = wait.PollImmediate(r.interval, r.timeout, condition)
	if err == wait.ErrWaitTimeout {
		err = apierrors.NewNotFound(kapi.Resource("replicationcontrollers"), name)
	}
	return target, err
}

// returnApplicationPodName returns the best candidate pod for the target deployment in order to
// view its logs.
func (r *REST) returnApplicationPodName(target *corev1.ReplicationController) (string, error) {
	selector := labels.SelectorFromValidatedSet(labels.Set(target.Spec.Selector))
	sortBy := func(pods []*corev1.Pod) sort.Interface { return controller.ByLogging(pods) }

	firstPod, _, err := GetFirstPod(r.podClient, target.Namespace, selector.String(), r.timeout, sortBy)
	if err != nil {
		return "", apierrors.NewInternalError(err)
	}
	return firstPod.Name, nil
}

// GetFirstPod returns a pod matching the namespace and label selector
// and the number of all pods that match the label selector.
func GetFirstPod(client corev1client.PodsGetter, namespace string, selector string, timeout time.Duration, sortBy func([]*corev1.Pod) sort.Interface) (*corev1.Pod, int, error) {
	options := metav1.ListOptions{LabelSelector: selector}

	podList, err := client.Pods(namespace).List(options)
	if err != nil {
		return nil, 0, err
	}
	pods := []*corev1.Pod{}
	for i := range podList.Items {
		pods = append(pods, &podList.Items[i])
	}
	if len(pods) > 0 {
		sort.Sort(sortBy(pods))
		return pods[0], len(podList.Items), nil
	}

	// Watch until we observe a pod
	options.ResourceVersion = podList.ResourceVersion
	w, err := client.Pods(namespace).Watch(options)
	if err != nil {
		return nil, 0, err
	}
	defer w.Stop()

	condition := func(event watch.Event) (bool, error) {
		return event.Type == watch.Added || event.Type == watch.Modified, nil
	}
	event, err := watch.Until(timeout, w, condition)
	if err != nil {
		return nil, 0, err
	}
	pod, ok := event.Object.(*corev1.Pod)
	if !ok {
		return nil, 0, fmt.Errorf("%#v is not a pod event", event)
	}
	return pod, 1, nil
}

// WaitForRunningDeployerPod waits a given period of time until the deployer pod
// for given replication controller is not running.
func WaitForRunningDeployerPod(podClient corev1client.PodsGetter, rc *corev1.ReplicationController, timeout time.Duration) error {
	podName := appsutil.DeployerPodNameForDeployment(rc.Name)
	canGetLogs := func(p *corev1.Pod) bool {
		return corev1.PodSucceeded == p.Status.Phase || corev1.PodFailed == p.Status.Phase || corev1.PodRunning == p.Status.Phase
	}
	pod, err := podClient.Pods(rc.Namespace).Get(podName, metav1.GetOptions{})
	if err == nil && canGetLogs(pod) {
		return nil
	}
	watcher, err := podClient.Pods(rc.Namespace).Watch(
		metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", podName).String(),
		},
	)
	if err != nil {
		return err
	}

	defer watcher.Stop()
	_, err = watch.Until(timeout, watcher, func(e watch.Event) (bool, error) {
		if e.Type == watch.Error {
			return false, fmt.Errorf("encountered error while watching for pod: %v", e.Object)
		}
		obj, isPod := e.Object.(*corev1.Pod)
		if !isPod {
			return false, errors.New("received unknown object while watching for pods")
		}
		return canGetLogs(obj), nil
	})
	return err
}

func DeploymentToPodLogOptions(opts *appsapi.DeploymentLogOptions) *corev1.PodLogOptions {
	return &corev1.PodLogOptions{
		Container:    opts.Container,
		Follow:       opts.Follow,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
	}
}
