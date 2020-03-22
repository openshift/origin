package loglevel

import (
	"context"
	"strings"
	"sync"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

type fakeLogger struct {
	verbosity klog.Level
}

func (l *fakeLogger) V(v klog.Level) klog.Verbose {
	return l.verbosity == v
}

var fakeLog = &fakeLogger{verbosity: 0}

func init() {
	verbosityFn = fakeLog.V
}

func TestClusterOperatorLoggingController(t *testing.T) {
	tests := []struct {
		name              string
		operatorSpec      operatorv1.OperatorSpec
		evalEvents        func([]*corev1.Event, *testing.T)
		startingVerbosity klog.Level
		expectedVerbosity klog.Level
		retrySyncTimes    int
	}{
		{
			name: "when OperatorLogLevel is set to Debug operator must set V(4)",
			operatorSpec: operatorv1.OperatorSpec{
				OperatorLogLevel: "Debug",
			},
			startingVerbosity: 0,
			expectedVerbosity: 4,
		},
		{
			name: "when OperatorLogLevel is set to Debug operator must set V(4) when it is currently V(2)",
			operatorSpec: operatorv1.OperatorSpec{
				OperatorLogLevel: "Debug",
			},
			startingVerbosity: 2,
			expectedVerbosity: 4,
			retrySyncTimes:    5,
			evalEvents: func(events []*corev1.Event, t *testing.T) {
				if len(events) != 1 {
					t.Errorf("expected exactly one event, got %d", len(events))
					return
				}
				if !strings.Contains(events[0].Message, `Operator log level changed from "Normal" to "Debug"`) {
					t.Errorf("expected message to be %q, got %q", `Operator log level changed from "Normal" to "Debug"`, events[0].Message)
				}
			},
		},
		{
			name: "when OperatorLogLevel is set to Debug operator must stay on V(4)",
			operatorSpec: operatorv1.OperatorSpec{
				OperatorLogLevel: "Debug",
			},
			retrySyncTimes:    5,
			startingVerbosity: 4,
			expectedVerbosity: 4,
			evalEvents: func(events []*corev1.Event, t *testing.T) {
				if len(events) != 0 {
					t.Errorf("expected no events, got %d", len(events))
				}
			},
		},
		{
			name: "when OperatorLogLevel is set to Unknown operator must set V(2)",
			operatorSpec: operatorv1.OperatorSpec{
				OperatorLogLevel: "Unknown",
			},
			startingVerbosity: 4,
			expectedVerbosity: 2,
		},
		{
			name: "when OperatorLogLevel is set to Normal operator must set V(2) once",
			operatorSpec: operatorv1.OperatorSpec{
				OperatorLogLevel: "Normal",
			},
			retrySyncTimes:    5,
			expectedVerbosity: 2,
		},
		{
			name: "when OperatorLogLevel is not set operator must set default V(2)",
			operatorSpec: operatorv1.OperatorSpec{
				OperatorLogLevel: "",
			},
			startingVerbosity: 0,
			expectedVerbosity: 2,
		},
		{
			name: "when OperatorLogLevel is not set operator must set default V(2) just once",
			operatorSpec: operatorv1.OperatorSpec{
				OperatorLogLevel: "",
			},
			retrySyncTimes:    5,
			startingVerbosity: 0,
			expectedVerbosity: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// always rest the log level
			fakeLog.verbosity = test.startingVerbosity
			fakeStaticPodOperatorClient := v1helpers.NewFakeOperatorClient(
				&test.operatorSpec,
				&operatorv1.OperatorStatus{},
				nil,
			)
			setLogLevel := func(level operatorv1.LogLevel) error {
				(&sync.Once{}).Do(func() {
					fakeLog.verbosity = klog.Level(LogLevelToVerbosity(level))
				})
				return nil
			}
			recorder := events.NewInMemoryRecorder("")

			c := &LogLevelController{
				operatorClient: fakeStaticPodOperatorClient,
				setLogLevelFn:  setLogLevel,
				getLogLevelFn:  GetLogLevel,
			}
			syncCtx := factory.NewSyncContext("LoggingController", recorder)
			for i := 0; i <= test.retrySyncTimes; i++ {
				if err := c.sync(context.TODO(), syncCtx); err != nil {
					t.Errorf("sync failed: %v", err)
					return
				}
			}
			if test.expectedVerbosity != fakeLog.verbosity {
				t.Errorf("expected log level %d to be set, got %d", test.expectedVerbosity, fakeLog.verbosity)
			}
			if test.evalEvents != nil {
				test.evalEvents(recorder.Events(), t)
			}
		})
	}
}
