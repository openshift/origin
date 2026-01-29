// Package two_node contains end-to-end tests for two-node (DualReplica) OpenShift clusters.
//
// Tests in this package verify etcd recovery, node replacement, and cluster behavior
// when running with only two control-plane nodes plus an arbiter for quorum.
//
// Precondition Skip Detection:
// Tests that skip due to unmet cluster preconditions (using the marker string
// "Skipping test due to unmet cluster preconditions:") are automatically detected
// by openshift-tests and converted to synthetic failures in the JUnit results.
// This ensures CI systems properly report these as failures rather than silent skips.
// See pkg/test/ginkgo/cmd_runsuite.go detectPreconditionSkips() for implementation.
package two_node
