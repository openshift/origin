package apiserveravailability

import (
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/klog/v2"
)

type SummarizationFunc func(locator, line string)

type APIServerClientAccessFailureSummary struct {
	lock                   sync.Mutex
	WriteOperationFailures []monitorapi.Interval
}

func timeFromPodLogTime(line string) time.Time {
	tokens := strings.Split(line, " ")
	timeString := tokens[0]
	t, err := time.Parse(time.RFC3339Nano, timeString)
	if err != nil {
		klog.Error(err)
		return time.Now()
	}

	return t
}

func (s *APIServerClientAccessFailureSummary) SummarizeLine(locator monitorapi.Locator, line string) {
	if strings.Contains(line, "write: operation not permitted") {
		timeOfLog := timeFromPodLogTime(line)
		// TODO collapse all in the same second into a single interval
		event := monitorapi.Interval{
			Condition: monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Warning).
				Locator(locator).
				Message(monitorapi.NewMessage().Reason(monitorapi.IPTablesNotPermitted).HumanMessage(line)).
				BuildCondition(),
			From: timeOfLog,
			To:   timeOfLog.Add(1 * time.Second),
		}
		s.WriteOperationFailures = append(s.WriteOperationFailures, event)
	}
}

func (s *APIServerClientAccessFailureSummary) AddSummary(rhs *APIServerClientAccessFailureSummary) {
	if rhs == nil {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()

	rhs.lock.Lock()
	defer rhs.lock.Unlock()

	s.WriteOperationFailures = append(s.WriteOperationFailures, rhs.WriteOperationFailures...)
}
