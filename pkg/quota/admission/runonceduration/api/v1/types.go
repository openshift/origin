package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// RunOnceDurationConfig is the configuration for the RunOnceDuration plugin.
// It specifies a maximum value for ActiveDeadlineSeconds for a run-once pod.
// The project that contains the pod may specify a different setting. That setting will
// take precedence over the one configured for the plugin here.
type RunOnceDurationConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// ActiveDeadlineSecondsOverride is the maximum value to set on containers of run-once pods
	// Only a positive value is valid. Absence of a value means that the plugin
	// won't make any changes to the pod
	// TODO: change the external name of this field to reflect that it is a limit, not an override
	// It is kept this way for compatibility. Only change it in a new version of the API.
	ActiveDeadlineSecondsOverride *int64 `json:"activeDeadlineSecondsOverride,omitempty",description:"maximum value for activeDeadlineSeconds in run-once pods"`
}
