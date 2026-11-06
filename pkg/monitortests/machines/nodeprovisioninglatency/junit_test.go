package nodeprovisioninglatency

import (
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

func TestAllowedWithGrace(t *testing.T) {
	jobType := func(topology string) *platformidentification.JobType {
		return &platformidentification.JobType{Platform: "aws", Topology: topology}
	}

	tests := []struct {
		name     string
		p99      time.Duration
		topology string
		want     time.Duration
	}{
		{name: "percentage wins on large ha values", p99: 100 * time.Second, topology: "ha", want: 120 * time.Second},
		{name: "fixed grace wins on small ha values", p99: 1 * time.Second, topology: "ha", want: 6 * time.Second},
		{name: "single node uses larger grace", p99: 100 * time.Second, topology: "single", want: 140 * time.Second},
		{name: "dual replica uses larger fixed grace", p99: 100 * time.Second, topology: "dual", want: 160 * time.Second},
		{name: "zero p99 is floored to one second", p99: 0, topology: "ha", want: 6 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, details := allowedWithGrace(test.p99, jobType(test.topology))
			if got != test.want {
				t.Errorf("allowedWithGrace(%v) = %v, want %v", test.p99, got, test.want)
			}
			if details == "" {
				t.Errorf("expected human-readable details")
			}
		})
	}
}

func TestEvaluateSegment(t *testing.T) {
	jobType := platformidentification.JobType{Platform: "aws", Topology: "ha"}
	allowed := 60 * time.Second

	tests := []struct {
		name        string
		rows        []provisioningRow
		allowed     *time.Duration
		jobType     platformidentification.JobType
		wantSkip    bool
		wantFailure bool
	}{
		{
			name:     "no matching samples skips",
			rows:     []provisioningRow{{MachineRole: roleMaster, CreatedDuringRun: true, MachineToInstance: durationPtr(time.Second)}},
			allowed:  &allowed,
			jobType:  jobType,
			wantSkip: true,
		},
		{
			name:     "missing historical data skips",
			rows:     []provisioningRow{{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(time.Second)}},
			allowed:  nil,
			jobType:  jobType,
			wantSkip: true,
		},
		{
			name:     "unknown platform skips",
			rows:     []provisioningRow{{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(time.Second)}},
			allowed:  &allowed,
			jobType:  platformidentification.JobType{},
			wantSkip: true,
		},
		{
			name: "median under the allowance passes",
			rows: []provisioningRow{
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(10 * time.Second)},
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(20 * time.Second)},
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(30 * time.Second)},
			},
			allowed: &allowed,
			jobType: jobType,
		},
		{
			name: "median over the allowance fails",
			rows: []provisioningRow{
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(10 * time.Second)},
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(5 * time.Minute)},
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(6 * time.Minute)},
			},
			allowed:     &allowed,
			jobType:     jobType,
			wantFailure: true,
		},
		{
			name: "rows from other cohorts are ignored",
			rows: []provisioningRow{
				{MachineRole: roleWorker, CreatedDuringRun: false, MachineToInstance: durationPtr(10 * time.Minute)},
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(10 * time.Second)},
			},
			allowed: &allowed,
			jobType: jobType,
		},
		{
			name: "nil durations are ignored",
			rows: []provisioningRow{
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: nil},
				{MachineRole: roleWorker, CreatedDuringRun: true, MachineToInstance: durationPtr(10 * time.Second)},
			},
			allowed: &allowed,
			jobType: jobType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := allowedWithGraceForLookup
			defer func() { allowedWithGraceForLookup = original }()
			allowedWithGraceForLookup = func(_, _ string, _ bool, _ platformidentification.JobType) (*time.Duration, string) {
				return test.allowed, "test details"
			}

			m := &nodeProvisioningLatency{rows: test.rows}
			junit := m.evaluateSegment(test.jobType, segmentMachineToInstance,
				func(row provisioningRow) *time.Duration { return row.MachineToInstance }, roleWorker, true)

			if test.wantSkip {
				if junit.SkipMessage == nil {
					t.Fatalf("expected a skip, got %+v", junit)
				}
				return
			}
			if junit.SkipMessage != nil {
				t.Fatalf("unexpected skip: %s", junit.SkipMessage.Message)
			}
			if test.wantFailure && junit.FailureOutput == nil {
				t.Fatalf("expected a failure, got a pass")
			}
			if !test.wantFailure && junit.FailureOutput != nil {
				t.Fatalf("unexpected failure: %s", junit.FailureOutput.Output)
			}
		})
	}
}

func TestEvaluateSegmentSkipsInstallerCreatedMasters(t *testing.T) {
	m := &nodeProvisioningLatency{rows: []provisioningRow{{
		MachineRole:       roleMaster,
		CreatedDuringRun:  false,
		MachineToInstance: durationPtr(time.Minute),
	}}}

	junit := m.evaluateSegment(platformidentification.JobType{Platform: "aws"}, segmentMachineToInstance,
		func(row provisioningRow) *time.Duration { return row.MachineToInstance }, roleMaster, false)

	if junit.SkipMessage == nil {
		t.Fatalf("expected a skip, got %+v", junit)
	}
	if !strings.Contains(junit.SkipMessage.Message, "install time") {
		t.Errorf("unexpected skip message: %s", junit.SkipMessage.Message)
	}
}

func TestSkipAllKeepsTheJUnitNamesStable(t *testing.T) {
	m := &nodeProvisioningLatency{}
	junits := m.skipAll("test message")

	wantCount := len(evaluatedSegments) * len(evaluatedRoles) * 2
	if len(junits) != wantCount {
		t.Fatalf("skipAll() returned %d junits, want %d", len(junits), wantCount)
	}

	seen := map[string]bool{}
	for _, junit := range junits {
		if junit.SkipMessage == nil || junit.SkipMessage.Message != "test message" {
			t.Errorf("%s is not a skip with the expected message", junit.Name)
		}
		if seen[junit.Name] {
			t.Errorf("duplicate junit name %q", junit.Name)
		}
		seen[junit.Name] = true
	}
}

func TestProvisioningTestNameIsStable(t *testing.T) {
	want := "[sig-cluster-lifecycle] node-provisioning-latency: NodeToReadySeconds (worker, created-during-run=true)"
	if got := provisioningTestName(segmentNodeToReady, roleWorker, true); got != want {
		t.Errorf("provisioningTestName() = %q, want %q", got, want)
	}
}
