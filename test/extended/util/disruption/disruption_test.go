package disruption

import (
	"context"
	"reflect"
	"testing"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

type recordingUpgradeTest struct {
	calls      []string
	setupPanic any
	testPanic  any
}

func (t *recordingUpgradeTest) Name() string {
	return "recording-upgrade-test"
}

func (t *recordingUpgradeTest) Setup(context.Context, *framework.Framework) {
	t.calls = append(t.calls, "setup")
	if t.setupPanic != nil {
		panic(t.setupPanic)
	}
}

func (t *recordingUpgradeTest) Test(context.Context, *framework.Framework, <-chan struct{}, upgrades.UpgradeType) {
	t.calls = append(t.calls, "test")
	if t.testPanic != nil {
		panic(t.testPanic)
	}
}

func (t *recordingUpgradeTest) Teardown(context.Context, *framework.Framework) {
	t.calls = append(t.calls, "teardown")
}

func TestRunUpgradeTest(t *testing.T) {
	testCases := []struct {
		name          string
		setupPanic    any
		testPanic     any
		expectedPanic any
		expectedCalls []string
	}{
		{
			name:          "successful test",
			expectedCalls: []string{"setup", "ready", "test", "teardown"},
		},
		{
			name:          "setup panic",
			setupPanic:    "setup failed",
			expectedPanic: "setup failed",
			expectedCalls: []string{"setup", "teardown"},
		},
		{
			name:          "test panic",
			testPanic:     "test failed",
			expectedPanic: "test failed",
			expectedCalls: []string{"setup", "ready", "test", "teardown"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			upgradeTest := &recordingUpgradeTest{
				setupPanic: testCase.setupPanic,
				testPanic:  testCase.testPanic,
			}

			var recovered any
			func() {
				defer func() {
					recovered = recover()
				}()
				runUpgradeTest(
					context.Background(),
					upgradeTest,
					nil,
					func() {
						upgradeTest.calls = append(upgradeTest.calls, "ready")
					},
					nil,
					upgrades.ClusterUpgrade,
				)
			}()

			if recovered != testCase.expectedPanic {
				t.Errorf("expected panic %v, got %v", testCase.expectedPanic, recovered)
			}
			if !reflect.DeepEqual(upgradeTest.calls, testCase.expectedCalls) {
				t.Errorf("expected calls %v, got %v", testCase.expectedCalls, upgradeTest.calls)
			}
		})
	}
}
