package loglevel

import (
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func TestClusterOperatorLoggingController(t *testing.T) {
	if err := SetVerbosityValue(operatorv1.Normal); err != nil {
		t.Fatal(err)
	}

	operatorSpec := &operatorv1.OperatorSpec{
		ManagementState: operatorv1.Managed,
	}

	fakeStaticPodOperatorClient := v1helpers.NewFakeOperatorClient(
		operatorSpec,
		&operatorv1.OperatorStatus{},
		nil,
	)

	recorder := events.NewInMemoryRecorder("")

	controller := NewClusterOperatorLoggingController(fakeStaticPodOperatorClient, recorder)

	// no-op, desired == current
	// When OperatorLogLevel is "" we assume the loglevel is Normal.
	if err := controller.sync(); err != nil {
		t.Fatal(err)
	}

	if len(recorder.Events()) > 0 {
		t.Fatalf("expected zero events, got %d", len(recorder.Events()))
	}

	// change the log level to trace should 1 emit event
	operatorSpec.OperatorLogLevel = operatorv1.Trace
	if err := controller.sync(); err != nil {
		t.Fatal(err)
	}

	if operatorEvents := recorder.Events(); len(operatorEvents) == 1 {
		expectedEventMessage := `Operator log level changed from "Normal" to "Trace"`
		if message := operatorEvents[0].Message; message != expectedEventMessage {
			t.Fatalf("expected event message %q, got %q", expectedEventMessage, message)
		}
	} else {
		t.Fatalf("expected 1 event, got %d", len(operatorEvents))
	}

	// next sync should not produce any extra event
	if err := controller.sync(); err != nil {
		t.Fatal(err)
	}

	if operatorEvents := recorder.Events(); len(operatorEvents) != 1 {
		t.Fatalf("expected 1 event recorded, got %d", len(operatorEvents))
	}

	if current := CurrentLogLevel(); current != operatorv1.Trace {
		t.Fatalf("expected log level 'Trace', got %v", current)
	}
}
