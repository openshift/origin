package imageprogress

import (
	"encoding/json"
	"io"
	"reflect"
	"strconv"
	"testing"
)

func TestReports(t *testing.T) {
	tests := []struct {
		name        string
		gen         func(*progressGenerator)
		errExpected bool
		expected    report
	}{
		{
			name: "simple report",
			gen: func(p *progressGenerator) {
				p.status("1", "Extracting")
				p.status("2", "Downloading")
				p.status("1", "Downloading")
				p.status("2", "Pull complete")
			},
			expected: report{
				statusDownloading: &layerDetail{Count: 1},
				statusComplete:    &layerDetail{Count: 1},
			},
		},
		{
			name: "ignore invalid layer id",
			gen: func(p *progressGenerator) {
				p.status("1", "Downloading")
				p.status("hello", "testing")
				p.status("1", "Downloading")
			},
			expected: report{
				statusDownloading: &layerDetail{Count: 1},
			},
		},
		{
			name: "ignore retrying status",
			gen: func(p *progressGenerator) {
				p.status("1", "Downloading")
				p.status("2", "Pull complete")
				p.status("1", "Downloading")
				p.status("3", "Retrying")
			},
			expected: report{
				statusDownloading: &layerDetail{Count: 1},
				statusComplete:    &layerDetail{Count: 1},
			},
		},
		{
			name: "detect error",
			gen: func(p *progressGenerator) {
				p.status("1", "Downloading")
				p.err("an error")
			},
			errExpected: true,
		},
	}

	for _, test := range tests {
		pipeIn, pipeOut := io.Pipe()
		go func() {
			p := newProgressGenerator(pipeOut)
			test.gen(p)
			pipeOut.Close()
		}()
		var lastReport report
		w := newWriter(
			func(r report) {
				lastReport = r
			},
			func(a report, b report) bool {
				return true
			},
		)
		w.(*imageProgressWriter).stableThreshhold = 0
		_, err := io.Copy(w, pipeIn)
		if err != nil {
			if !test.errExpected {
				t.Errorf("%s: unexpected: %v", test.name, err)
			}
			continue
		}
		if test.errExpected {
			t.Errorf("%s: did not get expected error", test.name)
			continue
		}
		if !compareReport(lastReport, test.expected) {
			t.Errorf("%s: unexpected report, got: %v, expected: %v", test.name, lastReport, test.expected)
		}
	}
}

func TestErrorOnCopy(t *testing.T) {
	// Producer pipe
	genIn, genOut := io.Pipe()
	p := newProgressGenerator(genOut)

	// generate some data
	go func() {
		for i := 0; i < 100; i++ {
			p.status("1", "Downloading")
			p.status("2", "Downloading")
			p.status("3", "Downloading")
		}
		p.err("data error")
		genOut.Close()
	}()

	w := newWriter(func(r report) {}, func(a, b report) bool { return true })

	// Ensure that the error is propagated to the copy
	_, err := io.Copy(w, genIn)
	if err == nil {
		t.Errorf("Did not get an error when copying to writer")
	}
	if err.Error() != "data error" {
		t.Errorf("Did not get expected error: %v", err)
	}
}

func TestStableLayerCount(t *testing.T) {

	tests := []struct {
		name             string
		lastLayerCount   int
		layerStatusCount int
		stableThreshhold int
		callCount        int
		expectStable     bool
	}{
		{
			name:             "increasing layer count",
			lastLayerCount:   3,
			layerStatusCount: 4,
			callCount:        1,
			expectStable:     false,
		},
		{
			name:             "has not met stable threshhold",
			lastLayerCount:   3,
			layerStatusCount: 3,
			callCount:        2,
			stableThreshhold: 3,
			expectStable:     false,
		},
		{
			name:             "met stable threshhold",
			lastLayerCount:   3,
			layerStatusCount: 3,
			callCount:        4,
			stableThreshhold: 3,
			expectStable:     true,
		},
	}
	for _, test := range tests {
		w := newWriter(func(report) {}, func(a, b report) bool { return true }).(*imageProgressWriter)
		w.lastLayerCount = test.lastLayerCount
		w.layerStatus = map[string]progressLine{}
		w.stableThreshhold = test.stableThreshhold
		for i := 0; i < test.layerStatusCount; i++ {
			w.layerStatus[strconv.Itoa(i)] = progressLine{}
		}
		var result bool
		for i := 0; i < test.callCount; i++ {
			result = w.isStableLayerCount()
		}
		if result != test.expectStable {
			t.Errorf("%s: expected %v, got %v", test.name, test.expectStable, result)
		}
	}
}

func compareReport(a, b report) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
		if !reflect.DeepEqual(*a[k], *b[k]) {
			return false
		}
	}
	return true
}

type progressGenerator json.Encoder

func newProgressGenerator(w io.Writer) *progressGenerator {
	return (*progressGenerator)(json.NewEncoder(w))
}

func (p *progressGenerator) status(id, status string) {
	(*json.Encoder)(p).Encode(&progressLine{ID: id, Status: status})
}
func (p *progressGenerator) detail(id, status string, current, total int64) {
	(*json.Encoder)(p).Encode(&progressLine{ID: id, Status: status, Detail: &progressDetail{Current: current, Total: total}})
}
func (p *progressGenerator) err(msg string) {
	(*json.Encoder)(p).Encode(&progressLine{Error: msg})
}
