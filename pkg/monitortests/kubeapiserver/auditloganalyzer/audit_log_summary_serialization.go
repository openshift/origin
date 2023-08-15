package auditloganalyzer

import (
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// These types exist because JSON cannot represent compound keys in maps and schema.GroupVersionResource is a compound
// key in many of our maps.  These types handle serialization and consistent ordering.

type SerializedAuditLogSummary struct {
	LineReadFailureCount      int
	RequestCounts             SerializedRequestCounts
	PerUserRequestCount       []SerializedPerUserRequestCount
	PerResourceRequestCount   []SerializedPerResourceRequestCount
	PerHTTPStatusRequestCount []SerializedPerHTTPStatusRequestCount
}

type SerializedRequestCounts struct {
	RequestStartedCount       int
	RequestFinishedCount      int
	ClientFailedRequestCount  int
	ServerFailedRequestCount  int
	PerHTTPStatusRequestCount []SerializedPerHTTPStatusCount
}

func mostRequestsFirst(lhs, rhs SerializedRequestCounts) bool {
	return lhs.RequestFinishedCount > rhs.RequestFinishedCount
}

type SerializedPerHTTPStatusCount struct {
	HTTPStatus int32
	Count      int
}

type httpStatusByBiggestCount []SerializedPerHTTPStatusCount

func (a httpStatusByBiggestCount) Len() int      { return len(a) }
func (a httpStatusByBiggestCount) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a httpStatusByBiggestCount) Less(i, j int) bool {
	if a[i].Count > a[j].Count {
		return true
	}
	return a[i].HTTPStatus < a[j].HTTPStatus
}

type SerializedPerHTTPStatusRequestCount struct {
	HTTPStatus              int32
	RequestCounts           SerializedRequestCounts
	PerResourceRequestCount []SerializedPerResourceCountOnly
	PerUserRequestCount     []SerializedPerUserCountOnly
}

type statusCountByBiggestCount []SerializedPerHTTPStatusRequestCount

func (a statusCountByBiggestCount) Len() int      { return len(a) }
func (a statusCountByBiggestCount) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a statusCountByBiggestCount) Less(i, j int) bool {
	return mostRequestsFirst(a[i].RequestCounts, a[j].RequestCounts)
}

type SerializedPerUserRequestCount struct {
	User                    string
	RequestCounts           SerializedRequestCounts
	PerResourceRequestCount []SerializedPerResourceCountOnly
	PerVerbRequestCount     []SerializedPerVerbCountOnly
}

type userCountByBiggestCount []SerializedPerUserRequestCount

func (a userCountByBiggestCount) Len() int      { return len(a) }
func (a userCountByBiggestCount) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a userCountByBiggestCount) Less(i, j int) bool {
	return mostRequestsFirst(a[i].RequestCounts, a[j].RequestCounts)
}

type SerializedPerResourceRequestCount struct {
	GroupVersionResource schema.GroupVersionResource
	RequestCounts        SerializedRequestCounts
	PerUserRequestCount  []SerializedPerUserCountOnly
	PerVerbRequestCount  []SerializedPerVerbCountOnly
}

type resourceCountByBiggestCount []SerializedPerResourceRequestCount

func (a resourceCountByBiggestCount) Len() int      { return len(a) }
func (a resourceCountByBiggestCount) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a resourceCountByBiggestCount) Less(i, j int) bool {
	return mostRequestsFirst(a[i].RequestCounts, a[j].RequestCounts)
}

type SerializedPerVerbCountOnly struct {
	Verb          string
	RequestCounts SerializedRequestCounts
}

type verbCountOnlyByBiggestCount []SerializedPerVerbCountOnly

func (a verbCountOnlyByBiggestCount) Len() int      { return len(a) }
func (a verbCountOnlyByBiggestCount) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a verbCountOnlyByBiggestCount) Less(i, j int) bool {
	return mostRequestsFirst(a[i].RequestCounts, a[j].RequestCounts)
}

type SerializedPerUserCountOnly struct {
	User          string
	RequestCounts SerializedRequestCounts
}

type userCountOnlyByBiggestCount []SerializedPerUserCountOnly

func (a userCountOnlyByBiggestCount) Len() int      { return len(a) }
func (a userCountOnlyByBiggestCount) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a userCountOnlyByBiggestCount) Less(i, j int) bool {
	return mostRequestsFirst(a[i].RequestCounts, a[j].RequestCounts)
}

type SerializedPerResourceCountOnly struct {
	GroupVersionResource schema.GroupVersionResource
	RequestCounts        SerializedRequestCounts
}

type resourceCountOnlyByBiggestCount []SerializedPerResourceCountOnly

func (a resourceCountOnlyByBiggestCount) Len() int      { return len(a) }
func (a resourceCountOnlyByBiggestCount) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a resourceCountOnlyByBiggestCount) Less(i, j int) bool {
	return mostRequestsFirst(a[i].RequestCounts, a[j].RequestCounts)
}

func NewSerializedAuditLogSummary(summary AuditLogSummary) SerializedAuditLogSummary {
	ret := SerializedAuditLogSummary{
		LineReadFailureCount:      summary.lineReadFailureCount,
		RequestCounts:             NewSerializedRequestCounts(summary.requestCounts),
		PerUserRequestCount:       []SerializedPerUserRequestCount{},
		PerResourceRequestCount:   []SerializedPerResourceRequestCount{},
		PerHTTPStatusRequestCount: []SerializedPerHTTPStatusRequestCount{},
	}
	for _, v := range summary.perUserRequestCount {
		ret.PerUserRequestCount = append(ret.PerUserRequestCount, NewSerializedPerUserRequestCount(*v))
	}
	for _, v := range summary.perResourceRequestCount {
		ret.PerResourceRequestCount = append(ret.PerResourceRequestCount, NewSerializedPerResourceRequestCount(*v))
	}
	for _, v := range summary.perHTTPStatusRequestCount {
		ret.PerHTTPStatusRequestCount = append(ret.PerHTTPStatusRequestCount, NewSerializedPerStatusRequestCount(*v))
	}
	sort.Sort(userCountByBiggestCount(ret.PerUserRequestCount))
	sort.Sort(resourceCountByBiggestCount(ret.PerResourceRequestCount))
	sort.Sort(statusCountByBiggestCount(ret.PerHTTPStatusRequestCount))

	return ret
}

func NewSerializedRequestCounts(summary RequestCounts) SerializedRequestCounts {
	ret := SerializedRequestCounts{
		RequestStartedCount:       summary.requestStartedCount,
		RequestFinishedCount:      summary.requestFinishedCount,
		ClientFailedRequestCount:  summary.clientFailedRequestCount,
		ServerFailedRequestCount:  summary.serverFailedRequestCount,
		PerHTTPStatusRequestCount: []SerializedPerHTTPStatusCount{},
	}
	for k, v := range summary.perHTTPStatusRequestCount {
		ret.PerHTTPStatusRequestCount = append(ret.PerHTTPStatusRequestCount, SerializedPerHTTPStatusCount{
			HTTPStatus: k,
			Count:      v,
		})
	}
	sort.Sort(httpStatusByBiggestCount(ret.PerHTTPStatusRequestCount))

	return ret
}

func NewSerializedPerStatusRequestCount(summary PerHTTPStatusRequestCount) SerializedPerHTTPStatusRequestCount {
	ret := SerializedPerHTTPStatusRequestCount{
		HTTPStatus:              summary.httpStatus,
		RequestCounts:           NewSerializedRequestCounts(summary.requestCounts),
		PerResourceRequestCount: []SerializedPerResourceCountOnly{},
		PerUserRequestCount:     []SerializedPerUserCountOnly{},
	}
	for k, v := range summary.perResourceRequestCount {
		ret.PerResourceRequestCount = append(ret.PerResourceRequestCount, NewSerializedPerResourceCountOnly(k, *v))
	}
	for k, v := range summary.perUserRequestCount {
		ret.PerUserRequestCount = append(ret.PerUserRequestCount, NewSerializedPerUserCountOnly(k, *v))
	}
	sort.Sort(resourceCountOnlyByBiggestCount(ret.PerResourceRequestCount))
	sort.Sort(userCountOnlyByBiggestCount(ret.PerUserRequestCount))

	return ret
}

func NewSerializedPerUserRequestCount(summary PerUserRequestCount) SerializedPerUserRequestCount {
	ret := SerializedPerUserRequestCount{
		User:                    summary.user,
		RequestCounts:           NewSerializedRequestCounts(summary.requestCounts),
		PerResourceRequestCount: []SerializedPerResourceCountOnly{},
		PerVerbRequestCount:     []SerializedPerVerbCountOnly{},
	}
	for k, v := range summary.perResourceRequestCount {
		ret.PerResourceRequestCount = append(ret.PerResourceRequestCount, NewSerializedPerResourceCountOnly(k, *v))
	}
	for k, v := range summary.perVerbRequestCount {
		ret.PerVerbRequestCount = append(ret.PerVerbRequestCount, NewSerializedPerVerbCountOnly(k, *v))
	}
	sort.Sort(resourceCountOnlyByBiggestCount(ret.PerResourceRequestCount))
	sort.Sort(verbCountOnlyByBiggestCount(ret.PerVerbRequestCount))

	return ret
}

func NewSerializedPerResourceRequestCount(summary PerResourceRequestCount) SerializedPerResourceRequestCount {
	ret := SerializedPerResourceRequestCount{
		GroupVersionResource: summary.groupVersionResource,
		RequestCounts:        NewSerializedRequestCounts(summary.requestCounts),
		PerUserRequestCount:  []SerializedPerUserCountOnly{},
		PerVerbRequestCount:  []SerializedPerVerbCountOnly{},
	}
	for k, v := range summary.perUserRequestCount {
		ret.PerUserRequestCount = append(ret.PerUserRequestCount, NewSerializedPerUserCountOnly(k, *v))
	}
	for k, v := range summary.perVerbRequestCount {
		ret.PerVerbRequestCount = append(ret.PerVerbRequestCount, NewSerializedPerVerbCountOnly(k, *v))
	}
	sort.Sort(userCountOnlyByBiggestCount(ret.PerUserRequestCount))
	sort.Sort(verbCountOnlyByBiggestCount(ret.PerVerbRequestCount))

	return ret
}

func NewSerializedPerVerbCountOnly(verb string, count RequestCounts) SerializedPerVerbCountOnly {
	return SerializedPerVerbCountOnly{
		Verb:          verb,
		RequestCounts: NewSerializedRequestCounts(count),
	}
}
func NewSerializedPerUserCountOnly(user string, count RequestCounts) SerializedPerUserCountOnly {
	return SerializedPerUserCountOnly{
		User:          user,
		RequestCounts: NewSerializedRequestCounts(count),
	}
}
func NewSerializedPerResourceCountOnly(gvr schema.GroupVersionResource, count RequestCounts) SerializedPerResourceCountOnly {
	return SerializedPerResourceCountOnly{
		GroupVersionResource: gvr,
		RequestCounts:        NewSerializedRequestCounts(count),
	}
}
