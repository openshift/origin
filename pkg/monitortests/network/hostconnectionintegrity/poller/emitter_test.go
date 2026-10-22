package poller

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

func TestJSONEmitterSeparatesDiagnostics(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	serializationError := errors.New("test serialization failure")
	emit := newJSONEmitter(&stdout, &stderr, func(monitorapi.Interval) ([]byte, error) {
		return nil, serializationError
	})
	emit(monitorapi.Interval{})
	if stdout.Len() != 0 {
		t.Fatalf("serialization diagnostic on stdout: %q", stdout.String())
	}
	if stderr.String() != "failed to serialize interval: "+serializationError.Error()+"\n" {
		t.Fatalf("missing serialization error on stderr: %q", stderr.String())
	}
	diagnostic := stderr.String()
	emit = JSONEmitter(&stdout, &stderr)
	now := time.Date(2026, time.October, 21, 0, 0, 0, 0, time.UTC)
	interval := lifecycleInterval("node-a", Target{Backend: BackendAPIIntSelf, Name: "api-int", URL: "https://api.example:6443/healthz"}, ReasonWatchStarted, "started watching", now)
	emit(interval)
	emit(interval)
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two JSON lines, got %q", stdout.String())
	}
	for _, line := range lines {
		decoded, err := monitorserialization.IntervalFromJSON([]byte(line))
		if err != nil {
			t.Fatalf("invalid interval JSON %q: %v", line, err)
		}
		if decoded.Source != interval.Source || decoded.Message.Reason != interval.Message.Reason || !decoded.From.Equal(now) {
			t.Errorf("interval changed during serialization: %+v", decoded)
		}
	}
	if stderr.String() != diagnostic {
		t.Errorf("valid intervals added diagnostics: %q", stderr.String())
	}
}
