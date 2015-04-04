package diagnostic

// This needed to be separate from other types to avoid import cycle
// diagnostic -> discovery -> types

import (
	"github.com/openshift/origin/pkg/diagnostics/discovery"
)

type DiagnosticCondition func(env *discovery.Environment) (skip bool, reason string)

type Diagnostic struct {
	Description string
	Condition   DiagnosticCondition
	Run         func(env *discovery.Environment)
}
