package riskanalysis

import (
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

// Define types, these are subsets of the sippy APIs of the same name, copied here to eliminate a lot of the cruft.
// ProwJobRunTest defines a join table linking tests to the job runs they execute in, along with the status for
// that execution.
// We're getting dangerously close to being able to live push results after a job run.

type ProwJobRun struct {
	ID          int
	ProwJob     ProwJob
	ClusterData platformidentification.ClusterData
	Tests       []ProwJobRunTest
	TestCount   int
}

type ProwJob struct {
	Name string
}

type Test struct {
	Name string
}

type Suite struct {
	Name string
}

type ProwJobRunTest struct {
	Test   Test
	Suite  Suite
	Status int // would like to use smallint here, but gorm auto-migrate breaks trying to change the type every start
}
