package util

import (
	"regexp"
	"strings"
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

func TestMaybeRenameTest(t *testing.T) {
	tests := []struct {
		name string

		testName             string
		excludedTestPatterns []string

		expectedText string
	}{
		{
			name:                 "simple serial match",
			testName:             "[Serial] test",
			excludedTestPatterns: []string{`\[Skipped:local\]`},
			expectedText:         "[Serial] test [Suite:openshift/conformance/serial]",
		},
		{
			name:                 "don't tag skipped",
			testName:             `[Serial] example test [Skipped:gce]`,
			excludedTestPatterns: []string{`\[Skipped:gce\]`},
			expectedText:         `[Serial] example test [Skipped:gce]`, // notice that this isn't categorized into any of our buckets
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testRenamer := &ginkgoTestRenamer{
				excludedTestsFilter: regexp.MustCompile(strings.Join(test.excludedTestPatterns, `|`)),
			}
			testNode := &testNode{
				text: test.testName,
			}

			testRenamer.maybeRenameTest(test.testName, testNode)

			if e, a := test.expectedText, testNode.Text(); e != a {
				t.Error(a)
			}

		})
	}
}

func TestStockRules(t *testing.T) {
	tests := []struct {
		name string

		testName string
		provider string

		expectedText string
	}{
		{
			name:         "should skip localssd on gce",
			provider:     "gce",
			testName:     `[sig-storage] In-tree Volumes [Driver: local][LocalVolumeType: gce-localssd-scsi-fs] [Serial] [Testpattern: Dynamic PV (default fs)] subPath should be able to unmount after the subpath directory is deleted`,
			expectedText: `[sig-storage] In-tree Volumes [Driver: local][LocalVolumeType: gce-localssd-scsi-fs] [Serial] [Testpattern: Dynamic PV (default fs)] subPath should be able to unmount after the subpath directory is deleted [Skipped:gce]`, // notice that this isn't categorized into any of our buckets
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testRenamer := newGinkgoTestRenamerFromGlobals(test.provider)
			testNode := &testNode{
				text: test.testName,
			}

			testRenamer.maybeRenameTest(test.testName, testNode)

			if e, a := test.expectedText, testNode.Text(); e != a {
				t.Error(a)
			}

		})
	}
}
