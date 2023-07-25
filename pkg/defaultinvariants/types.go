package defaultinvariants

import (
	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/invariants/timeline_serializer"
	watchpods "github.com/openshift/origin/pkg/invariants/watch_pods"
)

func NewDefaultInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	// TODO add invariantTests here
	invariantTests.AddInvariantOrDie("pod-lifecycle", "Test Framework", watchpods.NewPodWatcher())
	invariantTests.AddInvariantOrDie("timeline-serializer", "Test Framework", timeline_serializer.NewTimelineSerializer())

	return invariantTests
}
