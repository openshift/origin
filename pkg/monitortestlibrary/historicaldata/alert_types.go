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

type AlertStatisticalData struct {
	AlertDataKey `json:",inline"`
	Name         string
	P95          float64
	P99          float64
	JobRuns      int64
}

type AlertDataKey struct {
	AlertName      string
	AlertNamespace string
	AlertLevel     string

	platformidentification.JobType `json:",inline"`
}

type AlertBestMatcher struct {
	HistoricalData map[AlertDataKey]AlertStatisticalData
}

func NewAlertMatcher(historicalJSON []byte) (*AlertBestMatcher, error) {
	historicalData := map[AlertDataKey]AlertStatisticalData{}

	inFile := bytes.NewBuffer(historicalJSON)
	jsonDecoder := json.NewDecoder(inFile)

	type DecodingPercentile struct {
		AlertDataKey `json:",inline"`
		P95          string
		P99          string
		JobRuns      int64
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
		curr := AlertStatisticalData{
			AlertDataKey: currDecoded.AlertDataKey,
			P95:          p95,
			P99:          p99,
			JobRuns:      currDecoded.JobRuns,
		}
		historicalData[curr.AlertDataKey] = curr
	}

	return &AlertBestMatcher{
		HistoricalData: historicalData,
	}, nil
}

func NewAlertMatcherWithHistoricalData(data map[AlertDataKey]AlertStatisticalData) *AlertBestMatcher {
	return &AlertBestMatcher{
		HistoricalData: data,
	}
}

func (b *AlertBestMatcher) bestMatch(key AlertDataKey) (AlertStatisticalData, string, error) {
	exactMatchKey := key
	logrus.WithField("alertName", key.AlertName).Infof("searching for bestMatch for %+v", key.JobType)
	logrus.Infof("historicalData has %d entries", len(b.HistoricalData))

	if percentiles, ok := b.HistoricalData[exactMatchKey]; ok {
		if percentiles.JobRuns > minJobRuns {
			logrus.Infof("found exact match: %+v", percentiles)
			return percentiles, "", nil
		}
		percentiles, matchReason, err := b.evaluateBestGuesser(PreviousReleaseUpgrade, exactMatchKey)
		if err != nil {
			return AlertStatisticalData{}, "", err
		}
		if percentiles != (AlertStatisticalData{}) {
			return percentiles, matchReason, nil
		}
	}

	// tested in TestGetClosestP95Value in allowedbackendisruption.  Should get a local test at some point.
	for _, nextBestGuesser := range nextBestGuessers {
		percentiles, matchReason, err := b.evaluateBestGuesser(nextBestGuesser, exactMatchKey)
		if err != nil {
			return AlertStatisticalData{}, "", err
		}
		if percentiles != (AlertStatisticalData{}) {
			return percentiles, matchReason, nil
		}
	}

	// TODO: ensure our core platforms are here, error if not. We need to be sure our aggregated jobs are running this
	// but in a way that won't require manual code maintenance every release...

	// We now only track disruption data for frequently run jobs where we have enough runs to make a reliable P95 or P99
	// determination. If we did not record historical data for this NURP combination, we do not wish to enforce
	// disruption testing on a per job basis. Return an empty data result to signal we have no data, and skip the test.
	return AlertStatisticalData{},
		fmt.Sprintf("(no exact or fuzzy match for jobType=%#v)", key.JobType),
		nil
}

func (b *AlertBestMatcher) evaluateBestGuesser(nextBestGuesser NextBestKey, exactMatchKey AlertDataKey) (AlertStatisticalData, string, error) {
	nextBestJobType, ok := nextBestGuesser(exactMatchKey.JobType)
	if !ok {
		return AlertStatisticalData{}, "", nil
	}
	nextBestMatchKey := AlertDataKey{
		AlertName:      exactMatchKey.AlertName,
		AlertNamespace: exactMatchKey.AlertNamespace,
		AlertLevel:     exactMatchKey.AlertLevel,

		JobType: nextBestJobType,
	}
	if percentiles, ok := b.HistoricalData[nextBestMatchKey]; ok && percentiles.JobRuns >= minJobRuns {
		return percentiles, fmt.Sprintf("(no exact match for %#v, fell back to %#v)", exactMatchKey, nextBestMatchKey), nil
	}
	return AlertStatisticalData{}, "", nil
}

// BestMatchDuration returns the best possible match for this historical data.  It attempts an exact match first, then
// it attempts to match on the most important keys in order, before giving up and returning an empty default,
// which means to skip testing against this data.
func (b *AlertBestMatcher) BestMatchDuration(key AlertDataKey) (StatisticalDuration, string, error) {
	rawData, details, err := b.bestMatch(key)
	// Empty data implies we have none, and thus do not want to run the test.
	if rawData == (AlertStatisticalData{}) {
		return StatisticalDuration{}, details, err
	}
	return toAlertStatisticalDuration(rawData), details, err
}

func (b *AlertBestMatcher) BestMatchP99(key AlertDataKey) (*time.Duration, string, error) {
	rawData, details, err := b.BestMatchDuration(key)
	if rawData == (StatisticalDuration{}) {
		return nil, details, err
	}
	return &rawData.P99, details, err
}

func toAlertStatisticalDuration(in AlertStatisticalData) StatisticalDuration {
	return StatisticalDuration{
		JobType: in.AlertDataKey.JobType,
		P95:     DurationOrDie(in.P95),
		P99:     DurationOrDie(in.P99),
	}
}
