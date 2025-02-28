package auditloganalyzer

import (
	"fmt"
	"github.com/openshift/origin/pkg/dataloader"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
)

var threshold = 9.0

type auditLatencyRecords struct {
	lock    sync.Mutex
	matcher *regexp.Regexp
	records []auditLatencyRecord
}

type auditLatencyRecord struct {
	auditId   string
	resource  string
	namespace string
	name      string
	username  string
	latency   float64
}

func CheckForLatency() *auditLatencyRecords {

	decimalSeconds, err := regexp.Compile("([\\d\\.]+)s")
	if err != nil {
		panic(err)
	}

	return &auditLatencyRecords{matcher: decimalSeconds, records: make([]auditLatencyRecord, 0)}
}

func (v *auditLatencyRecords) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	// "apiserver.latency.k8s.io/total":"16.005122724s"
	if totalLatency, ok := auditEvent.Annotations["apiserver.latency.k8s.io/total"]; ok {
		// match regex
		match := v.matcher.FindStringSubmatch(totalLatency)

		if len(match) > 0 {
			seconds, err := strconv.ParseFloat(match[1], 64)
			if err == nil {
				if seconds > threshold {
					v.records = append(v.records, auditLatencyRecord{
						auditId:   string(auditEvent.AuditID),
						latency:   seconds,
						resource:  auditEvent.ObjectRef.Resource,
						namespace: auditEvent.ObjectRef.Namespace,
						name:      auditEvent.ObjectRef.Name,
						username:  auditEvent.User.Username,
					})
				}
			}
		}

	}
}

func (v *auditLatencyRecords) WriteAuditLogSummary(artifactDir, name, timeSuffix string) error {

	rows := make([]map[string]string, 0)
	for _, r := range v.records {
		data := map[string]string{"User": r.username, "Resource": r.resource, "Namespace": r.namespace, "Name": r.name, "Id": r.auditId, "Latency": strconv.FormatFloat(r.latency, 'g', 3, 64)}
		rows = append(rows, data)
	}

	dataFile := dataloader.DataFile{
		TableName: "audit_high_latency_requests",
		Schema:    map[string]dataloader.DataType{"User": dataloader.DataTypeString, "Resource": dataloader.DataTypeString, "Namespace": dataloader.DataTypeString, "Name": dataloader.DataTypeString, "Id": dataloader.DataTypeString, "Latency": dataloader.DataTypeFloat64},
		Rows:      rows,
	}
	fileName := filepath.Join(artifactDir, fmt.Sprintf("%s%s-%s", name, timeSuffix, dataloader.AutoDataLoaderSuffix))
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
	}

	return nil
}
