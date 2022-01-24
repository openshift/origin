package allowedalerts

import (
	"fmt"
	"time"

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

func (d *neverFailAllowance) FailAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	return 24 * time.Hour
}

func (d *neverFailAllowance) FlakeAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	return d.flakeDelegate.FlakeAfter(alertName, jobType)
}

// AlertTestAllowanceCalculator provides the duration after which an alert test should flake and fail.
// For instance, for if the alert test is checking pending, and the alert is pending for 4s and the FailAfter
// returns 6s and the FlakeAfter returns 2s, then test will flake.
type AlertTestAllowanceCalculator interface {
	// FailAfter returns a duration an alert can be at or above the required state before failing.
	FailAfter(alertName string, jobType platformidentification.JobType) time.Duration
	// FlakeAfter returns a duration an alert can be at or above the required state before flaking.
	FlakeAfter(alertName string, jobType platformidentification.JobType) time.Duration
}

type percentileAllowances struct {
}

var defaultAllowances = &percentileAllowances{}

func (d *percentileAllowances) FailAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	allowed := getClosestPercentilesValues(alertName, jobType)
	return allowed.P99
}

func (d *percentileAllowances) FlakeAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	allowed := getClosestPercentilesValues(alertName, jobType)
	return allowed.P95
}

// getClosestPercentilesValues uses the backend and information about the cluster to choose the best historical p95 to operate against.
// We enforce "don't get worse" for disruption by watching the aggregate data in CI over many runs.
func getClosestPercentilesValues(alertName string, jobType platformidentification.JobType) *percentileDuration {
	exactMatchKey := LastWeekPercentileKey{
		AlertName:   alertName,
		Release:     jobType.Release,
		FromRelease: jobType.FromRelease,
		Platform:    jobType.Platform,
		Network:     jobType.Network,
		Topology:    jobType.Topology,
	}
	_, percentileAsMap := getCurrentResults()

	// chose so we can find them easily in the log
	defaultSeconds, err := time.ParseDuration("2.718s")
	if err != nil {
		panic(err)
	}
	defaultPercentiles := &percentileDuration{
		P95: defaultSeconds,
		P99: defaultSeconds,
	}

	if percentiles, ok := percentileAsMap[exactMatchKey]; ok {
		p99, err := time.ParseDuration(fmt.Sprintf("%2fs", percentiles.P99))
		if err != nil {
			panic(err)
		}
		p95, err := time.ParseDuration(fmt.Sprintf("%2fs", percentiles.P95))
		if err != nil {
			panic(err)
		}
		return &percentileDuration{
			P95: p95,
			P99: p99,
		}
	}

	return defaultPercentiles
}

type percentileDuration struct {
	P95 time.Duration
	P99 time.Duration
}
