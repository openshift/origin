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

	// Enabled if false disables the effect of this plugin. A global override will
	// not be applied and projects will not be checked for an override annotation.
	Enabled bool

	// ActiveDeadlineSecondsOverride is the value to set on containers of run-once pods
	// Only a positive value is valid. Absence of a value means that the plugin
	// won't make any changes to the pod
	ActiveDeadlineSecondsOverride *int64
}

const ActiveDeadlineSecondsOverrideAnnotation = "openshift.io/active-deadline-seconds-override"
