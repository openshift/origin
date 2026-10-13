package legacynodemonitortests

import (
	"testing"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDowngradeToFlakeOnReducedTopology(t *testing.T) {
	type resultCounts struct {
		failures int
		passes   int
	}

	failure := func(name string) *junitapi.JUnitTestCase {
		return &junitapi.JUnitTestCase{
			Name:          name,
			FailureOutput: &junitapi.FailureOutput{Output: "failure"},
		}
	}
	pass := func(name string) *junitapi.JUnitTestCase {
		return &junitapi.JUnitTestCase{Name: name}
	}

	tests := []struct {
		name        string
		junits      []*junitapi.JUnitTestCase
		flakedTests map[string]struct{}
		want        map[string]resultCounts
	}{
		{
			name:        "mapped failure becomes a flake",
			junits:      []*junitapi.JUnitTestCase{failure("mapped")},
			flakedTests: map[string]struct{}{"mapped": {}},
			want:        map[string]resultCounts{"mapped": {failures: 1, passes: 1}},
		},
		{
			name:        "existing flake is unchanged",
			junits:      []*junitapi.JUnitTestCase{failure("mapped"), pass("mapped")},
			flakedTests: map[string]struct{}{"mapped": {}},
			want:        map[string]resultCounts{"mapped": {failures: 1, passes: 1}},
		},
		{
			name:        "mapped pass is unchanged",
			junits:      []*junitapi.JUnitTestCase{pass("mapped")},
			flakedTests: map[string]struct{}{"mapped": {}},
			want:        map[string]resultCounts{"mapped": {passes: 1}},
		},
		{
			name:        "unmapped failure is unchanged",
			junits:      []*junitapi.JUnitTestCase{failure("unmapped")},
			flakedTests: map[string]struct{}{"mapped": {}},
			want:        map[string]resultCounts{"unmapped": {failures: 1}},
		},
		{
			name: "multiple mapped failures become flakes",
			junits: []*junitapi.JUnitTestCase{
				failure("first"),
				failure("second"),
			},
			flakedTests: map[string]struct{}{"first": {}, "second": {}},
			want: map[string]resultCounts{
				"first":  {failures: 1, passes: 1},
				"second": {failures: 1, passes: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := downgradeToFlakeOnReducedTopology(tt.junits, tt.flakedTests)
			counts := map[string]resultCounts{}
			for _, junit := range got {
				count := counts[junit.Name]
				if junit.FailureOutput == nil {
					count.passes++
				} else {
					count.failures++
				}
				counts[junit.Name] = count
			}

			require.Equal(t, tt.want, counts)
		})
	}
}

func TestUseCollectionTopology(t *testing.T) {
	tests := []struct {
		name               string
		collectionTopology string
		evaluationTopology string
	}{
		{
			name:               "collection topology overrides a later value",
			collectionTopology: "ha",
			evaluationTopology: "dual",
		},
		{
			name:               "failed collection lookup remains strict",
			collectionTopology: "",
			evaluationTopology: "dual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterData := platformidentification.ClusterData{
				JobType: platformidentification.JobType{Topology: tt.evaluationTopology},
			}

			useCollectionTopology(&clusterData, tt.collectionTopology)

			assert.Equal(t, tt.collectionTopology, clusterData.Topology)
		})
	}
}
