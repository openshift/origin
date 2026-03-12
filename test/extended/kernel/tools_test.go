package kernel

import (
	"encoding/json"
	"fmt"
	"testing"
)

// buildTestReport generates a JSON report string matching the oslat/cyclictest format.
// cpuMaxValues is a map of CPU ID to max latency in usec.
func buildTestReport(cpuMaxValues map[int]int) string {
	threads := make(map[string]struct {
		Cpu int `json:"cpu"`
		Max int `json:"max"`
	})
	for cpu, max := range cpuMaxValues {
		threads[fmt.Sprintf("%d", cpu)] = struct {
			Cpu int `json:"cpu"`
			Max int `json:"max"`
		}{Cpu: cpu, Max: max}
	}
	report := struct {
		Threads map[string]struct {
			Cpu int `json:"cpu"`
			Max int `json:"max"`
		} `json:"thread"`
	}{Threads: threads}

	data, _ := json.Marshal(report)
	return string(data)
}

func TestParseLatencyResults_AllPass(t *testing.T) {
	cpus := make(map[int]int, 80)
	for i := 0; i < 80; i++ {
		cpus[i] = 50 // All well under 200us soft threshold
	}
	report := buildTestReport(cpus)
	thresholds := rtThresholdConfig{SoftThreshold: 200, HardThreshold: 400}

	analysis, err := parseLatencyResults("oslat", report, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Result != rtResultPass {
		t.Errorf("expected PASS, got %s: %s", analysis.Result, analysis.FailureReason)
	}
	if analysis.CPUsOverSoft != 0 {
		t.Errorf("expected 0 CPUs over soft, got %d", analysis.CPUsOverSoft)
	}
	if analysis.CPUsOverHard != 0 {
		t.Errorf("expected 0 CPUs over hard, got %d", analysis.CPUsOverHard)
	}
}

func TestParseLatencyResults_WarnFewOverSoft(t *testing.T) {
	cpus := make(map[int]int, 100)
	for i := 0; i < 100; i++ {
		cpus[i] = 50
	}
	// 1 CPU over soft (1% < 5%), but under hard
	cpus[17] = 211

	report := buildTestReport(cpus)
	thresholds := rtThresholdConfig{SoftThreshold: 200, HardThreshold: 400}

	analysis, err := parseLatencyResults("oslat", report, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Result != rtResultWarn {
		t.Errorf("expected WARN, got %s: %s", analysis.Result, analysis.FailureReason)
	}
	if analysis.CPUsOverSoft != 1 {
		t.Errorf("expected 1 CPU over soft, got %d", analysis.CPUsOverSoft)
	}
	if analysis.CPUsOverHard != 0 {
		t.Errorf("expected 0 CPUs over hard, got %d", analysis.CPUsOverHard)
	}
	if analysis.MaxLatency != 211 {
		t.Errorf("expected max latency 211, got %d", analysis.MaxLatency)
	}
}

func TestParseLatencyResults_FailSystemic(t *testing.T) {
	cpus := make(map[int]int, 100)
	for i := 0; i < 100; i++ {
		cpus[i] = 50
	}
	// 10 CPUs over soft (10% > 5%), but under hard
	for i := 0; i < 10; i++ {
		cpus[i] = 300
	}

	report := buildTestReport(cpus)
	thresholds := rtThresholdConfig{SoftThreshold: 200, HardThreshold: 400}

	analysis, err := parseLatencyResults("cyclictest", report, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Result != rtResultFail {
		t.Errorf("expected FAIL, got %s: %s", analysis.Result, analysis.FailureReason)
	}
	if analysis.CPUsOverSoft != 10 {
		t.Errorf("expected 10 CPUs over soft, got %d", analysis.CPUsOverSoft)
	}
	if analysis.PercentOverSoft != 10.0 {
		t.Errorf("expected 10.0%% over soft, got %.1f%%", analysis.PercentOverSoft)
	}
}

func TestParseLatencyResults_FailHardThreshold(t *testing.T) {
	cpus := make(map[int]int, 80)
	for i := 0; i < 80; i++ {
		cpus[i] = 50
	}
	// 1 CPU exceeds hard threshold
	cpus[35] = 500

	report := buildTestReport(cpus)
	thresholds := rtThresholdConfig{SoftThreshold: 200, HardThreshold: 400}

	analysis, err := parseLatencyResults("oslat", report, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.Result != rtResultFail {
		t.Errorf("expected FAIL, got %s: %s", analysis.Result, analysis.FailureReason)
	}
	if analysis.CPUsOverHard != 1 {
		t.Errorf("expected 1 CPU over hard, got %d", analysis.CPUsOverHard)
	}
}

func TestParseLatencyResults_CorrectStatistics(t *testing.T) {
	// 10 CPUs with known values for deterministic statistics
	cpus := map[int]int{
		0: 10, 1: 20, 2: 30, 3: 40, 4: 50,
		5: 60, 6: 70, 7: 80, 8: 90, 9: 100,
	}
	report := buildTestReport(cpus)
	thresholds := rtThresholdConfig{SoftThreshold: 200, HardThreshold: 400}

	analysis, err := parseLatencyResults("oslat", report, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis.TotalCPUs != 10 {
		t.Errorf("expected 10 total CPUs, got %d", analysis.TotalCPUs)
	}
	if analysis.MaxLatency != 100 {
		t.Errorf("expected max latency 100, got %d", analysis.MaxLatency)
	}
	expectedAvg := 55.0 // (10+20+30+40+50+60+70+80+90+100)/10
	if analysis.AvgMaxLatency != expectedAvg {
		t.Errorf("expected avg max latency %.1f, got %.1f", expectedAvg, analysis.AvgMaxLatency)
	}
	// P99 of 10 values: ceil(0.99*10)-1 = 9, so sorted[9] = 100
	if analysis.P99MaxLatency != 100 {
		t.Errorf("expected P99 max latency 100, got %d", analysis.P99MaxLatency)
	}
	if analysis.Result != rtResultPass {
		t.Errorf("expected PASS, got %s", analysis.Result)
	}
}

func TestParseLatencyResults_EmptyThreads(t *testing.T) {
	report := `{"thread": {}}`
	thresholds := rtThresholdConfig{SoftThreshold: 200, HardThreshold: 400}

	_, err := parseLatencyResults("oslat", report, thresholds)
	if err == nil {
		t.Fatal("expected error for empty threads, got nil")
	}
}

func TestParseLatencyResults_InvalidJSON(t *testing.T) {
	report := `{invalid json`
	thresholds := rtThresholdConfig{SoftThreshold: 200, HardThreshold: 400}

	_, err := parseLatencyResults("oslat", report, thresholds)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
