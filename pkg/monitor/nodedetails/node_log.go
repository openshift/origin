package nodedetails

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"

	"k8s.io/client-go/kubernetes"
)

// GetNodeLog returns logs for a particular systemd service on a given node.
// We're count on these logs to fit into some reasonable memory size.
func GetNodeLog(ctx context.Context, client kubernetes.Interface, nodeName, systemdServiceName string) ([]byte, error) {
	path := client.CoreV1().RESTClient().Get().
		Namespace("").Name(nodeName).
		Resource("nodes").SubResource("proxy", "logs").Suffix("journal").URL().Path

	req := client.CoreV1().RESTClient().Get().RequestURI(path).
		SetHeader("Accept", "text/plain, */*")
	req.Param("since", "-1d")
	req.Param("unit", systemdServiceName)

	in, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	return ioutil.ReadAll(in)
}

// GetAuditLogBytes returns logs for a particular systemd service on a given node.
// We're count on these logs to fit into some reasonable memory size.
func GetKubeAuditLogSummary(ctx context.Context, client kubernetes.Interface, nodeName string) (*AuditLogSummary, error) {
	return getAuditLogSummary(ctx, client, nodeName, "kube-apiserver")
}
func getAuditLogSummary(ctx context.Context, client kubernetes.Interface, nodeName, apiserver string) (*AuditLogSummary, error) {
	auditLogFilenames, err := getAuditLogFilenames(ctx, client, nodeName, apiserver)
	if err != nil {
		return nil, err
	}

	// we do not have enough memory to read all the content and then navigate it all in memory.
	// I mean, maybe we do, but we are looking at a number of bytes read measured in gigs, then the deserialization buffers,
	// and then the maps to actually store the content.
	// Instead, we will stream, deserialize as we consume, and run a summarize function to get aggregate values.
	wg := sync.WaitGroup{}

	errCh := make(chan error, len(auditLogFilenames))
	auditLogSummaries := make(chan *AuditLogSummary, len(auditLogFilenames))
	for _, auditLogFilename := range auditLogFilenames {
		wg.Add(1)
		go func(ctx context.Context) {
			defer wg.Done()

			auditStream, err := streamNodeLogFile(ctx, client, nodeName, auditLogFilename)
			if err != nil {
				errCh <- err
				return
			}

			auditLogSummary := NewAuditLogSummary()
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
					auditLogSummary.lineReadFailureCount++
					fmt.Printf("unable to decode %q line %d: %s to audit event: %v\n", auditLogFilename, line, string(auditLine), err)
					continue
				}

				auditLogSummary.Add(auditEvent, auditEventInfo{})
			}

		}(ctx)
	}
	wg.Wait()
	close(errCh)
	close(auditLogSummaries)

	errs := []error{}
	for err := range errCh {
		errs = append(errs, err)
	}

	fullSummary := NewAuditLogSummary()
	for auditLogSummary := range auditLogSummaries {
		fullSummary.AddSummary(auditLogSummary)
	}

	return fullSummary, nil
}

func streamNodeLogFile(ctx context.Context, client kubernetes.Interface, nodeName, filename string) (io.ReadCloser, error) {
	path := client.CoreV1().RESTClient().Get().
		Namespace("").Name(nodeName).
		Resource("nodes").SubResource("proxy", "logs").Suffix(filename).URL().Path

	req := client.CoreV1().RESTClient().Get().RequestURI(path).
		SetHeader("Accept", "text/plain, */*")

	return req.Stream(ctx)
}

func getNodeLogFile(ctx context.Context, client kubernetes.Interface, nodeName, filename string) ([]byte, error) {
	in, err := streamNodeLogFile(ctx, client, nodeName, filename)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	return ioutil.ReadAll(in)
}

func getAuditLogFilenames(ctx context.Context, client kubernetes.Interface, nodeName, apiserverName string) ([]string, error) {
	allBytes, err := getNodeLogFile(ctx, client, nodeName, apiserverName)
	if err != nil {
		return nil, err
	}

	filenames := []string{}
	scanner := bufio.NewScanner(bytes.NewBuffer(allBytes))
	for scanner.Scan() {
		filename := string(scanner.Bytes())
		switch {
		default:
			filenames = append(filenames, filename)
		}
	}

	return filenames, nil
}
