package ginkgo

import (
	"math/rand"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"

	"k8s.io/apimachinery/pkg/util/errors"
)

func testsForSuite() ([]*testCase, error) {
	var tests []*testCase
	var errs []error

	// Don't build the tree multiple times, it results in multiple initing of tests
	if !ginkgo.GetSuite().InPhaseBuildTree() {
		ginkgo.GetSuite().BuildTree()
	}

	ginkgo.GetSuite().WalkTests(func(name string, spec types.TestSpec) {
		tc, err := newTestCaseFromGinkgoSpec(spec)
		if err != nil {
			errs = append(errs, err)
		}
		tests = append(tests, tc)
	})
	if len(errs) > 0 {
		return nil, errors.NewAggregate(errs)
	}
	suiteConfig, _ := ginkgo.GinkgoConfiguration()
	random := rand.New(rand.NewSource(suiteConfig.RandomSeed))
	random.Shuffle(len(tests), func(i, j int) { tests[i], tests[j] = tests[j], tests[i] })
	return tests, nil
}
