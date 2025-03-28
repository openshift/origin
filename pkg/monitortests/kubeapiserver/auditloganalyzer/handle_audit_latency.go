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
	minBucket    = 0.0
	buckets      = []float64{45.0, 30.0, 20.0, 10.0, 5.0, 2.0, 1.0}
	latencyTypes = []string{"apiserver.latency.k8s.io/total", "apiserver.latency.k8s.io/etcd"}
	knownVerbs   = []string{"get", "list", "watch", "post", "patch", "update", "create", "delete"}
)

type auditLatencyRecords struct {
	lock    sync.Mutex
	matcher *regexp.Regexp
	summary auditLatencySummary
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

func CheckForLatency() *auditLatencyRecords {

	decimalSeconds, err := regexp.Compile("([\\d\\.]+)s")
	if err != nil {
		panic(err)
	}

	summary := auditLatencySummary{resourceBuckets: make(map[string]map[string]*auditLatencySummaryRecord)}
	return &auditLatencyRecords{matcher: decimalSeconds, summary: summary}
}

// HandleAuditLogEvent looks for latency annotations and increments counter for all latency buckets that are below
// the latency value.  e.g a 4s latency will increment the 2,1 & 0 bucket count values.  0 is the min and will effectively be
// all requests.  Data is collected to analyze over time for increases in latency completing requests
func (v *auditLatencyRecords) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime) {

	// we only want to count the response complete events
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) || auditEvent.Stage != auditv1.StageResponseComplete {
		return
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	resourceType := "unknown"
	if auditEvent.ObjectRef != nil && len(auditEvent.ObjectRef.Resource) > 0 {
		resourceType = auditEvent.ObjectRef.Resource
	}

	// rare but not always there...
	verb := "missing"
	if len(auditEvent.Verb) > 0 {
		verb = auditEvent.Verb
	}

	// "apiserver.latency.k8s.io/total":"16.005122724s"
	for _, latencyType := range latencyTypes {
		// https://github.com/openshift/kubernetes/blob/2b03f04ce589a57cf80b2153c7e5056c53c374d3/staging/src/k8s.io/apiserver/pkg/endpoints/filters/audit.go#L156
		// we only annotate requests over 500ms so default missing apiserver.latency.k8s.io/total to the minBucket if it isn't present
		if totalLatency, ok := auditEvent.Annotations[latencyType]; ok || latencyType == "apiserver.latency.k8s.io/total" {

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
					for _, b := range buckets {
						// for any bucket that the parsed seconds exceeds we increment the latency count
						// indicating the request took more than the bucketed value of time
						if seconds > b {
							if latencyBucket, ok = summaryRecord.buckets[b]; !ok {
								newLatencyBucket := &auditLatencyBucket{make(map[string]int64)}
								summaryRecord.buckets[b] = newLatencyBucket
								latencyBucket = newLatencyBucket
							}
							latencyBucket.totalCounts[verb]++
						}
					}
				}
			}

			// every request updates the minBucket
			if latencyBucket, ok = summaryRecord.buckets[minBucket]; !ok {
				newLatencyBucket := &auditLatencyBucket{make(map[string]int64)}
				summaryRecord.buckets[minBucket] = newLatencyBucket
				latencyBucket = newLatencyBucket
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

	rows = make([]map[string]string, 0)
	reportBuckets := buckets
	reportBuckets = append(reportBuckets, 0.0)
	for latencyType, latencyRecords := range v.summary.resourceBuckets {
		for resource, record := range latencyRecords {
			for _, bucket := range reportBuckets {

				if rv, ok := record.buckets[bucket]; ok {
					foundVerbs := make(map[string]bool)
					for verb, count := range rv.totalCounts {
						data := map[string]string{"LatencyType": latencyType, "Resource": resource, "Verb": verb, "Bucket": fmt.Sprintf("%.0f", bucket), "Count": fmt.Sprintf("%d", count)}
						rows = append(rows, data)
						foundVerbs[verb] = true
					}

					for _, verb := range knownVerbs {
						if !foundVerbs[verb] {
							data := map[string]string{"LatencyType": latencyType, "Resource": resource, "Verb": verb, "Bucket": fmt.Sprintf("%.0f", bucket), "Count": fmt.Sprintf("%d", 0)}
							rows = append(rows, data)
						}
					}

				} else {
					for _, verb := range knownVerbs {
						data := map[string]string{"LatencyType": latencyType, "Resource": resource, "Verb": verb, "Bucket": fmt.Sprintf("%.0f", bucket), "Count": fmt.Sprintf("%d", 0)}
						rows = append(rows, data)
					}
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
