package legacynodemonitortests

import (
	"testing"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/stretchr/testify/assert"
)

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
