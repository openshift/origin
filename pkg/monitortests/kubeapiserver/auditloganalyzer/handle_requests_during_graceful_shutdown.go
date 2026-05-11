package auditloganalyzer

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type lateRequest struct {
	auditID                  string
	requestReceivedTimestamp time.Time
	verb                     string
	requestURI               string
	username                 string
	userAgent                string
	sourceIPs                []string
	responseCode             int32
	annotationValue          string
	// elapsedSinceShutdown is populated by correlating with GracefulAPIServerShutdown intervals.
	// Negative duration means we could not determine.
	elapsedSinceShutdown time.Duration
}

func (r lateRequest) String() string {
	elapsed := "unknown"
	if r.elapsedSinceShutdown >= 0 {
		elapsed = fmt.Sprintf("%ds", int(r.elapsedSinceShutdown.Seconds()))
	}
	return fmt.Sprintf("auditID=%s time=%s elapsed-since-shutdown=%s verb=%s uri=%s user=%s sourceIPs=%s userAgent=%q responseCode=%d annotation=%s",
		r.auditID,
		r.requestReceivedTimestamp.UTC().Format(time.RFC3339),
		elapsed,
		r.verb,
		r.requestURI,
		r.username,
		strings.Join(r.sourceIPs, ","),
		r.userAgent,
		r.responseCode,
		r.annotationValue,
	)
}

type lateRequestTracking struct {
	lock         sync.Mutex
	lateRequests []lateRequest
}

func CheckForRequestsDuringShutdown() *lateRequestTracking {
	return &lateRequestTracking{}
}

func (l *lateRequestTracking) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}
	if value, ok := auditEvent.Annotations["openshift.io/during-graceful"]; ok {
		// Ignore disruptions done over loopback interface
		value_slice := strings.Split(value, ",")
		if len(value_slice) == 0 {
			return
		}
		if value_slice[0] == "loopback=true" {
			return
		}
		var responseCode int32
		if auditEvent.ResponseStatus != nil {
			responseCode = auditEvent.ResponseStatus.Code
		}
		l.lateRequests = append(l.lateRequests, lateRequest{
			auditID:                  string(auditEvent.AuditID),
			requestReceivedTimestamp: auditEvent.RequestReceivedTimestamp.Time,
			verb:                     auditEvent.Verb,
			requestURI:               auditEvent.RequestURI,
			username:                 auditEvent.User.Username,
			userAgent:                auditEvent.UserAgent,
			sourceIPs:                auditEvent.SourceIPs,
			responseCode:             responseCode,
			annotationValue:          value,
			elapsedSinceShutdown:     -1, // unknown until correlated
		})
	}
}

// correlateWithShutdownIntervals matches each late request against GracefulAPIServerShutdown
// intervals and computes how many seconds after shutdown start the request arrived.
func (l *lateRequestTracking) correlateWithShutdownIntervals(finalIntervals monitorapi.Intervals) {
	shutdownIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.GracefulAPIServerShutdown
	})
	if len(shutdownIntervals) == 0 {
		return
	}

	for i := range l.lateRequests {
		reqTime := l.lateRequests[i].requestReceivedTimestamp
		for _, shutdown := range shutdownIntervals {
			if !reqTime.Before(shutdown.From) && !reqTime.After(shutdown.To) {
				l.lateRequests[i].elapsedSinceShutdown = reqTime.Sub(shutdown.From)
				break
			}
		}
	}
}
