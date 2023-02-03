package historicaldata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

type StatisticalDuration struct {
	platformidentification.JobType `json:",inline"`
	P95                            time.Duration
	P99                            time.Duration
}

type DisruptionStatisticalData struct {
	DataKey `json:",inline"`
	P95     float64
	P99     float64
}

type DataKey struct {
	BackendName string

	platformidentification.JobType `json:",inline"`
}

type DisruptionBestMatcher struct {
	historicalData map[DataKey]DisruptionStatisticalData
}

func NewDisruptionMatcher(historicalJSON []byte) (*DisruptionBestMatcher, error) {
	historicalData := map[DataKey]DisruptionStatisticalData{}

	inFile := bytes.NewBuffer(historicalJSON)
	jsonDecoder := json.NewDecoder(inFile)

	type DecodingPercentile struct {
		DataKey `json:",inline"`
		P95     string
		P99     string
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
		}
		historicalData[curr.DataKey] = curr
	}

	return &DisruptionBestMatcher{
		historicalData: historicalData,
	}, nil
}

func NewDisruptionMatcherWithHistoricalData(data map[DataKey]DisruptionStatisticalData) *DisruptionBestMatcher {
	return &DisruptionBestMatcher{
		historicalData: data,
	}
}

func (b *DisruptionBestMatcher) bestMatch(name string, jobType platformidentification.JobType) (DisruptionStatisticalData, string, error) {
	exactMatchKey := DataKey{
		BackendName: name,
		JobType:     jobType,
	}

	if percentiles, ok := b.historicalData[exactMatchKey]; ok {
		return percentiles, "", nil
	}

	// tested in TestGetClosestP95Value in allowedbackendisruption.  Should get a local test at some point.
	for _, nextBestGuesser := range nextBestGuessers {
		nextBestJobType, ok := nextBestGuesser(jobType)
		if !ok {
			continue
		}
		nextBestMatchKey := DataKey{
			BackendName: name,
			JobType:     nextBestJobType,
		}
		if percentiles, ok := b.historicalData[nextBestMatchKey]; ok {
			return percentiles, fmt.Sprintf("(no exact match for %#v, fell back to %#v)", exactMatchKey, nextBestMatchKey), nil
		}
	}

	// TODO: ensure our core platforms are here, error if not. We need to be sure our aggregated jobs are running this
	// but in a way that won't require manual code maintenance every release...

	// We now only track disruption data for frequently run jobs where we have enough runs to make a reliable P95 or P99
	// determination. If we did not record historical data for this NURP combination, we do not wish to enforce
	// disruption testing on a per job basis. Return an empty data result to signal we have no data, and skip the test.
	return DisruptionStatisticalData{},
		fmt.Sprintf("(no exact or fuzzy match for jobType=%#v)", jobType),
		nil
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
