package auditloganalyzer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/nodeaccess"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/kubernetes"
)

func GetKubeAuditLogSummary(ctx context.Context, kubeClient kubernetes.Interface, beginning, end *time.Time, auditLogHandlers []AuditEventHandler) error {
	masterOnly, err := labels.NewRequirement("node-role.kubernetes.io/master", selection.Exists, nil)
	if err != nil {
		panic(err)
	}
	allNodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*masterOnly).String(),
	})

	if err != nil {
		return err
	}

	lock := sync.Mutex{}
	errCh := make(chan error, len(allNodes.Items))
	wg := sync.WaitGroup{}
	for _, node := range allNodes.Items {
		wg.Add(1)
		go func(ctx context.Context, nodeName string) {
			defer wg.Done()
			var microBeginning, microEnd *metav1.MicroTime
			if nil != beginning {
				micro := metav1.NewMicroTime(*beginning)
				microBeginning = &micro
			}
			if nil != end {
				micro := metav1.NewMicroTime(*end)
				microEnd = &micro
			}
			err := getNodeKubeAuditLogSummary(ctx, kubeClient, nodeName, microBeginning, microEnd, auditLogHandlers)
			if err != nil {
				errCh <- err
				return
			}

			lock.Lock()
			defer lock.Unlock()
		}(ctx, node.Name)
	}
	wg.Wait()

	errs := []error{}
	for len(errCh) > 0 {
		err := <-errCh
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

type AuditEventHandler interface {
	HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string)
}

func getNodeKubeAuditLogSummary(ctx context.Context, client kubernetes.Interface, nodeName string, beginning, end *metav1.MicroTime, auditLogHandlers []AuditEventHandler) error {
	return getAuditLogSummary(ctx, client, nodeName, "kube-apiserver", beginning, end, auditLogHandlers)
}
func getAuditLogSummary(ctx context.Context, client kubernetes.Interface, nodeName, apiserver string, beginning, end *metav1.MicroTime, auditLogHandlers []AuditEventHandler) error {
	auditLogFilenames, err := getAuditLogFilenames(ctx, client, nodeName, apiserver)
	if err != nil {
		return err
	}

	// we do not have enough memory to read all the content and then navigate it all in memory.
	// I mean, maybe we do, but we are looking at a number of bytes read measured in gigs, then the deserialization buffers,
	// and then the maps to actually store the content.
	// Instead, we will stream, deserialize as we consume, and run a summarize function to get aggregate values.
	wg := sync.WaitGroup{}

	errCh := make(chan error, len(auditLogFilenames))
	for _, auditLogFilename := range auditLogFilenames {
		if !strings.HasPrefix(auditLogFilename, "audit") {
			continue
		}
		if !strings.HasSuffix(auditLogFilename, ".log") {
			continue
		}

		wg.Add(1)
		go func(ctx context.Context, auditLogFilename, nodeName string) {
			defer wg.Done()

			auditStream, err := nodeaccess.StreamNodeLogFile(ctx, client, nodeName, filepath.Join(apiserver, auditLogFilename))
			if err != nil {
				errCh <- err
				return
			}

			scanner := bufio.NewScanner(auditStream)
			line := 0
			for scanner.Scan() {
				line++
				auditLine := scanner.Bytes()

				if len(auditLine) == 0 {
					continue
				}

				auditEvent := &auditv1.Event{}
				if err := json.Unmarshal(auditLine, auditEvent); err != nil {
					fmt.Printf("unable to decode %q line %d: %s to audit event: %v\n", auditLogFilename, line, string(auditLine), err)
					continue
				}

				for _, auditLogHandler := range auditLogHandlers {
					auditLogHandler.HandleAuditLogEvent(auditEvent, beginning, end, nodeName)
				}
			}

		}(ctx, auditLogFilename, nodeName)
	}
	wg.Wait()
	close(errCh)

	errs := []error{}
	for err := range errCh {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func getAuditLogFilenames(ctx context.Context, client kubernetes.Interface, nodeName, apiserverName string) ([]string, error) {
	allBytes, err := nodeaccess.GetNodeLogFile(ctx, client, nodeName, apiserverName)
	if err != nil {
		return nil, err
	}

	filenames, err := nodeaccess.GetDirectoryListing(bytes.NewBuffer(allBytes))
	if err != nil {
		return nil, err
	}

	return filenames, nil
}
