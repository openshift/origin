package nodedetails

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func GetKubeAuditLogSummary(ctx context.Context, kubeClient kubernetes.Interface) (*AuditLogSummary, error) {
	masterOnly, err := labels.NewRequirement("node-role.kubernetes.io/master", selection.Exists, nil)
	if err != nil {
		panic(err)
	}
	allNodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*masterOnly).String(),
	})

	if err != nil {
		return nil, err
	}

	ret := NewAuditLogSummary()
	lock := sync.Mutex{}
	errCh := make(chan error, len(allNodes.Items))
	wg := sync.WaitGroup{}
	for _, node := range allNodes.Items {
		wg.Add(1)
		go func(ctx context.Context, nodeName string) {
			defer wg.Done()

			auditLogSummary, err := getNodeKubeAuditLogSummary(ctx, kubeClient, nodeName)
			if err != nil {
				errCh <- err
				return
			}

			lock.Lock()
			defer lock.Unlock()
			ret.AddSummary(auditLogSummary)
		}(ctx, node.Name)
	}
	wg.Wait()

	errs := []error{}
	for len(errCh) > 0 {
		err := <-errCh
		errs = append(errs, err)
	}

	return ret, utilerrors.NewAggregate(errs)
}

func getNodeKubeAuditLogSummary(ctx context.Context, client kubernetes.Interface, nodeName string) (*AuditLogSummary, error) {
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
		if !strings.HasPrefix(auditLogFilename, "audit") {
			continue
		}
		if !strings.HasSuffix(auditLogFilename, ".log") {
			continue
		}

		wg.Add(1)
		go func(ctx context.Context, auditLogFilename string) {
			defer wg.Done()

			auditStream, err := streamNodeLogFile(ctx, client, nodeName, filepath.Join(apiserver, auditLogFilename))
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
			auditLogSummaries <- auditLogSummary

		}(ctx, auditLogFilename)
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

	return fullSummary, utilerrors.NewAggregate(errs)
}

// this is copy/pasted from the oc node logs impl
func GetDirectoryListing(in io.Reader) ([]string, error) {
	filenames := []string{}
	bufferSize := 4096
	buf := bufio.NewReaderSize(in, bufferSize)

	// turn href links into lines of output
	content, _ := buf.Peek(bufferSize)
	if bytes.HasPrefix(content, []byte("<pre>")) {
		reLink := regexp.MustCompile(`href="([^"]+)"`)
		s := bufio.NewScanner(buf)
		s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			matches := reLink.FindSubmatchIndex(data)
			if matches == nil {
				advance = bytes.LastIndex(data, []byte("\n"))
				if advance == -1 {
					advance = 0
				}
				return advance, nil, nil
			}
			advance = matches[1]
			token = data[matches[2]:matches[3]]
			return advance, token, nil
		})
		for s.Scan() {
			filename := s.Text()
			filenames = append(filenames, filename)
		}
		return filenames, s.Err()
	}

	return nil, fmt.Errorf("not a directory listing")
}

func streamNodeLogFile(ctx context.Context, client kubernetes.Interface, nodeName, filename string) (io.ReadCloser, error) {
	path := client.CoreV1().RESTClient().Get().
		Namespace("").Name(nodeName).
		Resource("nodes").SubResource("proxy", "logs").Suffix(filename).URL().Path

	req := client.CoreV1().RESTClient().Get().RequestURI(path).
		SetHeader("Accept", "text/plain, */*")

	return req.Stream(ctx)
}

func GetNodeLogFile(ctx context.Context, client kubernetes.Interface, nodeName, filename string) ([]byte, error) {
	in, err := streamNodeLogFile(ctx, client, nodeName, filename)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	return ioutil.ReadAll(in)
}

func getAuditLogFilenames(ctx context.Context, client kubernetes.Interface, nodeName, apiserverName string) ([]string, error) {
	allBytes, err := GetNodeLogFile(ctx, client, nodeName, apiserverName)
	if err != nil {
		return nil, err
	}

	filenames, err := GetDirectoryListing(bytes.NewBuffer(allBytes))
	if err != nil {
		return nil, err
	}

	return filenames, nil
}
