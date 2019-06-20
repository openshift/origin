package deploylog

import (
	"context"
	"fmt"
	"sort"
	"time"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericrest "k8s.io/apiserver/pkg/registry/generic/rest"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/openshift/api/apps"
	appsv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	"github.com/openshift/library-go/pkg/apps/appsutil"
	apiserverrest "github.com/openshift/openshift-apiserver/pkg/apiserver/rest"
	appsapi "github.com/openshift/openshift-apiserver/pkg/apps/apis/apps"
	"github.com/openshift/openshift-apiserver/pkg/apps/apis/apps/validation"
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
			klog.V(4).Infof("Deployment %s is in %s state. No logs to retrieve yet.", labelForDeployment, status)
			return &genericrest.LocationStreamer{}, nil
		}
		klog.V(4).Infof("Deployment %s is in %s state, waiting for it to start...", labelForDeployment, status)

		if err := appsutil.WaitForRunningDeployerPod(r.podClient, target, r.timeout); err != nil {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("failed to run deployer pod %s: %v", podName, err))
		}

		latest, err := WaitForRunningDeployment(r.rcClient, target, r.timeout)
		if err == wait.ErrWaitTimeout {
			return nil, apierrors.NewServerTimeout(kapi.Resource("ReplicationController"), "get", 2)
		}
		if err != nil {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("unable to wait for deployment %s to run: %v", labelForDeployment, err))
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
// DO NOT EDIT: this is a copy of the same function from kubectl to avoid carrying the dependency
func GetFirstPod(client corev1client.PodsGetter, namespace string, selector string, timeout time.Duration, sortBy func([]*corev1.Pod) sort.Interface) (*corev1.Pod, int, error) {
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = selector
			return client.Pods(namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = selector
			return client.Pods(namespace).Watch(options)
		},
	}

	var initialPods []*corev1.Pod
	preconditionFunc := func(store cache.Store) (bool, error) {
		items := store.List()
		if len(items) > 0 {
			for _, item := range items {
				pod, ok := item.(*corev1.Pod)
				if !ok {
					return true, fmt.Errorf("unexpected store item type: %#v", item)
				}

				initialPods = append(initialPods, pod)
			}

			sort.Sort(sortBy(initialPods))

			return true, nil
		}

		// Continue by watching for a new pod to appear
		return false, nil
	}

	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, preconditionFunc, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			// Any pod is good enough
			return true, nil

		case watch.Deleted:
			return true, fmt.Errorf("pod got deleted %#v", event.Object)

		case watch.Error:
			return true, fmt.Errorf("unexpected error %#v", event.Object)

		default:
			return true, fmt.Errorf("unexpected event type: %T", event.Type)
		}
	})
	if err != nil {
		return nil, 0, err
	}

	if len(initialPods) > 0 {
		return initialPods[0], len(initialPods), nil
	}

	pod, ok := event.Object.(*corev1.Pod)
	if !ok {
		return nil, 0, fmt.Errorf("%#v is not a pod event", event)
	}

	return pod, 1, nil
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
