package monitorapi

import (
	"strings"

	configv1 "github.com/openshift/api/config/v1"
)

// condition/Degraded status/True reason/DNSDegraded changed: DNS default is degraded
func GetOperatorConditionStatus(message string) *configv1.ClusterOperatorStatusCondition {
	if !strings.HasPrefix(message, "condition/") {
		return nil
	}
	stanzas := strings.Split(message, " ")
	condition := &configv1.ClusterOperatorStatusCondition{}

	for _, stanza := range stanzas {
		keyValue := strings.SplitN(stanza, "/", 2)
		if len(keyValue) != 2 {
			continue
		}

		switch keyValue[0] {
		case "condition":
			if condition.Type == "" {
				condition.Type = configv1.ClusterStatusConditionType(keyValue[1])
			}
		case "status":
			if condition.Status == "" {
				condition.Status = configv1.ConditionStatus(keyValue[1])
			}
		case "reason":
			if condition.Reason == "" {
				condition.Reason = keyValue[1]
			}
		}
	}

	messages := strings.SplitN(message, ": ", 2)
	if len(messages) < 2 {
		return condition
	}
	condition.Message = messages[1]

	return condition
}
