package nodedetails

import (
	"strings"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type LoadBalancerCheckSummary struct {
	perNodeLoadBalancerCheckSummary map[string]*PerNodeLoadBalancerCheckSummary
}

type PerNodeLoadBalancerCheckSummary struct {
	nodeName string

	// this uses the external load balancer. If these are present more than 10s after termination started
	// it means the LB is not set up well. If they are veryLate, then it's
	// very bad.
	duringTerminationDisruptionRequestsReceived []*auditv1.Event
	veryLateDisruptionRequestsReceived          []*auditv1.Event

	// nodes use the internal load balancer. If these are present more than 10s after termination started
	// it means the LB is not set up well. If they are veryLate, then it's
	// very bad.
	duringTerminationNodeRequestsReceived []*auditv1.Event
	veryLateNodeRequestsReceived          []*auditv1.Event

	// platform requests use the internal service load balancer. If these are present more than 10s after termination started
	// it means the service network is not set up well. If they are veryLate, then it's
	// very bad.
	// these will have to be checked periodically to be sure we are not accidentally
	// including internal LB connections as from the CVO and the network operator.
	duringTerminationPlatformRequestsReceived []*auditv1.Event
	veryLatePlatformRequestsReceived          []*auditv1.Event
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
	isDisruption := false

	switch {
	case isDisruption:
		s.duringTerminationDisruptionRequestsReceived = append(s.duringTerminationDisruptionRequestsReceived, auditEvent)
		if isVeryLate {
			s.veryLateDisruptionRequestsReceived = append(s.veryLateDisruptionRequestsReceived, auditEvent)
		}
	case isNode:
		s.duringTerminationNodeRequestsReceived = append(s.duringTerminationNodeRequestsReceived, auditEvent)
		if isVeryLate {
			s.veryLateNodeRequestsReceived = append(s.veryLateNodeRequestsReceived, auditEvent)
		}
	case isPlatform:
		s.duringTerminationPlatformRequestsReceived = append(s.duringTerminationPlatformRequestsReceived, auditEvent)
		if isVeryLate {
			s.veryLatePlatformRequestsReceived = append(s.veryLatePlatformRequestsReceived, auditEvent)
		}
	}
}
