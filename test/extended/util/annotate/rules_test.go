package main

import (
	"testing"

	"github.com/onsi/ginkgo/types"
)

type testNode struct {
	text string
}

func (n *testNode) Type() types.SpecComponentType {
	return 0
}
func (n *testNode) CodeLocation() types.CodeLocation {
	return types.CodeLocation{}
}
func (n *testNode) Text() string {
	return n.text
}
func (n *testNode) SetText(text string) {
	n.text = text
}
func (n *testNode) Flag() types.FlagType {
	return 0
}
func (n *testNode) SetFlag(flag types.FlagType) {
}

func TestStockRules(t *testing.T) {
	tests := []struct {
		name string

		testName string

		expectedText string
	}{
		{
			name:         "simple serial match",
			testName:     "[Serial] test",
			expectedText: "[Serial] test [Suite:openshift/conformance/serial]",
		},
		{
			name:         "don't tag skipped",
			testName:     `[Serial] example test [Skipped:gce]`,
			expectedText: `[Serial] example test [Skipped:gce] [Suite:openshift/conformance/serial]`, // notice that this isn't categorized into any of our buckets
		},
		{
			name:         "not skipped",
			testName:     `[sig-network] Networking Granular Checks: Pods should function for intra-pod communication: http [LinuxOnly] [NodeConformance] [Conformance]`,
			expectedText: `[sig-network] Networking Granular Checks: Pods should function for intra-pod communication: http [LinuxOnly] [NodeConformance] [Conformance] [Suite:openshift/conformance/parallel/minimal]`,
		},
		{
			name:         "should skip localssd on gce",
			testName:     `[sig-storage] In-tree Volumes [Driver: local][LocalVolumeType: gce-localssd-scsi-fs] [Serial] [Testpattern: Dynamic PV (default fs)] subPath should be able to unmount after the subpath directory is deleted`,
			expectedText: `[sig-storage] In-tree Volumes [Driver: local][LocalVolumeType: gce-localssd-scsi-fs] [Serial] [Testpattern: Dynamic PV (default fs)] subPath should be able to unmount after the subpath directory is deleted [Skipped:gce] [Suite:openshift/conformance/serial]`, // notice that this isn't categorized into any of our buckets
		},
		{
			name:         "should skip NetworkPolicy tests on multitenant",
			testName:     `[Feature:NetworkPolicy] should do something with NetworkPolicy`,
			expectedText: `[Feature:NetworkPolicy] should do something with NetworkPolicy [Skipped:Network/OpenShiftSDN/Multitenant] [Suite:openshift/conformance/parallel]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testRenamer := newGenerator()
			testNode := &testNode{
				text: test.testName,
			}

			testRenamer.generateRename(test.testName, testNode)
			changed := testRenamer.output[test.testName]

			if e, a := test.expectedText, changed; e != a {
				t.Error(a)
			}
			testRenamer = newRenamerFromGenerated(map[string]string{test.testName: test.expectedText})
			testRenamer.updateNodeText(test.testName, testNode)

			if e, a := test.expectedText, testNode.Text(); e != a {
				t.Error(a)
			}

		})
	}
}
