package historicaldata

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

// nextBestGuessers is the order in which to attempt to lookup other alternative matches that are close to this job type.
var nextBestGuessers = []NextBestKey{
	// The only guesser we try not is falling back to previous release. Otherwise if we don't have enough data, we don't
	// run the test. This was implemented after finding that we fail every attempt at a fallback.
	// Continuing with previous release helps us in the transition between major releases, so we kept this fallback.
	PreviousReleaseUpgrade,
}

// NextBestKey returns the next best key in the query_results.json generated from BigQuery and a bool indicating whether this guesser has an opinion.
// If the bool is false, the key should not be used.
// Returning true doesn't mean the key exists, it just means that the key is worth trying.
type NextBestKey func(in platformidentification.JobType) (platformidentification.JobType, bool)

// PreviousReleaseUpgrade if we don't have data for the current toRelease, perhaps we have data for the congruent test
// on the prior release.   A 4.11 to 4.11 upgrade will attempt a 4.10 to 4.10 upgrade.  A 4.11 no upgrade, will attempt a 4.10 no upgrade.
func PreviousReleaseUpgrade(in platformidentification.JobType) (platformidentification.JobType, bool) {
	toReleaseMajor := getMajor(in.Release)
	toReleaseMinor := getMinor(in.Release)

	ret := platformidentification.CloneJobType(in)
	ret.Release = fmt.Sprintf("%d.%d", toReleaseMajor, toReleaseMinor-1)
	if len(in.FromRelease) > 0 {
		fromReleaseMinor := getMinor(in.FromRelease)
		ret.FromRelease = fmt.Sprintf("%d.%d", toReleaseMajor, fromReleaseMinor-1)
	}
	return ret, true
}

func getMajor(in string) int {
	major, err := strconv.ParseInt(strings.Split(in, ".")[0], 10, 32)
	if err != nil {
		panic(err)
	}
	return int(major)
}

func getMinor(in string) int {
	minor, err := strconv.ParseInt(strings.Split(in, ".")[1], 10, 32)
	if err != nil {
		panic(err)
	}
	return int(minor)
}

func CurrentReleaseFromMap(releasesInQueryResults map[string]bool) string {
	var releaseSlice []struct {
		Key   string
		Value bool
	}
	for key, value := range releasesInQueryResults {
		releaseSlice = append(releaseSlice, struct {
			Key   string
			Value bool
		}{Key: key, Value: value})
	}

	// Sort the slice in descending order
	sort.Slice(releaseSlice, func(i, j int) bool {
		return compareReleaseString(releaseSlice[i].Key, releaseSlice[j].Key)
	})
	var firstKey string
	// Access the first key from the sorted slice
	if len(releaseSlice) > 0 {
		firstKey = releaseSlice[0].Key
	}
	return firstKey
}

func compareReleaseString(one, two string) bool {
	major1 := getMajor(one)
	major2 := getMajor(two)
	if major1 == major2 {
		minor1 := getMinor(one)
		minor2 := getMinor(two)
		return minor1 > minor2
	}
	return major1 > major2
}
