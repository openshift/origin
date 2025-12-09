package apiserverpprof

import (
	"testing"
)

func TestNewApiserverPprofCollector(t *testing.T) {
	collector := NewApiserverPprofCollector()
	if collector == nil {
		t.Fatal("NewApiserverPprofCollector returned nil")
	}
}
