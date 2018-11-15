package suite

import (
	"math/rand"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/internal/spec"
	"github.com/onsi/ginkgo/internal/spec_iterator"
)

func (suite *Suite) Iterator(config config.GinkgoConfigType) spec_iterator.SpecIterator {
	specsSlice := []*spec.Spec{}
	for _, collatedNodes := range suite.topLevelContainer.Collate() {
		specsSlice = append(specsSlice, spec.New(collatedNodes.Subject, collatedNodes.Containers, config.EmitSpecProgress))
	}

	specs := spec.NewSpecs(specsSlice)

	if config.RandomizeAllSpecs {
		specs.Shuffle(rand.New(rand.NewSource(config.RandomSeed)))
	}

	if config.SkipMeasurements {
		specs.SkipMeasurements()
	}
	return spec_iterator.NewSerialIterator(specs.Specs())
}
