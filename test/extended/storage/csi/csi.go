package csi

import (
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

// OpenShiftCSIDriverConfig holds definition test parameters of OpenShift specific CSI test
type OpenShiftCSIDriverConfig struct {
	// Name of the CSI driver.
	Driver string
	// Configuration of the LUN stress test. If nil, the test is skipped.
	LUNStressTest *LUNStressTestConfig
}

// Definition of the LUN stress test parameters.
// The test runs PodsTotal Pods with a single volume each, targeting the same node.
// All Pods are created at once. We expect that the CSI driver reports correct
// attach capacity and that the scheduler respects it*.
// Each pod does something very simple, like `ls /the/volume` and exits quickly.
// *) We know the scheduler does not respect the attach limit, see
// https://github.com/kubernetes/kubernetes/issues/126502
type LUNStressTestConfig struct {
	// How many Pods with one volume each to run in total. Set to 0 to disable the test.
	PodsTotal int
	// How long to wait for all Pods to start. 40 minutes by default.
	Timeout time.Duration
}

// runtime.DecodeInto needs a runtime.Object but doesn't do any
// deserialization of it and therefore none of the methods below need
// an implementation.
var _ runtime.Object = &OpenShiftCSIDriverConfig{}

const DefaultLUNStressTestTimeout = 40 * time.Minute

func (d *OpenShiftCSIDriverConfig) DeepCopyObject() runtime.Object {
	return nil
}

func (d *OpenShiftCSIDriverConfig) GetObjectKind() schema.ObjectKind {
	return nil
}

// Register all OCP specific CSI tests into upstream testsuites.CSISuites.
func AddOpenShiftCSITests(filename string) (string, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	// Select sane defaults
	cfg := &OpenShiftCSIDriverConfig{
		LUNStressTest: &LUNStressTestConfig{
			PodsTotal: 260,
			Timeout:   DefaultLUNStressTestTimeout,
		},
	}
	if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), bytes, cfg); err != nil {
		return "", fmt.Errorf("%s: %w", filename, err)
	}

	testsuites.CSISuites = append(testsuites.CSISuites, initSCSILUNOverflowCSISuite(cfg.LUNStressTest))
	return cfg.Driver, nil
}
