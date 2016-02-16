package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// RunOnceDurationConfig is the configuration for the RunOnceDuration plugin.
// It specifies a default override value for ActiveDeadlineSeconds for a run-once pod.
// The project that contains the pod may specify a different setting. That setting will
// take precedence over the one configured for the plugin here.
type RunOnceDurationConfig struct {
	unversioned.TypeMeta

	// ActiveDeadlineSecondsOverride is the value to set on containers of run-once pods
	// Only a positive value is valid. Absence of a value means that the plugin
	// won't make any changes to the pod
	ActiveDeadlineSecondsOverride *int64
}

// ActiveDeadlineSecondsOverrideAnnotation can be set on a project to override the number of
// seconds that a run-once pod can be active in that project
const ActiveDeadlineSecondsOverrideAnnotation = "openshift.io/active-deadline-seconds-override"
