package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// RunOnceDurationConfig is the configuration for the RunOnceDuration plugin.
// It specifies a default override value for ActiveDeadlineSeconds for a run-once pod.
// The project that contains the pod may specify a different setting. That setting will
// take precedence over the one configured for the plugin here.
type RunOnceDurationConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// ActiveDeadlineSecondsOverride is the value to set on containers of run-once pods
	// Only a positive value is valid. Absence of a value means that the plugin
	// won't make any changes to the pod
	ActiveDeadlineSecondsOverride *int64 `json:"activeDeadlineSecondsOverride,omitempty",description:"value to override activeDeadlineSeconds in run-once pods"`
}
