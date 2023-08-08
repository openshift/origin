package historicaldata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"github.com/sirupsen/logrus"
)

// minJobRuns is the required threshold for historical data to be sufficient to run the test.
// If we find matchig historical data but not enough job runs, we either fallback to the next
// best matcher, or failing that, skip the test entirely. We require 100 runs because we're
// attempting to match on a P99, and any less than this is logically not a P99.
const minJobRuns = 100

type StatisticalDuration struct {
	platformidentification.JobType `json:",inline"`
	P95                            time.Duration
	P99                            time.Duration
}

type DisruptionStatisticalData struct {
	DataKey `json:",inline"`
	P95     float64
	P99     float64
	JobRuns int64
}

type DataKey struct {
	BackendName string

	platformidentification.JobType `json:",inline"`
}

type DisruptionBestMatcher struct {
	HistoricalData map[DataKey]DisruptionStatisticalData
}

func NewDisruptionMatcher(historicalJSON []byte) (*DisruptionBestMatcher, error) {
	historicalData := map[DataKey]DisruptionStatisticalData{}

	inFile := bytes.NewBuffer(historicalJSON)
	jsonDecoder := json.NewDecoder(inFile)

	type DecodingPercentile struct {
		DataKey `json:",inline"`
		P95     string
		P99     string
		JobRuns int64
	}
	decodingPercentilesList := []DecodingPercentile{}

	if err := jsonDecoder.Decode(&decodingPercentilesList); err != nil {
		return nil, err
	}

	for _, currDecoded := range decodingPercentilesList {
		p95, err := strconv.ParseFloat(currDecoded.P95, 64)
		if err != nil {
			return nil, err
		}
		p99, err := strconv.ParseFloat(currDecoded.P99, 64)
		if err != nil {
			return nil, err
		}
		curr := DisruptionStatisticalData{
			DataKey: currDecoded.DataKey,
			P95:     p95,
			P99:     p99,
			JobRuns: currDecoded.JobRuns,
		}
		historicalData[curr.DataKey] = curr
	}

	return &DisruptionBestMatcher{
		HistoricalData: historicalData,
	}, nil
}

func NewDisruptionMatcherWithHistoricalData(data map[DataKey]DisruptionStatisticalData) *DisruptionBestMatcher {
	return &DisruptionBestMatcher{
		HistoricalData: data,
	}
}

func (b *DisruptionBestMatcher) bestMatch(name string, jobType platformidentification.JobType) (DisruptionStatisticalData, string, error) {
	exactMatchKey := DataKey{
		BackendName: name,
		JobType:     jobType,
	}
	logrus.WithField("backend", name).Infof("searching for bestMatch for %+v", jobType)
	logrus.Infof("historicalData has %d entries", len(b.HistoricalData))
	if percentiles, ok := b.HistoricalData[exactMatchKey]; ok {
		if percentiles.JobRuns > minJobRuns {
			logrus.Infof("found exact match: %+v", percentiles)
			return percentiles, "", nil
		}

		percentiles, matchReason, err := b.evaluateBestGuesser(name, PreviousReleaseUpgrade, exactMatchKey, jobType)
		if err != nil {
			return DisruptionStatisticalData{}, "", err
		}
		if percentiles != (DisruptionStatisticalData{}) {
			return percentiles, matchReason, nil
		}
	}
	// tested in TestGetClosestP99Value in allowedbackendisruption.  Should get a local test at some point.
	for _, nextBestGuesser := range nextBestGuessers {
		percentiles, matchReason, err := b.evaluateBestGuesser(name, nextBestGuesser, exactMatchKey, jobType)
		if err != nil {
			return DisruptionStatisticalData{}, "", err
		}
		if percentiles != (DisruptionStatisticalData{}) {
			return percentiles, matchReason, nil
		}
	}

	logrus.Warn("no exact or fuzzy match, no results will be returned, test will be skipped")

	// TODO: ensure our core platforms are here, error if not. We need to be sure our aggregated jobs are running this
	// but in a way that won't require manual code maintenance every release...

	// We now only track disruption data for frequently run jobs where we have enough runs to make a reliable P95 or P99
	// determination. If we did not record historical data for this NURP combination, we do not wish to enforce
	// disruption testing on a per job basis. Return an empty data result to signal we have no data, and skip the test.
	return DisruptionStatisticalData{},
		fmt.Sprintf("(no exact or fuzzy match for jobType=%#v)", jobType),
		nil
}

func (b *DisruptionBestMatcher) evaluateBestGuesser(name string, nextBestGuesser NextBestKey, exactMatchKey DataKey, jobType platformidentification.JobType) (DisruptionStatisticalData, string, error) {
	nextBestJobType, ok := nextBestGuesser(jobType)
	if !ok {
		return DisruptionStatisticalData{}, "", nil
	}
	nextBestMatchKey := DataKey{
		BackendName: name,
		JobType:     nextBestJobType,
	}
	if percentiles, ok := b.HistoricalData[nextBestMatchKey]; ok && percentiles.JobRuns > minJobRuns {
		logrus.Infof("no exact match fell back to %#v", nextBestMatchKey)
		logrus.Infof("found inexact match: %+v", percentiles)
		return percentiles, fmt.Sprintf("(no exact match for %#v, fell back to %#v)", exactMatchKey, nextBestMatchKey), nil
	}
	return DisruptionStatisticalData{}, "", nil
}

// BestMatchDuration returns the best possible match for this historical data.  It attempts an exact match first, then
// it attempts to match on the most important keys in order, before giving up and returning an empty default,
// which means to skip testing against this data.
func (b *DisruptionBestMatcher) BestMatchDuration(name string, jobType platformidentification.JobType) (StatisticalDuration, string, error) {
	rawData, details, err := b.bestMatch(name, jobType)
	// Empty data implies we have none, and thus do not want to run the test.
	if rawData == (DisruptionStatisticalData{}) {
		return StatisticalDuration{}, details, err
	}
	return toStatisticalDuration(rawData), details, err
}

func (b *DisruptionBestMatcher) BestMatchP99(name string, jobType platformidentification.JobType) (*time.Duration, string, error) {
	rawData, details, err := b.BestMatchDuration(name, jobType)
	if rawData == (StatisticalDuration{}) {
		return nil, details, err
	}
	return &rawData.P99, details, err
}

func toStatisticalDuration(in DisruptionStatisticalData) StatisticalDuration {
	return StatisticalDuration{
		JobType: in.DataKey.JobType,
		P95:     DurationOrDie(in.P95),
		P99:     DurationOrDie(in.P99),
	}
}

func DurationOrDie(seconds float64) time.Duration {
	ret, err := time.ParseDuration(fmt.Sprintf("%.3fs", seconds))
	if err != nil {
		panic(err)
	}
	return ret
}
