package aggregated_logging

import (
	"github.com/openshift/origin/pkg/diagnostics/log"
	"testing"
)

const fakeProject = "someProject"

type fakeLogMessage struct {
	id       string
	logLevel log.Level
	message  string
}

type fakeDiagnostic struct {
	err      error
	messages map[string]fakeLogMessage
	test     *testing.T
}

func newFakeDiagnostic(t *testing.T) *fakeDiagnostic {
	return &fakeDiagnostic{
		messages: map[string]fakeLogMessage{},
		test:     t,
	}
}

func (f *fakeDiagnostic) dumpMessages() {
	f.test.Log("<<<<<<<< Dumping test messages >>>>>>>>")
	for id, message := range f.messages {
		f.test.Logf("id: %s, logLevel: %s, message: %s", id, message.logLevel.Name, message.message)
	}
}

func (f *fakeDiagnostic) Info(id string, message string) {
	f.messages[id] = fakeLogMessage{id, log.InfoLevel, message}
}

func (f *fakeDiagnostic) Error(id string, err error, message string) {
	f.messages[id] = fakeLogMessage{id, log.ErrorLevel, message}
}

func (f *fakeDiagnostic) Debug(id string, message string) {
	f.messages[id] = fakeLogMessage{id, log.DebugLevel, message}
}

func (f *fakeDiagnostic) Warn(id string, err error, message string) {
	f.messages[id] = fakeLogMessage{id, log.WarnLevel, message}
}

func (d *fakeDiagnostic) assertMessage(id string, missing string, level log.Level) {
	message, ok := d.messages[id]
	if !ok {
		d.test.Errorf("Unable to find message with id %s. %s", id, missing)
		return
	}
	if message.logLevel != level {
		d.test.Errorf("Exp logLevel %s for %s but got %s", level.Name, id, message.logLevel.Name)
	}
}

func (d *fakeDiagnostic) assertNoWarnings() {
	for _, message := range d.messages {

		if message.logLevel == log.WarnLevel {
			d.test.Errorf("Exp no WarnLevel log messages.")
		}
	}
}
func (d *fakeDiagnostic) assertNoErrors() {
	for _, message := range d.messages {

		if message.logLevel == log.ErrorLevel {
			d.test.Errorf("Exp no ErrorLevel log messages.")
		}
	}
}
