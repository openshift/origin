package nodeprovisioninglatency

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	"github.com/openshift/origin/pkg/dataloader"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// internalAPIHost stands in for the address that must never reach a publicly archived CI log.
const internalAPIHost = "api-int.mycluster.example.com"

// unreachableAPIConfig points real clients at the internal host but fails every dial, so requests
// return the same URL-bearing errors a cluster outage would, without touching the network.
func unreachableAPIConfig() *rest.Config {
	return &rest.Config{
		Host:  "https://" + net.JoinHostPort(internalAPIHost, "6443"),
		Proxy: func(*http.Request) (*url.URL, error) { return nil, nil },
		Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("dialing is disabled in unit tests")
		},
	}
}

// logBuffer is safe for the informer goroutines that keep logging while a test reads the output.
type logBuffer struct {
	lock   sync.Mutex
	buffer bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.buffer.Write(p)
}

func (b *logBuffer) String() string {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.buffer.String()
}

func newTestMonitor() (*nodeProvisioningLatency, *logBuffer) {
	output := &logBuffer{}
	logger := logrus.New()
	logger.SetOutput(output)

	m := NewNodeProvisioningLatency().(*nodeProvisioningLatency)
	m.logger = logrus.NewEntry(logger)
	return m, output
}

func unreachableClients(t *testing.T, config *rest.Config) (machineclient.Interface, kubernetes.Interface) {
	t.Helper()
	machineClient, err := machineclient.NewForConfig(config)
	if err != nil {
		t.Fatalf("unable to build Machine client: %v", err)
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("unable to build Kubernetes client: %v", err)
	}
	return machineClient, kubeClient
}

func TestAPIErrorsDoNotLogTheAPIServerAddress(t *testing.T) {
	config := unreachableAPIConfig()
	machineClient, kubeClient := unreachableClients(t, config)

	// Without this precondition the assertions below would pass vacuously.
	_, rawErr := machineClient.MachineV1beta1().Machines(machineAPINamespace).List(context.Background(), metav1.ListOptions{})
	if rawErr == nil || !strings.Contains(rawErr.Error(), internalAPIHost) {
		t.Fatalf("expected the raw client error to contain %q, got %v", internalAPIHost, rawErr)
	}

	tests := []struct {
		name    string
		run     func(t *testing.T, m *nodeProvisioningLatency, output *logBuffer)
		wantLog []string
	}{
		{
			name: "Machine API probe in PrepareCollection",
			run: func(t *testing.T, m *nodeProvisioningLatency, output *logBuffer) {
				if err := m.PrepareCollection(context.Background(), config, nil); err != nil {
					t.Errorf("PrepareCollection returned %v, want nil", err)
				}
				if !m.unavailable {
					t.Errorf("expected the monitor to be marked unavailable")
				}
			},
			wantLog: []string{"unable to list Machines in " + machineAPINamespace},
		},
		{
			name: "final Machine and Node lists",
			run: func(t *testing.T, m *nodeProvisioningLatency, output *logBuffer) {
				m.machineClient = machineClient
				m.kubeClient = kubeClient
				m.listFinalState(context.Background())
			},
			wantLog: []string{"final Machine list failed", "final Node list failed"},
		},
		{
			name: "informer list failures",
			run: func(t *testing.T, m *nodeProvisioningLatency, output *logBuffer) {
				m.machineClient = machineClient
				m.kubeClient = kubeClient

				// The caches can never sync, so stop the informers as soon as both have reported a
				// failure instead of waiting for collectionSyncTimeout.
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				go func() {
					for ctx.Err() == nil {
						logged := output.String()
						if strings.Contains(logged, "Machine watch failed") && strings.Contains(logged, "Node watch failed") {
							cancel()
							return
						}
						time.Sleep(10 * time.Millisecond)
					}
				}()
				m.startInformers(ctx)
			},
			wantLog: []string{"Machine watch failed", "Node watch failed"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m, output := newTestMonitor()
			test.run(t, m, output)

			logged := output.String()
			for _, want := range test.wantLog {
				if !strings.Contains(logged, want) {
					t.Errorf("expected log to contain %q, got:\n%s", want, logged)
				}
			}
			if strings.Contains(logged, internalAPIHost) {
				t.Errorf("log leaked the internal apiserver address:\n%s", logged)
			}
		})
	}
}

func TestWatchErrorHandler(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantLog string
	}{
		{name: "watch closed", err: io.EOF},
		{name: "watch closed unexpectedly", err: io.ErrUnexpectedEOF},
		{name: "resource version expired", err: apierrors.NewResourceExpired("too old resource version")},
		{name: "resource version gone", err: apierrors.NewGone("gone")},
		{
			name:    "apiserver rejection",
			err:     apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "", errors.New("denied")),
			wantLog: "Node watch failed: apiserver status 403 Forbidden",
		},
		{
			name: "list request that failed with EOF",
			err: fmt.Errorf("failed to list *v1.Node: %w", &url.Error{
				Op:  "Get",
				URL: "https://" + net.JoinHostPort(internalAPIHost, "6443") + "/api/v1/nodes",
				Err: io.EOF,
			}),
			wantLog: "Node watch failed: Get request failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m, output := newTestMonitor()
			m.watchErrorHandler("Node")(nil, test.err)

			logged := output.String()
			if test.wantLog == "" {
				if logged != "" {
					t.Errorf("expected no log for a normal watch closure, got:\n%s", logged)
				}
				return
			}
			if !strings.Contains(logged, test.wantLog) {
				t.Errorf("expected log to contain %q, got:\n%s", test.wantLog, logged)
			}
			if strings.Contains(logged, internalAPIHost) {
				t.Errorf("log leaked the internal apiserver address:\n%s", logged)
			}
		})
	}
}

func TestEvaluateTestsFromConstructedIntervalsProducesNoTestCases(t *testing.T) {
	m, _ := newTestMonitor()
	slow := durationPtr(24 * time.Hour)
	m.rows = []provisioningRow{{
		MachineName:       "worker-1",
		MachineRole:       roleWorker,
		CreatedDuringRun:  true,
		MachineToInstance: slow,
		InstanceToNode:    slow,
		NodeToReady:       slow,
		MachineToNode:     slow,
		MachineToReady:    slow,
	}}

	junits, err := m.EvaluateTestsFromConstructedIntervals(context.Background(), nil)
	if err != nil {
		t.Errorf("EvaluateTestsFromConstructedIntervals returned %v, want nil", err)
	}
	if len(junits) != 0 {
		t.Errorf("expected no test cases however slow provisioning was, got %d", len(junits))
	}
}

func TestWriteContentToStorageWritesAutodlFile(t *testing.T) {
	beginning := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	m, _ := newTestMonitor()
	m.rows = []provisioningRow{
		{MachineName: "master-0", MachineRole: roleMaster, MachineCreationTimestamp: beginning.Add(-time.Hour)},
		{MachineName: "worker-1", MachineRole: roleWorker, CreatedDuringRun: true, MachineToReady: durationPtr(5 * time.Minute)},
	}

	storageDir := t.TempDir()
	if err := m.WriteContentToStorage(context.Background(), storageDir, "_20260101-100000", nil, nil); err != nil {
		t.Fatalf("WriteContentToStorage returned %v", err)
	}

	content, err := os.ReadFile(filepath.Join(storageDir, "node-provisioning-latency_20260101-100000-"+dataloader.AutoDataLoaderSuffix))
	if err != nil {
		t.Fatalf("expected the autodl file to be written: %v", err)
	}
	dataFile := dataloader.DataFile{}
	if err := json.Unmarshal(content, &dataFile); err != nil {
		t.Fatalf("autodl file is not valid JSON: %v", err)
	}
	if dataFile.TableName != autodlTableName {
		t.Errorf("TableName = %q, want %q", dataFile.TableName, autodlTableName)
	}
	if len(dataFile.Rows) != len(m.rows) {
		t.Fatalf("expected %d rows, got %d", len(m.rows), len(dataFile.Rows))
	}
	if got := dataFile.Rows[0]["MachineRole"]; got != roleMaster {
		t.Errorf("first row MachineRole = %q, want %q", got, roleMaster)
	}
	if got := dataFile.Rows[0]["CreatedDuringRun"]; got != "false" {
		t.Errorf("first row CreatedDuringRun = %q, want %q", got, "false")
	}
	if got := dataFile.Rows[1]["MachineToReadySeconds"]; got != "300.000" {
		t.Errorf("second row MachineToReadySeconds = %q, want %q", got, "300.000")
	}
}
