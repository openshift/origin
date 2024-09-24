package auditloganalyzer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type CountForSecond struct {
	// NumberOfConcurrentRequestsBeingHandled is the sum of requests that were ongoing (minus watches I think, I'm not sure).
	// That includes
	// 1. requests that were started before this second and not completed until during or after this second
	// 2. requests that were started this second
	NumberOfConcurrentRequestsBeingHandled atomic.Int32

	NumberOfRequestsReceived atomic.Int32

	// NumberOfRequestsReceivedThatLaterGot500 is calculated based on when the request was received instead of completed
	// because different requests have different timeouts, so it's more useful to categorize based on received time
	NumberOfRequestsReceivedThatLaterGot500 atomic.Int32
}

type CountsForRun struct {
	NumberOfSeconds int

	// CountsForEachSecond is a count for every second of a job run.  Every index is one second long and the count starts
	// with the creation time of the ClusterVersion resource.
	CountsForEachSecond []CountForSecond

	EstimatedStartOfCluster metav1.Time
	LastAcceptedTime        metav1.Time
}

type countTracking struct {
	lock         sync.Mutex
	CountsForRun CountsForRun
}

func CountsOverTime(estimatedStartOfCluster metav1.Time) *countTracking {
	numberOfSecondsToTrack := 6 * 60 * 60 // six hours

	return &countTracking{
		CountsForRun: CountsForRun{
			NumberOfSeconds:         numberOfSecondsToTrack,
			CountsForEachSecond:     make([]CountForSecond, numberOfSecondsToTrack),
			EstimatedStartOfCluster: estimatedStartOfCluster,
			LastAcceptedTime:        metav1.Time{Time: estimatedStartOfCluster.Add(time.Duration(numberOfSecondsToTrack-1) * time.Second)},
		},
	}
}

func (s *countTracking) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime) {
	receivedIndex, completionIndex, httpStatusCode, include := s.CountsForRun.countIndexesFromAuditTime(auditEvent)
	if !include {
		return
	}

	s.CountsForRun.CountsForEachSecond[receivedIndex].NumberOfRequestsReceived.Add(1)
	if httpStatusCode == 500 {
		s.CountsForRun.CountsForEachSecond[receivedIndex].NumberOfRequestsReceivedThatLaterGot500.Add(1)
	}

	for i := receivedIndex; i <= completionIndex; i++ {
		s.CountsForRun.CountsForEachSecond[receivedIndex].NumberOfConcurrentRequestsBeingHandled.Add(1)
	}
}

// returns received index, completion index, status code, and false if the index is out of bounds
func (c *CountsForRun) countIndexesFromAuditTime(in *auditv1.Event) (int, int, int, bool) {
	if in.Stage != auditv1.StageResponseComplete {
		return -1, -1, -1, false
	}
	if in.Verb == "watch" {
		return -1, -1, -1, false
	}
	if in.RequestReceivedTimestamp.BeforeTime(&c.EstimatedStartOfCluster) {
		return -1, -1, -1, false
	}
	if in.RequestReceivedTimestamp.After(c.LastAcceptedTime.Time) {
		return -1, -1, -1, false
	}

	receivedIndex := int(in.RequestReceivedTimestamp.Sub(c.EstimatedStartOfCluster.Time).Round(time.Second).Seconds())
	completionIndex := int(in.StageTimestamp.Sub(c.EstimatedStartOfCluster.Time).Round(time.Second).Seconds())
	if completionIndex >= c.NumberOfSeconds {
		completionIndex = c.NumberOfSeconds - 1
	}

	statusCode := 0
	if in.ResponseStatus != nil {
		statusCode = int(in.ResponseStatus.Code)
	}

	return receivedIndex, completionIndex, statusCode, true
}

func (c *CountsForRun) ToCSV() ([]byte, error) {
	out := &bytes.Buffer{}
	csvWriter := csv.NewWriter(out)
	if err := csvWriter.Write([]string{"seconds after cluster start", "number of requests being handled", "number of requests received", "number of requests received this second getting 500"}); err != nil {
		return nil, fmt.Errorf("failed writing headers: %w", err)
	}

	for i, curr := range c.CountsForEachSecond {
		record := []string{
			strconv.Itoa(i),
			strconv.Itoa(int(curr.NumberOfConcurrentRequestsBeingHandled.Load())),
			strconv.Itoa(int(curr.NumberOfRequestsReceived.Load())),
			strconv.Itoa(int(curr.NumberOfRequestsReceivedThatLaterGot500.Load())),
		}

		if err := csvWriter.Write(record); err != nil {
			return nil, fmt.Errorf("failed writing record %d: %w", i, err)
		}
	}
	csvWriter.Flush()

	return out.Bytes(), nil
}
