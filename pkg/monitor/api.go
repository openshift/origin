package monitor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	configclientset "github.com/openshift/client-go/config/clientset/versioned"
	clientimagev1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
)

// Start begins monitoring the cluster referenced by the default kube configuration until
// context is finished.
func Start(ctx context.Context) (*Monitor, error) {
	m := NewMonitor()
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	clusterConfig, err := cfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not load client configuration: %v", err)
	}
	client, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return nil, err
	}
	configClient, err := configclientset.NewForConfig(clusterConfig)
	if err != nil {
		return nil, err
	}

	if err := StartAPIMonitoring(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	startPodMonitoring(ctx, m, client)
	startNodeMonitoring(ctx, m, client)
	startEventMonitoring(ctx, m, client)
	startClusterOperatorMonitoring(ctx, m, configClient)

	m.StartSampling(ctx)
	return m, nil
}

func StartAPIMonitoring(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	pollingConfig := *clusterConfig
	pollingConfig.Timeout = timeout
	pollingClient, err := clientcorev1.NewForConfig(&pollingConfig)
	if err != nil {
		return err
	}
	openshiftPollingClient, err := clientimagev1.NewForConfig(&pollingConfig)
	if err != nil {
		return err
	}

	m.AddSampler(
		StartSampling(ctx, m, time.Second, func(previous bool) (condition *Condition, next bool) {
			_, err := pollingClient.Namespaces().Get("kube-system", metav1.GetOptions{})
			switch {
			case err == nil && !previous:
				condition = &Condition{
					Level:   Info,
					Locator: "kube-apiserver",
					Message: "Kube API started responding to GET requests",
				}
			case err != nil && previous:
				condition = &Condition{
					Level:   Error,
					Locator: "kube-apiserver",
					Message: fmt.Sprintf("Kube API started failing: %v", err),
				}
			}
			return condition, err == nil
		}).ConditionWhenFailing(&Condition{
			Level:   Error,
			Locator: "kube-apiserver",
			Message: fmt.Sprintf("Kube API is not responding to GET requests"),
		}),
	)

	m.AddSampler(
		StartSampling(ctx, m, time.Second, func(previous bool) (condition *Condition, next bool) {
			_, err := openshiftPollingClient.ImageStreams("openshift-apiserver").Get("missing", metav1.GetOptions{})
			if !errors.IsUnexpectedServerError(err) && errors.IsNotFound(err) {
				err = nil
			}
			switch {
			case err == nil && !previous:
				condition = &Condition{
					Level:   Info,
					Locator: "openshift-apiserver",
					Message: "OpenShift API started responding to GET requests",
				}
			case err != nil && previous:
				condition = &Condition{
					Level:   Info,
					Locator: "openshift-apiserver",
					Message: fmt.Sprintf("OpenShift API started failing: %v", err),
				}
			}
			return condition, err == nil
		}).ConditionWhenFailing(&Condition{
			Level:   Error,
			Locator: "openshift-apiserver",
			Message: fmt.Sprintf("OpenShift API is not responding to GET requests"),
		}),
	)
	return nil
}

func findContainerStatus(status []corev1.ContainerStatus, name string, position int) *corev1.ContainerStatus {
	if position < len(status) {
		if status[position].Name == name {
			return &status[position]
		}
	}
	for i := range status {
		if status[i].Name == name {
			return &status[i]
		}
	}
	return nil
}

func findNodeCondition(status []corev1.NodeCondition, name corev1.NodeConditionType, position int) *corev1.NodeCondition {
	if position < len(status) {
		if status[position].Type == name {
			return &status[position]
		}
	}
	for i := range status {
		if status[i].Type == name {
			return &status[i]
		}
	}
	return nil
}

func locateEvent(event *corev1.Event) string {
	if len(event.InvolvedObject.Namespace) > 0 {
		return fmt.Sprintf("ns/%s %s/%s", event.InvolvedObject.Namespace, strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name)
	}
	return fmt.Sprintf("%s/%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name)
}

func locatePod(pod *corev1.Pod) string {
	return fmt.Sprintf("ns/%s pod/%s node/%s", pod.Namespace, pod.Name, pod.Spec.NodeName)
}

func locateNode(node *corev1.Node) string {
	return fmt.Sprintf("node/%s", node.Name)
}

func locatePodContainer(pod *corev1.Pod, containerName string) string {
	return fmt.Sprintf("ns/%s pod/%s node/%s container=%s", pod.Namespace, pod.Name, pod.Spec.NodeName, containerName)
}

func filterToSystemNamespaces(obj runtime.Object) bool {
	m, ok := obj.(metav1.Object)
	if !ok {
		return true
	}
	ns := m.GetNamespace()
	if len(ns) == 0 {
		return true
	}
	return strings.HasPrefix(ns, "kube-") || strings.HasPrefix(ns, "openshift-") || ns == "default"
}

type errorRecordingListWatcher struct {
	lw cache.ListerWatcher

	recorder Recorder

	lock          sync.Mutex
	receivedError bool
}

func NewErrorRecordingListWatcher(recorder Recorder, lw cache.ListerWatcher) cache.ListerWatcher {
	return &errorRecordingListWatcher{
		lw:       lw,
		recorder: recorder,
	}
}

func (w *errorRecordingListWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	obj, err := w.lw.List(options)
	w.handle(err)
	return obj, err
}

func (w *errorRecordingListWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	obj, err := w.lw.Watch(options)
	w.handle(err)
	return obj, err
}

func (w *errorRecordingListWatcher) handle(err error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if err != nil {
		if !w.receivedError {
			w.recorder.Record(Condition{
				Level:   Error,
				Locator: "kube-apiserver",
				Message: fmt.Sprintf("failed contacting the API: %v", err),
			})
		}
		w.receivedError = true
	} else {
		w.receivedError = false
	}
}
