package defaultinvariants

import (
	"github.com/openshift/origin/pkg/invariants"
	watchpods "github.com/openshift/origin/pkg/invariants/watch_pods"
)

func NewDefaultInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	// TODO add invariantTests here
	invariantTests.AddInvariantOrDie("pod-lifecycle", "Test Framework", watchpods.NewPodWatcher())

	return invariantTests
}
