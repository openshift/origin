package podaccess

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
)

type OneTimePodStreamer struct {
	kubeClient    kubernetes.Interface
	namespace     string
	podName       string
	containerName string

	logHandlers []LogHandler
}

func NewOneTimePodStreamer(kubeClient kubernetes.Interface, namespace, podName, containerName string, logHandlers ...LogHandler) *OneTimePodStreamer {
	return &OneTimePodStreamer{
		kubeClient:    kubeClient,
		namespace:     namespace,
		podName:       podName,
		containerName: containerName,
		logHandlers:   logHandlers,
	}
}

func (s *OneTimePodStreamer) ReadLog(ctx context.Context) error {
	currPod, err := s.kubeClient.CoreV1().Pods(s.namespace).Get(ctx, s.podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting pod to read logs: %w", err)
	}

	return s.streamLogsReader(ctx, currPod)
}

// streamLogsReader will run and block until
// 1. server closes the http connection
// 2. context is closed.
func (s *OneTimePodStreamer) streamLogsReader(ctx context.Context, currPod *corev1.Pod) error {
	locator := monitorapi.NewLocator().ContainerFromPod(currPod, s.containerName)
	reader, err := s.kubeClient.CoreV1().Pods(s.namespace).GetLogs(s.podName, &kapiv1.PodLogOptions{
		Container:  s.containerName,
		Follow:     false,
		Timestamps: true,
	}).Stream(ctx)
	if err != nil {
		return err
	}

	scan := bufio.NewScanner(reader)
	for scan.Scan() {
		// exit if we have been stopped.
		if ctx.Err() != nil {
			return nil
		}

		line := scan.Text()

		tokens := strings.SplitN(line, " ", 2)
		timeString := tokens[0]
		lineTime, err := time.Parse(time.RFC3339, timeString)
		if err != nil {
			utilruntime.HandleError(err)
		}

		currLine := LogLineContent{
			Instant: lineTime,
			Pod:     currPod,
			Locator: locator,
			Line:    tokens[1],
		}

		for _, logHandler := range s.logHandlers {
			logHandler.HandleLogLine(currLine)
		}

	}

	return nil
}
