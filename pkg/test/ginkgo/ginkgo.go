package ginkgo

import (
	"fmt"

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
	return tests, nil
}
