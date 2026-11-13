package nodeprovisioninglatency

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/allowednodeprovisioning"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

var evaluatedSegments = []struct {
	segment  string
	duration func(provisioningRow) *time.Duration
}{
	{segmentMachineToInstance, func(row provisioningRow) *time.Duration { return row.MachineToInstance }},
	{segmentInstanceToNode, func(row provisioningRow) *time.Duration { return row.InstanceToNode }},
	{segmentNodeToReady, func(row provisioningRow) *time.Duration { return row.NodeToReady }},
	{segmentMachineToReady, func(row provisioningRow) *time.Duration { return row.MachineToReady }},
}

var evaluatedRoles = []string{roleWorker, roleMaster}

func provisioningTestName(segment, role string, createdDuringRun bool) string {
	return fmt.Sprintf("[sig-cluster-lifecycle] node-provisioning-latency: %s (%s, created-during-run=%t)",
		segment, role, createdDuringRun)
}

func skipJunit(name, message string) *junitapi.JUnitTestCase {
	return &junitapi.JUnitTestCase{
		Name:        name,
		SkipMessage: &junitapi.SkipMessage{Message: message},
	}
}

// allowedWithGrace mirrors the disruption grace policy: a percentage for large values and a
// fixed floor for small ones, relaxed further on single-node and dual-replica topologies.
func allowedWithGrace(p99 time.Duration, jobType *platformidentification.JobType) (time.Duration, string) {
	details := []string{fmt.Sprintf("P99 from historical data for similar jobs over past 3 weeks: %s", p99)}
	if p99 < time.Second {
		p99 = time.Second
		details = append(details, "rounded P99 up to always allow one second")
	}

	graceSeconds, gracePercent := 5.0, 1.2
	percentDetails := "added an additional 20% of grace"
	switch jobType.Topology {
	case "single":
		graceSeconds, gracePercent = 10.0, 1.4
		percentDetails = "added an additional 40% of grace for single node cluster"
	case "dual":
		graceSeconds, gracePercent = 60.0, 1.4
		percentDetails = "added an additional 40% of grace for dual replica cluster"
	}

	p99Seconds := p99.Seconds()
	withSeconds := p99Seconds + graceSeconds
	withPercent := p99Seconds * gracePercent
	allowedSeconds := withSeconds
	details = append(details, fmt.Sprintf("added an additional %gs of grace", graceSeconds))
	if withPercent > withSeconds {
		allowedSeconds = withPercent
		details = append(details, percentDetails)
	}

	rounded := time.Duration(math.Round(allowedSeconds)) * time.Second
	return rounded, strings.Join(details, "\n")
}

func formatSamples(samples []time.Duration) string {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)

	formatted := make([]string, 0, len(sorted))
	for _, sample := range sorted {
		formatted = append(formatted, (time.Duration(math.Round(sample.Seconds())) * time.Second).String())
	}
	return strings.Join(formatted, ", ")
}

func (m *nodeProvisioningLatency) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if m.unavailable || len(m.rows) == 0 {
		return nil, nil
	}

	jobType, err := platformidentification.GetJobType(ctx, m.adminRESTConfig)
	if err != nil {
		m.logger.Warnf("unable to determine job type, skipping evaluation: %v", err)
		return m.skipAll("unable to determine job type: " + err.Error()), nil
	}

	// Groups are fixed so the set of JUnit names stays stable across runs. A group with no
	// samples is reported as a skip rather than omitted.
	junits := make([]*junitapi.JUnitTestCase, 0, len(evaluatedSegments)*len(evaluatedRoles)*2)
	for _, segment := range evaluatedSegments {
		for _, role := range evaluatedRoles {
			for _, createdDuringRun := range []bool{false, true} {
				junits = append(junits, m.evaluateSegment(*jobType, segment.segment, segment.duration, role, createdDuringRun))
			}
		}
	}

	if bool(m.flakeJunits) {
		junits = monitortestframework.JUnitsToFlakes(junits)
	}

	return junits, nil
}

func (m *nodeProvisioningLatency) skipAll(message string) []*junitapi.JUnitTestCase {
	junits := make([]*junitapi.JUnitTestCase, 0, len(evaluatedSegments)*len(evaluatedRoles)*2)
	for _, segment := range evaluatedSegments {
		for _, role := range evaluatedRoles {
			for _, createdDuringRun := range []bool{false, true} {
				junits = append(junits, skipJunit(provisioningTestName(segment.segment, role, createdDuringRun), message))
			}
		}
	}

	if bool(m.flakeJunits) {
		junits = monitortestframework.JUnitsToFlakes(junits)
	}
	return junits
}

func (m *nodeProvisioningLatency) evaluateSegment(jobType platformidentification.JobType, segment string, duration func(provisioningRow) *time.Duration, role string, createdDuringRun bool) *junitapi.JUnitTestCase {
	testName := provisioningTestName(segment, role, createdDuringRun)

	if role == roleMaster && !createdDuringRun {
		return skipJunit(testName, "masters created before the run are excluded: they measure install time, not Machine API provisioning")
	}

	samples := []time.Duration{}
	for _, row := range m.rows {
		if row.MachineRole != role || row.CreatedDuringRun != createdDuringRun {
			continue
		}
		if value := duration(row); value != nil {
			samples = append(samples, *value)
		}
	}
	if len(samples) == 0 {
		return skipJunit(testName, "no Machine samples matched this segment, role and run cohort")
	}

	if jobType.Platform == "" {
		return skipJunit(testName, "unknown platform, skipping provisioning latency testing")
	}

	allowed, details := allowedWithGraceForLookup(segment, role, createdDuringRun, jobType)
	if allowed == nil {
		return skipJunit(testName, "no historical data to calculate allowed latency")
	}

	observed := medianDuration(samples)
	roundedObserved := time.Duration(math.Round(observed.Seconds())) * time.Second
	if roundedObserved <= *allowed {
		return &junitapi.JUnitTestCase{Name: testName}
	}

	message := fmt.Sprintf(
		"median %s provisioning latency for %s (created-during-run=%t) was %s, allowed is %s:\n%s\nobserved machines: %s",
		segment, role, createdDuringRun, roundedObserved, *allowed, details, formatSamples(samples))

	return &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: message,
		},
		SystemOut: message,
	}
}

// allowedWithGraceForLookup is split out so tests can substitute the historical lookup.
var allowedWithGraceForLookup = func(segment, role string, createdDuringRun bool, jobType platformidentification.JobType) (*time.Duration, string) {
	p99, _, err := allowednodeprovisioning.GetAllowedProvisioningLatency(segment, role, createdDuringRun, jobType)
	if err != nil || p99 == nil {
		return nil, ""
	}
	allowed, details := allowedWithGrace(*p99, &jobType)
	return &allowed, details
}
