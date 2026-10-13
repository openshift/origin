// Package two_node contains end-to-end tests for two-node (DualReplica) OpenShift clusters.
//
// Tests in this package verify etcd recovery, node replacement, and cluster behavior
// when running with only two control-plane nodes plus an arbiter for quorum.
//
// Precondition Validation:
// Tests use the preconditions package (pkg/test/preconditions) to check cluster health
// before running. When precondition checking is invoked, a synthetic JUnit test called
// "[openshift-tests] cluster precondition validation" is generated:
//   - PASSES if all tests ran successfully (no precondition failures)
//   - FAILS if any tests were skipped due to unmet cluster preconditions
//
// This provides the Technical Release Team (TRT) with a consistent test name that has
// meaningful pass/fail rates. See pkg/test/preconditions/preconditions.go for details.
package two_node
