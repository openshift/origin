package historicaldata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

type BestMatcher interface {
	// BestMatch returns the best possible match for this historical data.  It attempts a full match first, then
	// it attempts to match on the most important keys in order, before giving up and returning a default.
	BestMatch(name string, jopType platformidentification.JobType) (StatisticalData, string, error)
	// BestMatchDuration returns the best possible match for this historical data.  It attempts a full match first, then
	// it attempts to match on the most important keys in order, before giving up and returning a default.
	BestMatchDuration(name string, jopType platformidentification.JobType) (StatisticalDuration, string, error)

	BestMatchP99(name string, jobType platformidentification.JobType) (*time.Duration, string, error)
}

type StatisticalDuration struct {
	DataKey `json:",inline"`
	P95     time.Duration
	P99     time.Duration
}

type StatisticalData struct {
	DataKey `json:",inline"`
	P95     float64
	P99     float64
}

type DataKey struct {
	// Name is the identifier for the particular bit of data.  It's like BackendName or AlertName
	Name string

	platformidentification.JobType `json:",inline"`
}

type bestMatcher struct {
	historicalData map[DataKey]StatisticalData
	defaultReturn  float64
}

func NewMatcher(historicalJSON []byte, defaultReturn float64) (BestMatcher, error) {
	historicalData := map[DataKey]StatisticalData{}

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
		curr := StatisticalData{
			DataKey: currDecoded.DataKey,
			P95:     p95,
			P99:     p99,
		}
		historicalData[curr.DataKey] = curr
	}

	return &bestMatcher{
		historicalData: historicalData,
		defaultReturn:  defaultReturn,
	}, nil
}

func (b *bestMatcher) BestMatch(name string, jobType platformidentification.JobType) (StatisticalData, string, error) {
	exactMatchKey := DataKey{
		Name:    name,
		JobType: jobType,
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
			Name:    name,
			JobType: nextBestJobType,
		}
		if percentiles, ok := b.historicalData[nextBestMatchKey]; ok {
			return percentiles, fmt.Sprintf("(no exact match for %#v, fell back to %#v)", exactMatchKey, nextBestMatchKey), nil
		}
	}

	defaultReturn := StatisticalData{
		DataKey: exactMatchKey,
		P95:     b.defaultReturn,
		P99:     b.defaultReturn,
	}
	return defaultReturn,
		fmt.Sprintf("(no exact or fuzzy match for jobType=%#v)", jobType),
		nil
}

func (b *bestMatcher) BestMatchDuration(name string, jobType platformidentification.JobType) (StatisticalDuration, string, error) {
	rawData, details, err := b.BestMatch(name, jobType)
	return toStatisticalDuration(rawData), details, err
}

func (b *bestMatcher) BestMatchP99(name string, jobType platformidentification.JobType) (*time.Duration, string, error) {
	rawData, details, err := b.BestMatchDuration(name, jobType)
	return &rawData.P99, details, err
}

func toStatisticalDuration(in StatisticalData) StatisticalDuration {
	return StatisticalDuration{
		DataKey: in.DataKey,
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
