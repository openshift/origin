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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/kubernetes"
)

func GetKubeAuditLogSummary(ctx context.Context, kubeClient kubernetes.Interface, beginning, end *time.Time, auditLogHandlers []AuditEventHandler) error {
	events, err := GetEvents(`/home/deads/Downloads/audit-logs(13)/registry-build04-ci-openshift-org-ci-op-ywclgz0t-stable-sha256-1bcf20915cd7b01643a5555630e06be72e4fef84cd34d1bdd10242c9fcf6cfdf/audit_logs/kube-apiserver`)
	if err != nil {
		return err
	}

	for _, auditEvent := range events {
		for _, auditLogHandler := range auditLogHandlers {
			auditLogHandler.HandleAuditLogEvent(auditEvent, nil, nil)
		}
	}
	return nil
}

type AuditEventHandler interface {
	HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime)
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
		go func(ctx context.Context, auditLogFilename string) {
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
					auditLogHandler.HandleAuditLogEvent(auditEvent, beginning, end)
				}
			}

		}(ctx, auditLogFilename)
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
