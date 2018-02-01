package chug_test

import (
	"fmt"
	"reflect"

	"code.cloudfoundry.org/lager/chug"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

func MatchLogEntry(entry chug.LogEntry) types.GomegaMatcher {
	return &logEntryMatcher{entry}
}

type logEntryMatcher struct {
	entry chug.LogEntry
}

func (m *logEntryMatcher) Match(actual interface{}) (success bool, err error) {
	actualEntry, ok := actual.(chug.LogEntry)
	if !ok {
		return false, fmt.Errorf("MatchLogEntry must be passed a chug.LogEntry.  Got:\n%s", format.Object(actual, 1))
	}

	return reflect.DeepEqual(m.entry.Error, actualEntry.Error) &&
		m.entry.LogLevel == actualEntry.LogLevel &&
		m.entry.Source == actualEntry.Source &&
		m.entry.Message == actualEntry.Message &&
		m.entry.Session == actualEntry.Session &&
		m.entry.Trace == actualEntry.Trace &&
		reflect.DeepEqual(m.entry.Data, actualEntry.Data), nil
}

func (m *logEntryMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to equal", m.entry)
}

func (m *logEntryMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to equal", m.entry)
}
