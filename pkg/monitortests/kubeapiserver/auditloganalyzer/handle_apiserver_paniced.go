package auditloganalyzer

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type panicEvent struct {
	auditID   types.UID
	timestamp time.Time
}

func (p panicEvent) String() string {
	return fmt.Sprintf("auditID %s at %s", p.auditID, p.timestamp.String())
}

type PanicEventByTimestamp []panicEvent

func (n PanicEventByTimestamp) Len() int {
	return len(n)
}
func (n PanicEventByTimestamp) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n PanicEventByTimestamp) Less(i, j int) bool {
	diff := n[i].timestamp.Compare(n[j].timestamp)
	switch {
	case diff < 0:
		return true
	case diff > 0:
		return false
	}

	return strings.Compare(string(n[i].auditID), string(n[j].auditID)) < 0
}

type panicEventsForEndpoint struct {
	panicEvents map[string]sets.Set[panicEvent]
}

func NewPanicEventsForEndpoint() panicEventsForEndpoint {
	return panicEventsForEndpoint{
		panicEvents: make(map[string]sets.Set[panicEvent]),
	}
}

func (p panicEventsForEndpoint) Insert(endpoint string, pe panicEvent) {
	events, ok := p.panicEvents[endpoint]
	if !ok {
		events = sets.New[panicEvent]()
	}
	events.Insert(pe)
	p.panicEvents[endpoint] = events
}

func (p panicEventsForEndpoint) String() string {
	result := ""
	for endpoint, events := range p.panicEvents {
		sortedEvents := events.UnsortedList()
		sort.Sort(PanicEventByTimestamp(sortedEvents))
		eventsAsStrings := []string{}
		for _, event := range sortedEvents {
			eventsAsStrings = append(eventsAsStrings, event.String())
		}
		eventString := fmt.Sprintf("  %s", strings.Join(eventsAsStrings, "\n  "))
		result = fmt.Sprintf("%s\nFound %d panics for endpoint %q:\n%s", result, len(events), endpoint, eventString)
	}
	return result
}

func (p panicEventsForEndpoint) Len() int {
	sum := 0
	for _, endpoints := range p.panicEvents {
		sum += endpoints.Len()
	}
	return sum
}

type panicEventsForUserAgent struct {
	panicEvents map[string]panicEventsForEndpoint
}

func NewPanicEventsForUserAgent() panicEventsForUserAgent {
	return panicEventsForUserAgent{
		panicEvents: make(map[string]panicEventsForEndpoint),
	}
}

func (p panicEventsForUserAgent) Insert(useragent string, endpoint string, pe panicEvent) {
	events, ok := p.panicEvents[useragent]
	if !ok {
		events = NewPanicEventsForEndpoint()
	}
	events.Insert(endpoint, pe)
	p.panicEvents[useragent] = events
}

type apiserverPaniced struct {
	lock                    sync.Mutex
	panicEventsPerUserAgent panicEventsForUserAgent
}

func CheckForApiserverPaniced() *apiserverPaniced {
	return &apiserverPaniced{
		panicEventsPerUserAgent: NewPanicEventsForUserAgent(),
	}
}

func (s *apiserverPaniced) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	if auditEvent.ResponseStatus == nil {
		return
	}
	if auditEvent.ResponseStatus.Code != 500 {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	pe := panicEvent{
		auditID:   auditEvent.AuditID,
		timestamp: auditEvent.RequestReceivedTimestamp.Time,
	}
	s.panicEventsPerUserAgent.Insert(auditEvent.UserAgent, auditEvent.RequestURI, pe)
}
