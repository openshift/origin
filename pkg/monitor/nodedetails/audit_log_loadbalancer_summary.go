package nodedetails

import (
	"strings"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type LoadBalancerCheckSummary struct {
	PerNodeLoadBalancerCheckSummary map[string]*PerNodeLoadBalancerCheckSummary
}

func NewLoadBalancerCheckSummary() *LoadBalancerCheckSummary {
	return &LoadBalancerCheckSummary{
		PerNodeLoadBalancerCheckSummary: map[string]*PerNodeLoadBalancerCheckSummary{},
	}
}

type PerNodeLoadBalancerCheckSummary struct {
	NodeName string

	VeryLateRequestsReceived []*auditv1.Event

	// this uses the external load balancer. If these are present more than 10s after termination started
	// it means the LB is not set up well. If they are veryLate, then it's
	// very bad.
	DuringTerminationDisruptionRequestsReceived []*auditv1.Event
	VeryLateDisruptionRequestsReceived          []*auditv1.Event

	// nodes use the internal load balancer. If these are present more than 10s after termination started
	// it means the LB is not set up well. If they are veryLate, then it's
	// very bad.
	DuringTerminationNodeRequestsReceived []*auditv1.Event
	VeryLateNodeRequestsReceived          []*auditv1.Event

	// platform requests use the internal service load balancer. If these are present more than 10s after termination started
	// it means the service network is not set up well. If they are veryLate, then it's
	// very bad.
	// these will have to be checked periodically to be sure we are not accidentally
	// including internal LB connections as from the CVO and the network operator.
	DuringTerminationPlatformRequestsReceived []*auditv1.Event
	VeryLatePlatformRequestsReceived          []*auditv1.Event
}

func (s *LoadBalancerCheckSummary) Add(nodeName string, auditEvent *auditv1.Event, auditEventInfo auditEventInfo) {
	if _, ok := s.PerNodeLoadBalancerCheckSummary[nodeName]; !ok {
		s.PerNodeLoadBalancerCheckSummary[nodeName] = &PerNodeLoadBalancerCheckSummary{
			NodeName: nodeName,
		}
	}
	s.PerNodeLoadBalancerCheckSummary[nodeName].Add(auditEvent, auditEventInfo)
}

func (s *LoadBalancerCheckSummary) AddSummary(rhs *LoadBalancerCheckSummary) {
	for nodeName := range rhs.PerNodeLoadBalancerCheckSummary {
		if _, ok := s.PerNodeLoadBalancerCheckSummary[nodeName]; !ok {
			s.PerNodeLoadBalancerCheckSummary[nodeName] = &PerNodeLoadBalancerCheckSummary{
				NodeName: nodeName,
			}
		}
		s.PerNodeLoadBalancerCheckSummary[nodeName].AddSummary(rhs.PerNodeLoadBalancerCheckSummary[nodeName])
	}

}

func (s *PerNodeLoadBalancerCheckSummary) Add(auditEvent *auditv1.Event, auditEventInfo auditEventInfo) {
	info, isDuringTermination := auditEvent.Annotations["openshift.io/during-termination"]
	if !isDuringTermination {
		return
	}
	// don't care about loopback
	if strings.Contains(info, "loopback=true") {
		return
	}
	veryLateInfo, isVeryLate := auditEvent.Annotations["openshift.io/during-graceful"]
	if strings.Contains(veryLateInfo, "loopback=true") {
		return
	}
	isNode := false
	for _, group := range auditEvent.User.Groups {
		if group == "system:nodes" {
			isNode = true
			break
		}
	}
	isPlatform := false
	for _, group := range auditEvent.User.Groups {
		if group == "system:serviceaccounts:openshift-network-operator" { // uses internal LB
			break
		}
		if group == "system:serviceaccounts:openshift-cluster-version" { // sometimes uses internal LB
			break
		}
		if strings.HasPrefix(group, "system:serviceaccounts:openshift-") { // the rest should use the service network
			isPlatform = true
			break
		}
	}
	isDisruption := isAPIServerDisruption(auditEvent)

	// every very late, not-loopback request is interesting.
	if isVeryLate {
		s.VeryLateRequestsReceived = append(s.VeryLateRequestsReceived, auditEvent)
	}

	switch {
	case isDisruption:
		s.DuringTerminationDisruptionRequestsReceived = append(s.DuringTerminationDisruptionRequestsReceived, auditEvent)
		if isVeryLate {
			s.VeryLateDisruptionRequestsReceived = append(s.VeryLateDisruptionRequestsReceived, auditEvent)
		}
	case isNode:
		s.DuringTerminationNodeRequestsReceived = append(s.DuringTerminationNodeRequestsReceived, auditEvent)
		if isVeryLate {
			s.VeryLateNodeRequestsReceived = append(s.VeryLateNodeRequestsReceived, auditEvent)
		}
	case isPlatform:
		s.DuringTerminationPlatformRequestsReceived = append(s.DuringTerminationPlatformRequestsReceived, auditEvent)
		if isVeryLate {
			s.VeryLatePlatformRequestsReceived = append(s.VeryLatePlatformRequestsReceived, auditEvent)
		}
	}
}

func isAPIServerDisruption(auditEvent *auditv1.Event) bool {
	switch {
	case strings.HasPrefix(auditEvent.UserAgent, "openshift-external-backend-sampler-"):
		return true
	default:
		return false
	}
}

func (s *PerNodeLoadBalancerCheckSummary) AddSummary(rhs *PerNodeLoadBalancerCheckSummary) {
	if rhs == nil {
		return
	}
	if s.NodeName != rhs.NodeName {
		return
	}

	s.VeryLateRequestsReceived = combineAuditEventNoDupes(s.VeryLateRequestsReceived, rhs.VeryLateRequestsReceived)
	s.DuringTerminationDisruptionRequestsReceived = combineAuditEventNoDupes(s.DuringTerminationDisruptionRequestsReceived, rhs.DuringTerminationDisruptionRequestsReceived)
	s.VeryLateDisruptionRequestsReceived = combineAuditEventNoDupes(s.VeryLateDisruptionRequestsReceived, rhs.VeryLateDisruptionRequestsReceived)
	s.DuringTerminationNodeRequestsReceived = combineAuditEventNoDupes(s.DuringTerminationNodeRequestsReceived, rhs.DuringTerminationNodeRequestsReceived)
	s.VeryLateNodeRequestsReceived = combineAuditEventNoDupes(s.VeryLateNodeRequestsReceived, rhs.VeryLateNodeRequestsReceived)
	s.DuringTerminationPlatformRequestsReceived = combineAuditEventNoDupes(s.DuringTerminationPlatformRequestsReceived, rhs.DuringTerminationPlatformRequestsReceived)
	s.VeryLatePlatformRequestsReceived = combineAuditEventNoDupes(s.VeryLatePlatformRequestsReceived, rhs.VeryLatePlatformRequestsReceived)
}

func combineAuditEventNoDupes(lhs, rhs []*auditv1.Event) []*auditv1.Event {
	ret := make([]*auditv1.Event, len(lhs))
	copy(ret, lhs)

	for i := range rhs {
		found := true
		for j := range lhs {
			if rhs[i].AuditID == lhs[j].AuditID {
				found = true
				break
			}
		}
		if found {
			continue
		}
		ret = append(ret, rhs[i])
	}

	return ret
}
