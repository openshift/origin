package allowedalerts

import (
	"time"

	"github.com/openshift/origin/pkg/synthetictests/historicaldata"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

type neverFailAllowance struct {
	flakeDelegate AlertTestAllowanceCalculator
}

func neverFail(flakeDelegate AlertTestAllowanceCalculator) AlertTestAllowanceCalculator {
	return &neverFailAllowance{
		flakeDelegate,
	}
}

func (d *neverFailAllowance) FailAfter(alertName string, jobType platformidentification.JobType) (time.Duration, error) {
	return 24 * time.Hour, nil
}

func (d *neverFailAllowance) FlakeAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	return d.flakeDelegate.FlakeAfter(alertName, jobType)
}

// AlertTestAllowanceCalculator provides the duration after which an alert test should flake and fail.
// For instance, for if the alert test is checking pending, and the alert is pending for 4s and the FailAfter
// returns 6s and the FlakeAfter returns 2s, then test will flake.
type AlertTestAllowanceCalculator interface {
	// FailAfter returns a duration an alert can be at or above the required state before failing.
	FailAfter(alertName string, jobType platformidentification.JobType) (time.Duration, error)
	// FlakeAfter returns a duration an alert can be at or above the required state before flaking.
	FlakeAfter(alertName string, jobType platformidentification.JobType) time.Duration
}

type percentileAllowances struct {
}

var defaultAllowances = &percentileAllowances{}

func (d *percentileAllowances) FailAfter(alertName string, jobType platformidentification.JobType) (time.Duration, error) {
	allowed, _, _ := getClosestPercentilesValues(alertName, jobType)
	return allowed.P99, nil
}

func (d *percentileAllowances) FlakeAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	allowed, _, _ := getClosestPercentilesValues(alertName, jobType)
	return allowed.P95
}

// getClosestPercentilesValues uses the backend and information about the cluster to choose the best historical p99 to operate against.
// We enforce "don't get worse" for disruption by watching the aggregate data in CI over many runs.
func getClosestPercentilesValues(alertName string, jobType platformidentification.JobType) (historicaldata.StatisticalDuration, string, error) {
	return getCurrentResults().BestMatchDuration(alertName, jobType)
}
