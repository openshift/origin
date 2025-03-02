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

var (
	threshold    = 14.0
	minBucket    = 0.0
	buckets      = []float64{45.0, 30.0, 20.0, 10.0}
	latencyTypes = []string{"apiserver.latency.k8s.io/total", "apiserver.latency.k8s.io/etcd"}
)

type auditLatencyRecords struct {
	lock    sync.Mutex
	matcher *regexp.Regexp
	records []auditLatencyRecord
	summary auditLatencySummary
}

type auditLatencyRecord struct {
	auditId   string
	resource  string
	namespace string
	name      string
	username  string
	latency   float64
}

type auditLatencyBucket struct {
	totalCount int64
}

type auditLatencySummaryRecord struct {
	buckets map[float64]*auditLatencyBucket
}

type auditLatencySummary struct {
	resourceBuckets map[string]map[string]*auditLatencySummaryRecord
}

func CheckForLatency() *auditLatencyRecords {

	decimalSeconds, err := regexp.Compile("([\\d\\.]+)s")
	if err != nil {
		panic(err)
	}

	summary := auditLatencySummary{resourceBuckets: make(map[string]map[string]*auditLatencySummaryRecord)}
	return &auditLatencyRecords{matcher: decimalSeconds, records: make([]auditLatencyRecord, 0), summary: summary}
}

func (v *auditLatencyRecords) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	// "apiserver.latency.k8s.io/total":"16.005122724s"
	for _, latencyType := range latencyTypes {
		if totalLatency, ok := auditEvent.Annotations[latencyType]; ok {

			resourceType := "unknown"
			if auditEvent.ObjectRef != nil && len(auditEvent.ObjectRef.Resource) > 0 {
				resourceType = auditEvent.ObjectRef.Resource
			}

			var typeRecord map[string]*auditLatencySummaryRecord
			var summaryRecord *auditLatencySummaryRecord
			var latencyBucket *auditLatencyBucket

			if typeRecord, ok = v.summary.resourceBuckets[latencyType]; !ok {
				newTypeRecord := make(map[string]*auditLatencySummaryRecord)
				v.summary.resourceBuckets[latencyType] = newTypeRecord
				typeRecord = newTypeRecord
			}

			if summaryRecord, ok = typeRecord[resourceType]; !ok {
				newSummaryRecord := &auditLatencySummaryRecord{buckets: make(map[float64]*auditLatencyBucket)}
				typeRecord[resourceType] = newSummaryRecord
				summaryRecord = newSummaryRecord
			}

			// match regex
			match := v.matcher.FindStringSubmatch(totalLatency)
			if len(match) > 0 {
				seconds, err := strconv.ParseFloat(match[1], 64)
				if err == nil {
					if seconds > threshold {
						if auditEvent.ObjectRef != nil {
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
					for _, b := range buckets {
						if seconds > b {
							if latencyBucket, ok = summaryRecord.buckets[b]; !ok {
								newLatencyBucket := &auditLatencyBucket{}
								summaryRecord.buckets[b] = newLatencyBucket
								latencyBucket = newLatencyBucket
							}
							break
						}
					}
				}
			}

			if latencyBucket == nil {
				if latencyBucket, ok = summaryRecord.buckets[minBucket]; !ok {
					newLatencyBucket := &auditLatencyBucket{}
					summaryRecord.buckets[minBucket] = newLatencyBucket
					latencyBucket = newLatencyBucket
				}
			}

			latencyBucket.totalCount++
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
		TableName: "high_audit_latency_requests",
		Schema:    map[string]dataloader.DataType{"User": dataloader.DataTypeString, "Resource": dataloader.DataTypeString, "Namespace": dataloader.DataTypeString, "Name": dataloader.DataTypeString, "Id": dataloader.DataTypeString, "Latency": dataloader.DataTypeFloat64},
		Rows:      rows,
	}
	fileName := filepath.Join(artifactDir, fmt.Sprintf("%s-high-requests%s-%s", name, timeSuffix, dataloader.AutoDataLoaderSuffix))
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
	}

	rows = make([]map[string]string, 0)
	for latencyType, latencyRecords := range v.summary.resourceBuckets {
		for resource, record := range latencyRecords {
			for rk, rv := range record.buckets {
				data := map[string]string{"LatencyType": latencyType, "Resource": resource, "Bucket": strconv.FormatFloat(rk, 'g', 1, 64), "Count": strconv.FormatInt(rv.totalCount, 10)}
				rows = append(rows, data)
			}
		}
	}

	dataFile = dataloader.DataFile{
		TableName: "audit_latency_counts",
		Schema:    map[string]dataloader.DataType{"LatencyType": dataloader.DataTypeString, "Resource": dataloader.DataTypeString, "Bucket": dataloader.DataTypeFloat64, "Count": dataloader.DataTypeInteger},
		Rows:      rows,
	}
	fileName = filepath.Join(artifactDir, fmt.Sprintf("%ssummary-counts%s-%s", name, timeSuffix, dataloader.AutoDataLoaderSuffix))
	err = dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
	}

	return nil
}
