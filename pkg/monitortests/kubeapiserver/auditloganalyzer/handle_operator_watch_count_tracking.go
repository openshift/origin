package auditloganalyzer

import (
	"context"
	"fmt"
	"github.com/openshift/origin/pkg/dataloader"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// with https://github.com/openshift/kubernetes/pull/2113 we no longer have the counts used in
// https://github.com/openshift/origin/blob/35b3c221634bcaef9a5148477334c27d166b7838/pkg/monitortests/testframework/watchrequestcountscollector/monitortest.go#L111
// attempt to recreate via audit events

type watchCountChecker struct {
	lock                  sync.Mutex
	startTime             time.Time
	watchRequestCountsMap map[OperatorKey]*RequestCount
}

func NewWatchCountChecker() *watchCountChecker {
	return &watchCountChecker{startTime: time.Now(), watchRequestCountsMap: map[OperatorKey]*RequestCount{}}
}

type OperatorKey struct {
	NodeName string
	Operator string
	Hour     int
}

type RequestCount struct {
	Count    int64
	Operator string
	NodeName string
	Hour     int
}

func (s *watchCountChecker) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	// only want verb watch and response complete events
	if auditEvent.Verb != "watch" || auditEvent.Stage != auditv1.StageResponseComplete || !strings.HasSuffix(auditEvent.User.Username, "-operator") {
		return
	}

	requestHour := int(auditEvent.RequestReceivedTimestamp.Sub(s.startTime).Round(time.Hour))
	key := OperatorKey{Hour: requestHour, Operator: auditEvent.User.Username, NodeName: nodeName}

	var counter *RequestCount
	var ok bool
	if counter, ok = s.watchRequestCountsMap[key]; !ok {
		counter = &RequestCount{NodeName: key.NodeName, Hour: key.Hour, Operator: key.Operator, Count: 0}
		s.watchRequestCountsMap[key] = counter
	}
	counter.Count++

}

func (s *watchCountChecker) WriteAuditLogSummary(ctx context.Context, artifactDir, name, timeSuffix string) error {
	oc := exutil.NewCLIWithoutNamespace(name)

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		logrus.WithError(err).Warn("unable to get cluster infrastructure")
		return nil
	}

	// take maximum from all hours through all nodes
	watchRequestCountsMapMax := map[OperatorKey]*RequestCount{}
	for _, requestCount := range s.watchRequestCountsMap {
		key := OperatorKey{
			Operator: requestCount.Operator,
		}
		if _, exists := watchRequestCountsMapMax[key]; exists {
			if watchRequestCountsMapMax[key].Count < requestCount.Count {
				watchRequestCountsMapMax[key].Count = requestCount.Count
				watchRequestCountsMapMax[key].NodeName = requestCount.NodeName
				watchRequestCountsMapMax[key].Hour = requestCount.Hour
			}
		} else {
			watchRequestCountsMapMax[key] = requestCount
		}
	}

	// sort the requests counts so it's easy to see the biggest offenders
	watchRequestCounts := []*RequestCount{}
	for _, requestCount := range watchRequestCountsMapMax {
		watchRequestCounts = append(watchRequestCounts, requestCount)
	}

	sort.Slice(watchRequestCounts, func(i int, j int) bool {
		return watchRequestCounts[i].Count > watchRequestCounts[j].Count
	})

	// infra.Status.ControlPlaneTopology, infra.Spec.PlatformSpec.Type, operator, value
	rows := make([]map[string]string, 0)
	for _, item := range watchRequestCounts {
		operator := strings.Split(item.Operator, ":")[3]
		rows = append(rows, map[string]string{"ControlPlaneTopology": string(infra.Status.ControlPlaneTopology), "PlatformType": string(infra.Spec.PlatformSpec.Type), "Operator": operator, "WatchRequestCount": strconv.FormatInt(item.Count, 10)})
	}

	dataFile := dataloader.DataFile{
		TableName: "operator_watch_requests",
		Schema:    map[string]dataloader.DataType{"ControlPlaneTopology": dataloader.DataTypeString, "PlatformType": dataloader.DataTypeString, "Operator": dataloader.DataTypeString, "WatchRequestCount": dataloader.DataTypeInteger},
		Rows:      rows,
	}
	fileName := filepath.Join(artifactDir, fmt.Sprintf("operator-%s%s-%s", name, timeSuffix, dataloader.AutoDataLoaderSuffix))
	err = dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
		return nil
	}

	return nil
}
