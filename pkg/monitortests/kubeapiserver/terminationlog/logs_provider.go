package terminationlog

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/nodeaccess"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ClusterTerminationLogs struct {
	errs      []error
	beginning time.Time
	end       time.Time
}

func NewClusterTerminationLogs(beginning, end time.Time) *ClusterTerminationLogs {
	return &ClusterTerminationLogs{
		beginning: beginning,
		end:       end,
	}
}

func (p *ClusterTerminationLogs) Process(ctx context.Context, client kubernetes.Interface, handlers ...func(entry *LogEntry)) error {
	entries := p.LogEntries(ctx, client)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case logEntry, ok := <-entries:
			if !ok {
				return p.Err()
			}
			for _, handle := range handlers {
				handle(logEntry)
			}
		}
	}
}

func (p *ClusterTerminationLogs) LogEntries(ctx context.Context, client kubernetes.Interface) <-chan *LogEntry {
	out := make(chan *LogEntry)
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
	if err != nil {
		p.errs = append(p.errs, err)
		close(out)
		return out
	}

	// for each node
	var wg sync.WaitGroup
	p.errs = make([]error, len(nodes.Items))
	for i, node := range nodes.Items {
		select {
		case <-ctx.Done():
			p.errs[i] = ctx.Err()
			break
		default:
		}
		wg.Add(1)
		go func(node *corev1.Node) {
			defer wg.Done()
			nodeLogs := NewNodeTerminationLogs(client, node.Name, p.beginning, p.end)
			for line := range nodeLogs.LogEntries(ctx) {
				out <- line
			}
			p.errs[i] = nodeLogs.Err()
		}(&node)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (p *ClusterTerminationLogs) Err() error {
	return errors.Join(p.errs...)
}

func NewNodeTerminationLogs(client kubernetes.Interface, node string, beginning, end time.Time) *NodeTerminationLogs {
	return &NodeTerminationLogs{
		client:    client,
		nodeName:  node,
		beginning: beginning,
		end:       end,
	}
}

type NodeTerminationLogs struct {
	client    kubernetes.Interface
	nodeName  string
	errs      []error
	beginning time.Time
	end       time.Time
}

func (p *NodeTerminationLogs) Err() error {
	return errors.Join(p.errs...)
}

func (p *NodeTerminationLogs) LogEntries(ctx context.Context) <-chan *LogEntry {
	out := make(chan *LogEntry)
	dirData, err := nodeaccess.GetNodeLogFile(ctx, p.client, p.nodeName, "kube-apiserver")
	if err != nil {
		p.errs = []error{err}
		close(out)
		return out
	}
	fileNames, err := nodeaccess.GetDirectoryListing(bytes.NewBuffer(dirData))
	if err != nil {
		p.errs = []error{err}
		close(out)
		return out
	}

	var wg sync.WaitGroup
	fileNames = slices.DeleteFunc(fileNames, func(s string) bool {
		return !strings.HasPrefix(s, "termination") || !strings.HasSuffix(s, ".log")
	})
	p.errs = make([]error, len(fileNames))
	for i, fileName := range fileNames {
		select {
		case <-ctx.Done():
			p.errs[i] = ctx.Err()
			break
		default:
		}
		wg.Add(1)
		go func(ctx context.Context, fileName string, i int) {
			defer wg.Done()
			lines, err := nodeaccess.StreamNodeLogFile(ctx, p.client, p.nodeName, path.Join("kube-apiserver", fileName))
			if err != nil {
				p.errs[i] = err
				return
			}
			scanner := bufio.NewScanner(lines)
			line := 0
			for scanner.Scan() {
				line++
				text := scanner.Text()
				entry, _, err := NewLogEntry(text)
				if err != nil {
					fmt.Printf("unable to decode %q line %d: %s to audit event: %v\n", fileName, line, text, err)
					continue
				}
				entry.Node = p.nodeName
				if (!p.beginning.IsZero() && entry.TS.Before(p.beginning)) || (!p.end.IsZero() && entry.TS.After(p.end)) {
					continue
				}
				out <- entry
			}
		}(ctx, fileName, i)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

type LogEntry struct {
	Node   string
	Level  string
	TS     time.Time
	File   string
	LineNo string
	Msg    string
}

// Log lines have this form: Lmmdd hh:mm:ss.uuuuuu threadid file:line] msg...
var linePattern = regexp.MustCompile(`^(?P<level>[WEIF])(?P<ts>\d{4} \d\d:\d\d:\d\d\.\d{6})\s+(?P<tid>\d+)\s+(?P<file>[^:]+):(?P<line>\d+)]\s+(?P<msg>.*)`)

// termination log timestamps do not specify the year. Assume it is the current
// year, this might cause problems if a job spans over the new year.
var year = time.Now().Year()

func NewLogEntry(line string) (*LogEntry, bool, error) {
	match := linePattern.FindStringSubmatch(line)
	if match == nil {
		// return the non-parsable line as is
		return &LogEntry{Msg: line}, false, nil
	}
	ts, err := time.Parse(`0102 15:04:05.000000`, match[2])
	if err != nil {
		return nil, false, err
	}
	ts = time.Date(year, ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), ts.Nanosecond(), ts.Location())
	return &LogEntry{
		Level:  match[1],
		TS:     ts,
		File:   match[4],
		LineNo: match[5],
		Msg:    match[6],
	}, true, nil
}
