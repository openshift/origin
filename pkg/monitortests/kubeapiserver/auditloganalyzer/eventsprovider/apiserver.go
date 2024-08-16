package eventsprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/openshift/origin/pkg/monitortestlibrary/nodeaccess"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	v2 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type APIServerEvents struct {
	apiserver string
	nodeName  string
	beginning *v1.MicroTime
	end       *v1.MicroTime

	done chan struct{}
	errs []error
}

func NewAPIServerEvents(nodeName, apiserver string, beginning, end *v1.MicroTime) *APIServerEvents {
	return &APIServerEvents{
		nodeName:  nodeName,
		apiserver: apiserver,
		beginning: beginning,
		end:       end,
		done:      make(chan struct{}),
	}
}

func (p *APIServerEvents) Done() <-chan struct{} {
	return p.done
}

// Err returns nil if Done is not yet closed.
func (p *APIServerEvents) Err() error {
	select {
	case <-p.done:
		return errors.Join(p.errs...)
	default:
		return nil
	}
}

func (p *APIServerEvents) Events(ctx context.Context, client kubernetes.Interface) <-chan *v2.Event {
	out := make(chan *v2.Event)
	auditLogFilenames, err := getAuditLogFilenames(ctx, client, p.nodeName, p.apiserver)
	if err != nil {
		p.errs = []error{err}
		close(out)
		close(p.done)
		return out
	}

	var wg sync.WaitGroup
	p.errs = make([]error, len(auditLogFilenames))
	for i, filename := range auditLogFilenames {
		if !strings.HasPrefix(filename, "audit") {
			continue
		}
		if !strings.HasSuffix(filename, ".log") {
			continue
		}
		wg.Add(1)
		go func(ctx context.Context, filename string, i int) {
			defer wg.Done()
			klog.InfoS("Started parsing audit log", "node", p.nodeName, "apiserver", p.apiserver, "filename", filename)
			defer klog.InfoS("Finished parsing audit log", "node", p.nodeName, "apiserver", p.apiserver, "filename", filename)
			auditStream, err := nodeaccess.StreamNodeLogFile(ctx, client, p.nodeName, filepath.Join(p.apiserver, filename))
			if err != nil {
				p.errs[i] = err
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

				auditEvent := &v2.Event{}
				if err := json.Unmarshal(auditLine, auditEvent); err != nil {
					// TODO auditLogSummary.lineReadFailureCount++
					fmt.Printf("unable to decode %q line %d: %s to audit event: %v\n", filename, line, string(auditLine), err)
					continue
				}

				if p.beginning != nil && auditEvent.RequestReceivedTimestamp.Before(p.beginning) || p.end != nil && p.end.Before(&auditEvent.RequestReceivedTimestamp) {
					continue
				}
				out <- auditEvent
			}
		}(ctx, filename, i)
	}

	go func() {
		wg.Wait()
		close(out)
		close(p.done)
	}()

	return out
}
