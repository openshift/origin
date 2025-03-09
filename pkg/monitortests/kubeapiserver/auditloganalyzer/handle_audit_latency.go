package auditloganalyzer

import (
	"encoding/json"
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
	buckets      = []float64{45.0, 30.0, 20.0, 10.0, 5.0, 2.0, 1.0}
	latencyTypes = []string{"apiserver.latency.k8s.io/total", "apiserver.latency.k8s.io/etcd"}
	//	knownVerbs   = []string{"get", "list", "watch", "post", "patch", "update", "create", "delete"}
)

type auditLatencyRecords struct {
	lock           sync.Mutex
	matcher        *regexp.Regexp
	records        []auditLatencyRecord
	summary        auditLatencySummary
	persistRecords bool
	persistJson    bool
}

type auditLatencyRecord struct {
	auditId   string
	resource  string
	namespace string
	name      string
	username  string
	latency   float64
	verb      string
	json      []byte
}

type auditLatencyBucket struct {
	totalCounts map[string]int64
}

type auditLatencySummaryRecord struct {
	buckets map[float64]*auditLatencyBucket
}

type auditLatencySummary struct {
	resourceBuckets map[string]map[string]*auditLatencySummaryRecord
}

func CheckForLatency(persistRecords, persistJson bool) *auditLatencyRecords {

	decimalSeconds, err := regexp.Compile("([\\d\\.]+)s")
	if err != nil {
		panic(err)
	}

	summary := auditLatencySummary{resourceBuckets: make(map[string]map[string]*auditLatencySummaryRecord)}
	return &auditLatencyRecords{matcher: decimalSeconds, records: make([]auditLatencyRecord, 0), summary: summary, persistRecords: persistRecords, persistJson: persistJson}
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
					if v.persistRecords && seconds > threshold {
						if auditEvent.ObjectRef != nil {
							var bytes []byte
							if v.persistJson {
								// best attempt
								bytes, _ = json.Marshal(auditEvent)
							}

							v.records = append(v.records, auditLatencyRecord{
								auditId:   string(auditEvent.AuditID),
								latency:   seconds,
								resource:  auditEvent.ObjectRef.Resource,
								namespace: auditEvent.ObjectRef.Namespace,
								name:      auditEvent.ObjectRef.Name,
								username:  auditEvent.User.Username,
								verb:      auditEvent.Verb,
								json:      bytes,
							})
						}
					}
					for _, b := range buckets {
						if seconds > b {
							if latencyBucket, ok = summaryRecord.buckets[b]; !ok {
								newLatencyBucket := &auditLatencyBucket{make(map[string]int64)}
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
					newLatencyBucket := &auditLatencyBucket{make(map[string]int64)}
					summaryRecord.buckets[minBucket] = newLatencyBucket
					latencyBucket = newLatencyBucket
				}
			}

			verb := "default"
			if len(auditEvent.Verb) > 0 {
				verb = auditEvent.Verb
			}

			latencyBucket.totalCounts[verb]++
		}
	}
}

func (v *auditLatencyRecords) WriteAuditLogSummary(artifactDir, name, timeSuffix string) error {
	var dataFile dataloader.DataFile
	var fileName string
	var err error
	var rows []map[string]string

	if v.persistRecords {
		rows := make([]map[string]string, 0)
		for _, r := range v.records {
			data := map[string]string{"User": r.username, "Resource": r.resource, "Namespace": r.namespace, "Name": r.name, "Id": r.auditId, "Latency": fmt.Sprintf("%.3f", r.latency)}
			if v.persistJson && r.json != nil {
				data["JSON"] = string(r.json)
			}
			rows = append(rows, data)
		}

		dataFile = dataloader.DataFile{
			TableName: "high_audit_latency_requests",
			Schema:    map[string]dataloader.DataType{"User": dataloader.DataTypeString, "Resource": dataloader.DataTypeString, "Namespace": dataloader.DataTypeString, "Name": dataloader.DataTypeString, "Id": dataloader.DataTypeString, "Latency": dataloader.DataTypeFloat64, "JSON": dataloader.DataTypeJSON},
			Rows:      rows,
		}
		fileName := filepath.Join(artifactDir, fmt.Sprintf("%s-high-requests%s-%s", name, timeSuffix, dataloader.AutoDataLoaderSuffix))
		err := dataloader.WriteDataFile(fileName, dataFile)
		if err != nil {
			logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
		}
	}

	rows = make([]map[string]string, 0)
	reportBuckets := buckets
	reportBuckets = append(reportBuckets, 0.0)
	for latencyType, latencyRecords := range v.summary.resourceBuckets {
		for resource, record := range latencyRecords {
			for _, bucket := range reportBuckets {

				if rv, ok := record.buckets[bucket]; ok {
					foundVerbs := make(map[string]bool)
					for verb, count := range rv.totalCounts {
						// we want the default 0s eventually but for now only record entries we have seen
						data := map[string]string{"LatencyType": latencyType, "Resource": resource, "Verb": verb, "Bucket": fmt.Sprintf("%.0f", bucket), "Count": fmt.Sprintf("%d", count)}
						rows = append(rows, data)
						foundVerbs[verb] = true
					}

					//for _,verb := range knownVerbs{
					//	if !foundVerbs[verb] {
					//		data := map[string]string{"LatencyType": latencyType, "Resource": resource, "Verb": verb, "Bucket": fmt.Sprintf("%.0f", bucket), "Count": fmt.Sprintf("%d", 0)}
					//		rows = append(rows, data)
					//	}
					//}

				} else {
					//for _,verb := range knownVerbs{
					//				data := map[string]string{"LatencyType": latencyType, "Resource": resource, "Verb": verb, "Bucket": fmt.Sprintf("%.0f", bucket), "Count": fmt.Sprintf("%d", 0)}
					//				rows = append(rows, data)
					//}
				}

			}
		}
	}

	dataFile = dataloader.DataFile{
		TableName: "audit_latency_counts",
		Schema:    map[string]dataloader.DataType{"LatencyType": dataloader.DataTypeString, "Resource": dataloader.DataTypeString, "Verb": dataloader.DataTypeString, "Bucket": dataloader.DataTypeFloat64, "Count": dataloader.DataTypeInteger},
		Rows:      rows,
	}
	fileName = filepath.Join(artifactDir, fmt.Sprintf("%s-summary-counts%s-%s", name, timeSuffix, dataloader.AutoDataLoaderSuffix))
	err = dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
	}

	return nil
}
