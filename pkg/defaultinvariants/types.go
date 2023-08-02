package defaultinvariants

import "github.com/openshift/origin/pkg/invariants"

func NewDefaultInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	// TODO add invariantTests here

	return invariantTests
}
