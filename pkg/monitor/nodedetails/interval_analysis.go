package nodedetails

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/apimachinery/pkg/types"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/kubernetes"
)

func IntervalsFromAuditLogs(ctx context.Context, kubeClient kubernetes.Interface, beginning, end time.Time) (*AuditLogSummary, monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}

	// TODO honor begin and end times.  maybe
	auditLogSummary, lbCheckSummary, err := GetKubeAuditLogSummary(ctx, kubeClient)
	if err != nil {
		// TODO report the error AND the best possible summary we have
		return auditLogSummary, nil, err
	}

	auditEventIDsAlreadyNoted := sets.New[types.UID]()
	for nodeName, summary := range lbCheckSummary.PerNodeLoadBalancerCheckSummary {
		for _, auditEvent := range summary.VeryLateDisruptionRequestsReceived {
			if auditEventIDsAlreadyNoted.Has(auditEvent.AuditID) {
				continue
			}
			if !isAuditEventInTimeWindow(auditEvent, beginning, end) {
				continue
			}
			auditEventIDsAlreadyNoted.Insert(auditEvent.AuditID)
			disruptionBackendName, connectionType := backendAndConnectionTypeFromAudit(auditEvent)
			ret = append(ret, monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.VeryLateDisruptionCheckForNode(nodeName, disruptionBackendName, connectionType),
					Message: "reason/VeryLate",
				},
				From: auditEvent.RequestReceivedTimestamp.Time,
				To:   auditEvent.RequestReceivedTimestamp.Time.Add(1 * time.Second),
			})
			fmt.Printf("#### 1a\n")
		}

		for _, auditEvent := range summary.DuringTerminationDisruptionRequestsReceived {
			if auditEventIDsAlreadyNoted.Has(auditEvent.AuditID) {
				continue
			}
			if !isAuditEventInTimeWindow(auditEvent, beginning, end) {
				continue
			}
			auditEventIDsAlreadyNoted.Insert(auditEvent.AuditID)
			disruptionBackendName, connectionType := backendAndConnectionTypeFromAudit(auditEvent)
			ret = append(ret, monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: monitorapi.DuringTerminationDisruptionCheckForNode(nodeName, disruptionBackendName, connectionType),
					Message: "reason/DuringTermination",
				},
				From: auditEvent.RequestReceivedTimestamp.Time,
				To:   auditEvent.RequestReceivedTimestamp.Time.Add(1 * time.Second),
			})
			fmt.Printf("#### 1b\n")
		}
	}

	for nodeName, summary := range lbCheckSummary.PerNodeLoadBalancerCheckSummary {
		for _, auditEvent := range summary.VeryLateRequestsReceived {
			if auditEventIDsAlreadyNoted.Has(auditEvent.AuditID) {
				continue
			}
			if !isAuditEventInTimeWindow(auditEvent, beginning, end) {
				continue
			}
			auditEventIDsAlreadyNoted.Insert(auditEvent.AuditID)
			ret = append(ret, monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.VeryLateRequestForNode(nodeName, auditEvent.User.Username),
					Message: "reason/VeryLate",
				},
				From: auditEvent.RequestReceivedTimestamp.Time,
				To:   auditEvent.RequestReceivedTimestamp.Time.Add(1 * time.Second),
			})
			fmt.Printf("#### 1c\n")
		}
	}

	return auditLogSummary, ret, nil
}

func backendAndConnectionTypeFromAudit(auditEvent *auditv1.Event) (string, monitorapi.BackendConnectionType) {
	if !strings.HasPrefix(auditEvent.UserAgent, "openshift-external-backend-sampler-") {
		return "", ""
	}

	combined := auditEvent.UserAgent[len("openshift-external-backend-sampler-"):]
	lastDashIndex := strings.Index(combined, "-")
	connectionType := combined[:lastDashIndex]
	disruptionBackendName := combined[lastDashIndex:]
	return disruptionBackendName, monitorapi.BackendConnectionType(connectionType)
}

func isAuditEventInTimeWindow(auditEvent *auditv1.Event, beginning, end time.Time) bool {
	if beginning.IsZero() || end.IsZero() {
		return true
	}
	if auditEvent.RequestReceivedTimestamp.Time.Before(beginning) {
		return false
	}
	if auditEvent.RequestReceivedTimestamp.Time.After(end) {
		return false
	}
	return true
}
