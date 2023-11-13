package podaccess

import (
	"bufio"
	"context"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	kapiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type PodStreamer struct {
	kubeClient    kubernetes.Interface
	namespace     string
	podName       string
	containerName string

	localStop          context.CancelFunc
	doneReading        chan struct{}
	lastUID            types.UID
	lastLine           string
	lastContainerStart time.Time
	nextContent        chan LogLineContent
	errs               chan LogError
}

type LogLineContent struct {
	Instant time.Time
	Pod     *corev1.Pod
	Locator monitorapi.Locator
	Line    string
}

type LogError struct {
	Pod     *corev1.Pod
	Locator monitorapi.Locator
	Error   error
}

func NewPodStreamer(kubeClient kubernetes.Interface, namespace, podName, containerName string) *PodStreamer {
	return &PodStreamer{
		kubeClient:    kubeClient,
		namespace:     namespace,
		podName:       podName,
		containerName: containerName,
		nextContent:   make(chan LogLineContent, 100),
		errs:          make(chan LogError, 100),
		doneReading:   make(chan struct{}),
	}
}

func (s *PodStreamer) Run(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer close(s.doneReading)
	defer close(s.nextContent)
	defer close(s.errs)

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	s.localStop = cancelFn

	return wait.PollUntilContextCancel(ctx, time.Second, true, s.streamLogs)
}

func (s *PodStreamer) Output() (chan LogLineContent, chan LogError) {
	return s.nextContent, s.errs
}

func (s *PodStreamer) Stop(ctx context.Context) {
	s.localStop()

	select {
	case <-s.doneReading:
	case <-ctx.Done():
		klog.Errorf("couldn't finish reading")
	}
}

func (s *PodStreamer) streamLogs(ctx context.Context) (bool, error) {
	currPod, err := s.kubeClient.CoreV1().Pods(s.namespace).Get(ctx, s.podName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		utilruntime.HandleError(err)
		return false, nil
	}

	s.streamLogsReader(ctx, currPod)
	return false, nil
}

// streamLogsReader will run and block until
// 1. server closes the http connection
// 2. context is closed.
func (s *PodStreamer) streamLogsReader(ctx context.Context, currPod *corev1.Pod) {
	caughtUpToLastLine := false
	if currPod.UID != s.lastUID {
		s.lastLine = ""
		s.lastUID = currPod.UID
		caughtUpToLastLine = true
	}
	lastStart := lastContainerStart(currPod, s.containerName)
	if lastStart != s.lastContainerStart {
		s.lastLine = ""
		s.lastContainerStart = lastStart
		caughtUpToLastLine = true
	}

	locator := monitorapi.NewLocator().ContainerFromPod(currPod, s.containerName)
	reader, err := s.kubeClient.CoreV1().Pods(s.namespace).GetLogs(s.podName, &kapiv1.PodLogOptions{
		Container:  s.containerName,
		Follow:     true,
		Timestamps: true,
	}).Stream(ctx)
	if err != nil {
		s.errs <- LogError{
			Pod:     currPod,
			Locator: locator,
			Error:   err,
		}
		return
	}

	scan := bufio.NewScanner(reader)
	for scan.Scan() {
		// exit if we have been stopped.
		if ctx.Err() != nil {
			return
		}

		line := scan.Text()
		// if we're searching for the last line, then check
		if !caughtUpToLastLine && len(s.lastLine) > 0 {
			if line == s.lastLine {
				caughtUpToLastLine = true
			}
			continue
		}

		tokens := strings.SplitN(line, " ", 2)
		timeString := tokens[0]
		lineTime, err := time.Parse(time.RFC3339, timeString)
		if err != nil {
			utilruntime.HandleError(err)
		}

		s.nextContent <- LogLineContent{
			Instant: lineTime,
			Pod:     currPod,
			Locator: locator,
			Line:    tokens[1],
		}
		s.lastLine = line
	}
}

func lastContainerStart(pod *corev1.Pod, containerName string) time.Time {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name != containerName {
			continue
		}
		if containerStatus.State.Running != nil {
			return containerStatus.State.Running.StartedAt.Time
		}
		if containerStatus.State.Terminated != nil {
			return containerStatus.State.Terminated.StartedAt.Time
		}
	}

	return time.Time{}
}
