package ginkgo

import (
	"fmt"
	"math/rand"

	"github.com/onsi/ginkgo/v2"
	"github.com/openshift/origin/test/extended/util/annotate/generated"
)

func testsForSuite() ([]*testCase, error) {
	if err := ginkgo.GetSuite().BuildTree(); err != nil {
		return nil, err
	}
	specs := ginkgo.GetSpecs()
	var tests []*testCase
	for _, spec := range specs {
		if append, ok := generated.Annotations[spec.Text()]; ok {
			spec.AppendText(append)

		} else {
			panic(fmt.Sprintf("unable to find test %s", spec.Text()))
		}

		tc, err := newTestCaseFromGinkgoSpec(ginkgo.Spec{InternalSpec: spec})
		if err != nil {
			return nil, err
		}
		tests = append(tests, tc)
	}
	suiteConfig, _ := ginkgo.GinkgoConfiguration()
	r := rand.New(rand.NewSource(suiteConfig.RandomSeed))
	r.Shuffle(len(tests), func(i, j int) { tests[i], tests[j] = tests[j], tests[i] })
	return tests, nil
}
