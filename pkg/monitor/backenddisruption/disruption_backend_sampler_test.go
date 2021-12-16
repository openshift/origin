package backenddisruption

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/client-go/tools/events"
)

func TestBackendSampler_checkConnection(t *testing.T) {
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/200":
			w.WriteHeader(200)
			w.Write([]byte("200"))
		case req.URL.Path == "/302-bad-response":
			w.WriteHeader(302)
			w.Write([]byte("302-bad-response"))
		case req.URL.Path == "/302":
			w.Header().Set("Location", "http://google.com")
			w.WriteHeader(302)
			w.Write([]byte("302"))
		case req.URL.Path == "/503":
			w.WriteHeader(503)
			w.Write([]byte("503"))
		case req.URL.Path == "/timeout":
			time.Sleep(2 * time.Second)
			w.WriteHeader(200)
			w.Write([]byte("200"))
		default:
			w.WriteHeader(404)
		}
	}))
	testHost := testServer.URL
	defer testServer.Close()

	type fields struct {
		disruptionBackendName string
		connectionType        BackendConnectionType
		path                  string
		expect                string
		expectRegexp          string

		cancelImmediately bool
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "simple-200",
			fields: fields{
				disruptionBackendName: "simple-200",
				connectionType:        NewConnectionType,
				path:                  "/200",
				expect:                "200",
			},
			wantErr: false,
		},
		{
			name: "200-bad-exect",
			fields: fields{
				disruptionBackendName: "simple-200",
				connectionType:        NewConnectionType,
				path:                  "/200",
				expect:                "other",
			},
			wantErr: true,
		},
		{
			// 302 response missing Location header
			name: "302-no-expect-bad-response",
			fields: fields{
				disruptionBackendName: "302-bad-response",
				connectionType:        NewConnectionType,
				path:                  "/302-bad-response",
			},
			wantErr: true,
		},
		{
			name: "302-no-expect",
			fields: fields{
				disruptionBackendName: "302",
				connectionType:        NewConnectionType,
				path:                  "/302",
			},
			wantErr: false,
		},
		{
			name: "503",
			fields: fields{
				disruptionBackendName: "503",
				connectionType:        NewConnectionType,
				path:                  "/503",
			},
			wantErr: true,
		},
		{
			name: "timeout",
			fields: fields{
				disruptionBackendName: "timeout",
				connectionType:        NewConnectionType,
				path:                  "/timeout",
			},
			wantErr: true,
		},
		{
			name: "cancel-immediately",
			fields: fields{
				disruptionBackendName: "timeout",
				connectionType:        NewConnectionType,
				path:                  "/timeout",
				cancelImmediately:     true,
			},
			wantErr: false, // cancelling doesn't produce errors, it just means we were shutdown
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := NewSimpleBackend(testHost, tt.fields.disruptionBackendName, tt.fields.path, tt.fields.connectionType)
			timeout := 1 * time.Second
			backend.timeout = &timeout
			if len(tt.fields.expect) > 0 {
				backend = backend.WithExpectedBody(tt.fields.expect)
			}
			if len(tt.fields.expectRegexp) > 0 {
				backend = backend.WithExpectedBodyRegex(tt.fields.expectRegexp)
			}

			ctx, cancel := context.WithCancel(context.Background())
			if tt.fields.cancelImmediately {
				cancel()
			}

			if err := backend.checkConnection(ctx); (err != nil) != tt.wantErr {
				t.Errorf("checkConnection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_disruptionSampler_consumeSamples(t *testing.T) {
	tests := []struct {
		name            string
		estimatedTime   time.Duration
		produceSamples  func(ctx context.Context, backendSampler *disruptionSampler)
		validateSamples func(t *testing.T, eventIntervals []monitorapi.EventInterval) error
	}{
		{
			name:          "in-order",
			estimatedTime: 1 * time.Second,
			produceSamples: func(ctx context.Context, backendSampler *disruptionSampler) {
				now := time.Now()
				firstSample := backendSampler.newSample(ctx)
				firstSample.startTime = now
				firstSample.setSampleError(nil)
				close(firstSample.finished)

				secondSample := backendSampler.newSample(ctx)
				secondSample.startTime = firstSample.startTime.Add(1 * time.Second)
				secondSample.setSampleError(nil)
				close(secondSample.finished)

				thirdSample := backendSampler.newSample(ctx)
				thirdSample.startTime = secondSample.startTime.Add(1 * time.Second)
				thirdSample.setSampleError(fmt.Errorf("now fail"))
				close(thirdSample.finished)

				fourthSample := backendSampler.newSample(ctx)
				fourthSample.startTime = thirdSample.startTime.Add(1 * time.Second)
				fourthSample.setSampleError(fmt.Errorf("now fail"))
				close(fourthSample.finished)
			},
			validateSamples: func(t *testing.T, eventIntervals []monitorapi.EventInterval) error {
				if len(eventIntervals) != 2 {
					t.Fatal(eventIntervals)
				}
				if duration := eventIntervals[0].To.Sub(eventIntervals[0].From); duration != 2*time.Second {
					t.Error(eventIntervals[0])
				}
				if !strings.Contains(eventIntervals[0].Message, "started responding") {
					t.Error(eventIntervals[0])
				}
				if duration := eventIntervals[1].To.Sub(eventIntervals[1].From); duration != 2*time.Second {
					t.Error(eventIntervals[0])
				}
				if !strings.Contains(eventIntervals[1].Message, "now fail") {
					t.Error(eventIntervals[1])
				}
				return nil
			},
		},
		{
			name:          "out-of-order-finish",
			estimatedTime: 1 * time.Second,
			produceSamples: func(ctx context.Context, backendSampler *disruptionSampler) {
				now := time.Now()
				firstSample := backendSampler.newSample(ctx)
				firstSample.startTime = now
				firstSample.setSampleError(nil)

				secondSample := backendSampler.newSample(ctx)
				secondSample.startTime = firstSample.startTime.Add(1 * time.Second)
				secondSample.setSampleError(nil)

				thirdSample := backendSampler.newSample(ctx)
				thirdSample.startTime = secondSample.startTime.Add(1 * time.Second)
				thirdSample.setSampleError(fmt.Errorf("now fail"))

				fourthSample := backendSampler.newSample(ctx)
				fourthSample.startTime = thirdSample.startTime.Add(1 * time.Second)
				fourthSample.setSampleError(fmt.Errorf("now fail"))

				close(fourthSample.finished)
				close(secondSample.finished)
				close(firstSample.finished)
				close(thirdSample.finished)
			},
			validateSamples: func(t *testing.T, eventIntervals []monitorapi.EventInterval) error {
				if len(eventIntervals) != 2 {
					t.Fatal(eventIntervals)
				}
				if duration := eventIntervals[0].To.Sub(eventIntervals[0].From); duration != 2*time.Second {
					t.Error(eventIntervals[0])
				}
				if !strings.Contains(eventIntervals[0].Message, "started responding") {
					t.Error(eventIntervals[0])
				}
				if duration := eventIntervals[1].To.Sub(eventIntervals[1].From); duration != 2*time.Second {
					t.Error(eventIntervals[0])
				}
				if !strings.Contains(eventIntervals[1].Message, "now fail") {
					t.Error(eventIntervals[1])
				}
				return nil
			},
		},
		{
			name:          "new-interval-on-different-message",
			estimatedTime: 1 * time.Second,
			produceSamples: func(ctx context.Context, backendSampler *disruptionSampler) {
				now := time.Now()
				firstSample := backendSampler.newSample(ctx)
				firstSample.startTime = now
				firstSample.setSampleError(fmt.Errorf("early"))

				secondSample := backendSampler.newSample(ctx)
				secondSample.startTime = firstSample.startTime.Add(1 * time.Second)
				secondSample.setSampleError(fmt.Errorf("early"))

				thirdSample := backendSampler.newSample(ctx)
				thirdSample.startTime = secondSample.startTime.Add(1 * time.Second)
				thirdSample.setSampleError(fmt.Errorf("late"))

				fourthSample := backendSampler.newSample(ctx)
				fourthSample.startTime = thirdSample.startTime.Add(1 * time.Second)
				fourthSample.setSampleError(fmt.Errorf("late"))

				close(fourthSample.finished)
				close(secondSample.finished)
				close(firstSample.finished)
				close(thirdSample.finished)
			},
			validateSamples: func(t *testing.T, eventIntervals []monitorapi.EventInterval) error {
				if len(eventIntervals) != 2 {
					t.Fatal(eventIntervals)
				}
				if duration := eventIntervals[0].To.Sub(eventIntervals[0].From); duration != 2*time.Second {
					t.Error(eventIntervals[0])
				}
				if !strings.Contains(eventIntervals[0].Message, "early") {
					t.Error(eventIntervals[0])
				}
				if duration := eventIntervals[1].To.Sub(eventIntervals[1].From); duration != 2*time.Second {
					t.Error(eventIntervals[0])
				}
				if !strings.Contains(eventIntervals[1].Message, "late") {
					t.Error(eventIntervals[1])
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			parent := NewSimpleBackend("host", "backend", "path", NewConnectionType)
			backendSampler := newDisruptionSampler(parent)
			interval := 1 * time.Second
			monitor := newSimpleMonitor()
			fakeEventRecorder := events.NewFakeRecorder(100)
			go func() {
				backendSampler.consumeSamples(ctx, interval, monitor, fakeEventRecorder)
			}()

			// now we start supplying the samples
			tt.produceSamples(ctx, backendSampler)
			time.Sleep(2 * time.Second) // wait just a bit for the consumption to happen before cancelling. this must be longer than the interval above
			cancel()
			time.Sleep(1 * time.Second) // wait just a bit for the consumption finish and complete the deferal

			tt.validateSamples(t, monitor.Intervals(time.Time{}, time.Time{}))
		})
	}
}