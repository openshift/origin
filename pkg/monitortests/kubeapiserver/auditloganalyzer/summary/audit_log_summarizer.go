package summary

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

var systemNode = "system:node"
var openshiftServiceAccount = "system:serviceaccount:openshift-"
var monitoredUsers = []string{"system:serviceaccount:kube-", openshiftServiceAccount, systemNode}
var filteredUsers = []string{"system:serviceaccount:openshift-must-gather"}
var mappedUsers = map[string]string{}

// every audit log summarizer is not threadsafe. The idea is that you use one per thread and
// later combine the summarizers together into an overall summary
type AuditLogSummary struct {
	lineReadFailureCount      int
	requestCounts             RequestCounts
	perUserRequestCount       map[string]*PerUserRequestCount
	perResourceRequestCount   map[schema.GroupVersionResource]*PerResourceRequestCount
	perHTTPStatusRequestCount map[int32]*PerHTTPStatusRequestCount
}

type RequestCounts struct {
	requestStartedCount       int
	requestFinishedCount      int
	clientFailedRequestCount  int
	serverFailedRequestCount  int
	perHTTPStatusRequestCount map[int32]int
}

type RequestCountsWithVerbs struct {
	requestCounts       *RequestCounts
	perVerbRequestCount map[string]*RequestCounts
}

type PerHTTPStatusRequestCount struct {
	httpStatus              int32
	requestCounts           RequestCounts
	perResourceRequestCount map[schema.GroupVersionResource]*RequestCounts
	perUserRequestCount     map[string]*RequestCounts
}

type PerUserRequestCount struct {
	user                    string
	requestCounts           RequestCounts
	perResourceRequestCount map[schema.GroupVersionResource]*RequestCountsWithVerbs
	perVerbRequestCount     map[string]*RequestCounts
}

type PerResourceRequestCount struct {
	groupVersionResource schema.GroupVersionResource
	requestCounts        RequestCounts
	perUserRequestCount  map[string]*RequestCounts
	perVerbRequestCount  map[string]*RequestCounts
}

type auditEventInfo struct {
	auditID              types.UID
	groupVersionResource *schema.GroupVersionResource
}

func (i *auditEventInfo) getGroupVersionResource(auditEvent *auditv1.Event) schema.GroupVersionResource {
	if len(i.auditID) > 0 && i.auditID != auditEvent.AuditID {
		panic(fmt.Sprintf("mismatched auditID: have %v, need %v", i.auditID, auditEvent.AuditID))
	}
	i.auditID = auditEvent.AuditID

	if i.groupVersionResource != nil {
		return *i.groupVersionResource
	}

	_, gvr, _, _ := URIToParts(auditEvent.RequestURI)
	i.groupVersionResource = &gvr
	return *i.groupVersionResource
}

func (s *AuditLogSummary) Add(auditEvent *auditv1.Event) {
	if auditEvent == nil {
		s.lineReadFailureCount++
		return
	}

	s.requestCounts.Add(auditEvent)
	info := auditEventInfo{}
	gvr := info.getGroupVersionResource(auditEvent)
	if _, ok := s.perResourceRequestCount[gvr]; !ok {
		s.perResourceRequestCount[gvr] = NewPerResourceRequestCount(gvr)
	}
	s.perResourceRequestCount[gvr].Add(auditEvent, info)

	if _, ok := s.perUserRequestCount[auditEvent.User.Username]; !ok {
		s.perUserRequestCount[auditEvent.User.Username] = NewPerUserRequestCount(auditEvent.User.Username)
	}
	s.perUserRequestCount[auditEvent.User.Username].Add(auditEvent, info)

	if auditEvent.ResponseStatus != nil {
		httpStatus := auditEvent.ResponseStatus.Code
		if _, ok := s.perHTTPStatusRequestCount[httpStatus]; !ok {
			s.perHTTPStatusRequestCount[httpStatus] = NewPerStatusRequestCount(httpStatus)
		}
		s.perHTTPStatusRequestCount[httpStatus].Add(auditEvent, info)
	}
}

func (s *RequestCounts) Add(auditEvent *auditv1.Event) {
	switch auditEvent.Stage {
	case auditv1.StageRequestReceived:
		s.requestStartedCount++
	case auditv1.StageResponseComplete:
		s.requestFinishedCount++
	}

	if auditEvent.ResponseStatus != nil {
		httpStatus := auditEvent.ResponseStatus.Code
		s.perHTTPStatusRequestCount[httpStatus] = s.perHTTPStatusRequestCount[httpStatus] + 1
		switch {
		case httpStatus >= 400 && httpStatus < 500:
			s.clientFailedRequestCount++
		case httpStatus >= 500 && httpStatus < 600:
			s.serverFailedRequestCount++
		default:
			// nothing special
		}
	}
}

func (s *RequestCountsWithVerbs) Add(auditEvent *auditv1.Event) {
	s.requestCounts.Add(auditEvent)
	AddPerVerbRequestCount(s.perVerbRequestCount, auditEvent)
}

func (s *PerHTTPStatusRequestCount) Add(auditEvent *auditv1.Event, auditEventInfo auditEventInfo) {
	if auditEvent.ResponseStatus == nil {
		return
	}
	if auditEvent.ResponseStatus.Code != s.httpStatus {
		return
	}
	s.requestCounts.Add(auditEvent)

	gvr := auditEventInfo.getGroupVersionResource(auditEvent)
	if _, ok := s.perResourceRequestCount[gvr]; !ok {
		s.perResourceRequestCount[gvr] = NewRequestCounts()
	}
	s.perResourceRequestCount[gvr].Add(auditEvent)

	if _, ok := s.perUserRequestCount[auditEvent.User.Username]; !ok {
		s.perUserRequestCount[auditEvent.User.Username] = NewRequestCounts()
	}
	s.perUserRequestCount[auditEvent.User.Username].Add(auditEvent)
}

func (s *PerUserRequestCount) Add(auditEvent *auditv1.Event, auditEventInfo auditEventInfo) {
	if auditEvent.User.Username != s.user {
		return
	}
	s.requestCounts.Add(auditEvent)

	gvr := auditEventInfo.getGroupVersionResource(auditEvent)
	if _, ok := s.perResourceRequestCount[gvr]; !ok {
		s.perResourceRequestCount[gvr] = NewRequestCountsWithVerbs()
	}
	s.perResourceRequestCount[gvr].Add(auditEvent)

	AddPerVerbRequestCount(s.perVerbRequestCount, auditEvent)
}

func AddPerVerbRequestCount(perVerbRequestCount map[string]*RequestCounts, auditEvent *auditv1.Event) {
	if _, ok := perVerbRequestCount[auditEvent.Verb]; !ok {
		perVerbRequestCount[auditEvent.Verb] = NewRequestCounts()
	}
	perVerbRequestCount[auditEvent.Verb].Add(auditEvent)
}

func (s *PerResourceRequestCount) Add(auditEvent *auditv1.Event, auditEventInfo auditEventInfo) {
	gvr := auditEventInfo.getGroupVersionResource(auditEvent)
	if gvr != s.groupVersionResource {
		return
	}
	s.requestCounts.Add(auditEvent)

	if _, ok := s.perUserRequestCount[auditEvent.User.Username]; !ok {
		s.perUserRequestCount[auditEvent.User.Username] = NewRequestCounts()
	}
	s.perUserRequestCount[auditEvent.User.Username].Add(auditEvent)

	AddPerVerbRequestCount(s.perVerbRequestCount, auditEvent)
}

func (s *AuditLogSummary) AddSummary(rhs *AuditLogSummary) {
	s.lineReadFailureCount += rhs.lineReadFailureCount
	s.requestCounts.AddSummary(&rhs.requestCounts)

	for k, v := range rhs.perUserRequestCount {
		if _, ok := s.perUserRequestCount[k]; !ok {
			s.perUserRequestCount[k] = NewPerUserRequestCount(k)
		}
		s.perUserRequestCount[k].AddSummary(v)
	}
	for k, v := range rhs.perResourceRequestCount {
		if _, ok := s.perResourceRequestCount[k]; !ok {
			s.perResourceRequestCount[k] = NewPerResourceRequestCount(k)
		}
		s.perResourceRequestCount[k].AddSummary(v)
	}
	for k, v := range rhs.perHTTPStatusRequestCount {
		if _, ok := s.perHTTPStatusRequestCount[k]; !ok {
			s.perHTTPStatusRequestCount[k] = NewPerStatusRequestCount(k)
		}
		s.perHTTPStatusRequestCount[k].AddSummary(v)
	}
}

func (s *RequestCounts) AddSummary(rhs *RequestCounts) {
	s.requestStartedCount += rhs.requestStartedCount
	s.requestFinishedCount += rhs.requestFinishedCount
	s.clientFailedRequestCount += rhs.clientFailedRequestCount
	s.serverFailedRequestCount += rhs.serverFailedRequestCount
	for k, v := range rhs.perHTTPStatusRequestCount {
		s.perHTTPStatusRequestCount[k] = s.perHTTPStatusRequestCount[k] + v
	}
}

func (s *RequestCountsWithVerbs) AddSummary(rhs *RequestCountsWithVerbs) {
	s.requestCounts.AddSummary(rhs.requestCounts)
	AddSummaryPerVerbRequestCount(s.perVerbRequestCount, rhs.perVerbRequestCount)
}

func (s *PerHTTPStatusRequestCount) AddSummary(rhs *PerHTTPStatusRequestCount) {
	if s.httpStatus != rhs.httpStatus {
		panic(fmt.Sprintf("mismatching key: have %v, need %v", s.httpStatus, rhs.httpStatus))
	}
	s.requestCounts.AddSummary(&rhs.requestCounts)
	for k, v := range rhs.perResourceRequestCount {
		if _, ok := s.perResourceRequestCount[k]; !ok {
			s.perResourceRequestCount[k] = NewRequestCounts()
		}
		s.perResourceRequestCount[k].AddSummary(v)
	}
	AddSummaryPerVerbRequestCount(s.perUserRequestCount, rhs.perUserRequestCount)
}

func AddSummaryPerVerbRequestCount(lhs, rhs map[string]*RequestCounts) {
	for k, v := range rhs {
		if _, ok := lhs[k]; !ok {
			lhs[k] = NewRequestCounts()
		}
		lhs[k].AddSummary(v)
	}
}

func (s *PerUserRequestCount) AddSummary(rhs *PerUserRequestCount) {
	if s.user != rhs.user {
		panic(fmt.Sprintf("mismatching key: have %v, need %v", s.user, rhs.user))
	}
	s.requestCounts.AddSummary(&rhs.requestCounts)
	for k, v := range rhs.perResourceRequestCount {
		if _, ok := s.perResourceRequestCount[k]; !ok {
			s.perResourceRequestCount[k] = NewRequestCountsWithVerbs()
		}
		s.perResourceRequestCount[k].AddSummary(v)
	}
	for k, v := range rhs.perVerbRequestCount {
		if _, ok := s.perVerbRequestCount[k]; !ok {
			s.perVerbRequestCount[k] = NewRequestCounts()
		}
		s.perVerbRequestCount[k].AddSummary(v)
	}
}

func (s *PerResourceRequestCount) AddSummary(rhs *PerResourceRequestCount) {
	if s.groupVersionResource != rhs.groupVersionResource {
		panic(fmt.Sprintf("mismatching key: have %v, need %v", s.groupVersionResource, rhs.groupVersionResource))
	}
	s.requestCounts.AddSummary(&rhs.requestCounts)
	for k, v := range rhs.perUserRequestCount {
		if _, ok := s.perUserRequestCount[k]; !ok {
			s.perUserRequestCount[k] = NewRequestCounts()
		}
		s.perUserRequestCount[k].AddSummary(v)
	}
	for k, v := range rhs.perVerbRequestCount {
		if _, ok := s.perVerbRequestCount[k]; !ok {
			s.perVerbRequestCount[k] = NewRequestCounts()
		}
		s.perVerbRequestCount[k].AddSummary(v)
	}
}

func NewAuditLogSummary() *AuditLogSummary {
	return &AuditLogSummary{
		lineReadFailureCount:      0,
		requestCounts:             *NewRequestCounts(),
		perUserRequestCount:       map[string]*PerUserRequestCount{},
		perResourceRequestCount:   map[schema.GroupVersionResource]*PerResourceRequestCount{},
		perHTTPStatusRequestCount: map[int32]*PerHTTPStatusRequestCount{},
	}
}
func NewRequestCounts() *RequestCounts {
	return &RequestCounts{
		perHTTPStatusRequestCount: map[int32]int{},
	}
}
func NewRequestCountsWithVerbs() *RequestCountsWithVerbs {
	return &RequestCountsWithVerbs{
		requestCounts:       NewRequestCounts(),
		perVerbRequestCount: map[string]*RequestCounts{},
	}
}
func NewPerStatusRequestCount(httpStatus int32) *PerHTTPStatusRequestCount {
	return &PerHTTPStatusRequestCount{
		httpStatus:              httpStatus,
		requestCounts:           *NewRequestCounts(),
		perResourceRequestCount: map[schema.GroupVersionResource]*RequestCounts{},
		perUserRequestCount:     map[string]*RequestCounts{},
	}
}
func NewPerUserRequestCount(user string) *PerUserRequestCount {
	return &PerUserRequestCount{
		user:                    user,
		requestCounts:           *NewRequestCounts(),
		perResourceRequestCount: map[schema.GroupVersionResource]*RequestCountsWithVerbs{},
		perVerbRequestCount:     map[string]*RequestCounts{},
	}
}
func NewPerResourceRequestCount(gvr schema.GroupVersionResource) *PerResourceRequestCount {
	return &PerResourceRequestCount{
		groupVersionResource: gvr,
		requestCounts:        *NewRequestCounts(),
		perUserRequestCount:  map[string]*RequestCounts{},
		perVerbRequestCount:  map[string]*RequestCounts{},
	}
}

func URIToParts(uri string) (string, schema.GroupVersionResource, string, string) {
	ns := ""
	gvr := schema.GroupVersionResource{}
	name := ""

	if len(uri) >= 1 {
		if uri[0] == '/' {
			uri = uri[1:]
		}
	}

	// some request URL has query parameters like: /apis/image.openshift.io/v1/images?limit=500&resourceVersion=0
	// we are not interested in the query parameters.
	uri = strings.Split(uri, "?")[0]
	parts := strings.Split(uri, "/")
	if len(parts) == 0 {
		return ns, gvr, name, ""
	}
	// /api/v1/namespaces/<name>
	if parts[0] == "api" {
		if len(parts) >= 2 {
			gvr.Version = parts[1]
		}
		if len(parts) < 3 {
			return ns, gvr, name, ""
		}

		switch {
		case parts[2] != "namespaces": // cluster scoped request that is not a namespace
			gvr.Resource = parts[2]
			if len(parts) >= 4 {
				name = parts[3]
				return ns, gvr, name, ""
			}
		case len(parts) == 3 && parts[2] == "namespaces": // a namespace request /api/v1/namespaces
			gvr.Resource = parts[2]
			return "", gvr, "", ""

		case len(parts) == 4 && parts[2] == "namespaces": // a namespace request /api/v1/namespaces/<name>
			gvr.Resource = parts[2]
			name = parts[3]
			ns = parts[3]
			return ns, gvr, name, ""

		case len(parts) == 5 && parts[2] == "namespaces" && parts[4] == "finalize", // a namespace request /api/v1/namespaces/<name>/finalize
			len(parts) == 5 && parts[2] == "namespaces" && parts[4] == "status": // a namespace request /api/v1/namespaces/<name>/status
			gvr.Resource = parts[2]
			name = parts[3]
			ns = parts[3]
			return ns, gvr, name, parts[4]

		default:
			// this is not a cluster scoped request and not a namespace request we recognize
		}

		if len(parts) < 4 {
			return ns, gvr, name, ""
		}

		ns = parts[3]
		if len(parts) >= 5 {
			gvr.Resource = parts[4]
		}
		if len(parts) >= 6 {
			name = parts[5]
		}
		if len(parts) >= 7 {
			return ns, gvr, name, strings.Join(parts[6:], "/")
		}
		return ns, gvr, name, ""
	}

	if parts[0] != "apis" {
		return ns, gvr, name, ""
	}

	// /apis/group/v1/namespaces/<name>
	if len(parts) >= 2 {
		gvr.Group = parts[1]
	}
	if len(parts) >= 3 {
		gvr.Version = parts[2]
	}
	if len(parts) < 4 {
		return ns, gvr, name, ""
	}

	if parts[3] != "namespaces" {
		gvr.Resource = parts[3]
		if len(parts) >= 5 {
			name = parts[4]
			return ns, gvr, name, ""
		}
	}
	if len(parts) < 5 {
		return ns, gvr, name, ""
	}

	ns = parts[4]
	if len(parts) >= 6 {
		gvr.Resource = parts[5]
	}
	if len(parts) >= 7 {
		name = parts[6]
	}
	if len(parts) >= 8 {
		return ns, gvr, name, strings.Join(parts[7:], "/")
	}
	return ns, gvr, name, ""
}

func isMonitoredUser(user string) bool {
	for _, userPrefix := range filteredUsers {
		if strings.HasPrefix(user, userPrefix) {
			return false
		}
	}

	for _, userPrefix := range monitoredUsers {
		if strings.HasPrefix(user, userPrefix) {
			return true
		}
	}
	return false
}

func cleanupUser(user string) (string, string) {
	if strings.HasPrefix(user, systemNode) {
		if mappedUser, ok := mappedUsers[user]; ok {
			return mappedUser, user
		}
		mappedUser := fmt.Sprintf("%s-%d", systemNode, len(mappedUsers))
		mappedUsers[user] = mappedUser
		return mappedUser, user
	}
	return user, ""
}

func writeAuditLogDL(artifactDir, timeSuffix string, auditLogSummary *AuditLogSummary) {
	rows := make([]map[string]string, 0)
	for _, uv := range auditLogSummary.perUserRequestCount {
		if !isMonitoredUser(uv.user) {
			continue
		}
		for rk, rv := range uv.perResourceRequestCount {
			for vk, vv := range rv.perVerbRequestCount {
				for sk, sv := range vv.perHTTPStatusRequestCount {
					user, unmodifiedUser := cleanupUser(uv.user)
					data := map[string]string{"User": user, "Resource": rk.Resource, "Verb": vk, "HttpStatus": strconv.FormatInt(int64(sk), 10), "RequestCount": strconv.FormatInt(int64(sv), 10)}
					if len(unmodifiedUser) > 0 {
						data["UserUnmodified"] = unmodifiedUser
					}
					rows = append(rows, data)
				}

			}
		}
	}

	dataFile := dataloader.DataFile{
		TableName: "audit_resource_requests_per_user",
		Schema:    map[string]dataloader.DataType{"User": dataloader.DataTypeString, "Resource": dataloader.DataTypeString, "Verb": dataloader.DataTypeString, "HttpStatus": dataloader.DataTypeInteger, "RequestCount": dataloader.DataTypeInteger},
		Rows:      rows,
	}
	fileName := filepath.Join(artifactDir, fmt.Sprintf("audit-resource-requests-per-user%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
	}
}

func WriteAuditLogSummary(artifactDir, timeSuffix string, auditLogSummary *AuditLogSummary) error {
	serializable := NewSerializedAuditLogSummary(*auditLogSummary)
	writeSummary(artifactDir, fmt.Sprintf("audit-log-summary_%s.json", timeSuffix), serializable)

	justUsers := NewSerializedAuditLogSummary(*auditLogSummary)
	justUsers.RequestCounts.PerHTTPStatusRequestCount = nil
	justUsers.PerHTTPStatusRequestCount = nil
	justUsers.PerResourceRequestCount = nil
	for i := range justUsers.PerUserRequestCount {
		justUsers.PerUserRequestCount[i].RequestCounts.PerHTTPStatusRequestCount = nil
		justUsers.PerUserRequestCount[i].PerVerbRequestCount = nil
		justUsers.PerUserRequestCount[i].PerResourceRequestCount = nil
	}
	writeSummary(artifactDir, fmt.Sprintf("just-users-audit-log-summary_%s.json", timeSuffix), justUsers)

	justResources := NewSerializedAuditLogSummary(*auditLogSummary)
	justResources.RequestCounts.PerHTTPStatusRequestCount = nil
	justResources.PerHTTPStatusRequestCount = nil
	justResources.PerUserRequestCount = nil
	for i := range justResources.PerResourceRequestCount {
		justResources.PerResourceRequestCount[i].RequestCounts.PerHTTPStatusRequestCount = nil
		justResources.PerResourceRequestCount[i].PerVerbRequestCount = nil
		justResources.PerResourceRequestCount[i].PerUserRequestCount = nil
	}
	writeSummary(artifactDir, fmt.Sprintf("just-resources-audit-log-summary_%s.json", timeSuffix), justResources)

	writeAuditLogDL(artifactDir, timeSuffix, auditLogSummary)

	return nil
}

func writeSummary(artifactDir, filename string, serializable SerializedAuditLogSummary) error {
	summaryBytes, err := json.MarshalIndent(serializable, "", "    ")
	if err != nil {
		return err
	}
	summaryPath := filepath.Join(artifactDir, filename)
	if err := os.WriteFile(summaryPath, summaryBytes, 0644); err != nil {
		return fmt.Errorf("failed to write %v: %w", summaryPath, err)
	}
	return nil
}
