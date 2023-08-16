package historicaldata

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

// nextBestGuessers is the order in which to attempt to lookup other alternative matches that are close to this job type.
// TODO building a cross multiply would likely be beneficial
var nextBestGuessers = []NextBestKey{
	MicroReleaseUpgrade,
	//	MinorReleaseUpgrade,
	PreviousReleaseUpgrade,

	combine(PreviousReleaseUpgrade, MicroReleaseUpgrade),
	//	combine(PreviousReleaseUpgrade, MinorReleaseUpgrade),

	OnArchitecture("amd64"),
	OnArchitecture("ppc64le"),
	OnArchitecture("s390x"),
	OnArchitecture("arm64"),

	combine(OnArchitecture("amd64"), MicroReleaseUpgrade),
	combine(OnArchitecture("ppc64le"), MicroReleaseUpgrade),
	combine(OnArchitecture("s390x"), MicroReleaseUpgrade),
	combine(OnArchitecture("arm64"), MicroReleaseUpgrade),

	//	combine(OnArchitecture("amd64"), MinorReleaseUpgrade),
	//	combine(OnArchitecture("ppc64le"), MinorReleaseUpgrade),
	//	combine(OnArchitecture("s390x"), MinorReleaseUpgrade),
	//	combine(OnArchitecture("arm64"), MinorReleaseUpgrade),

	combine(OnArchitecture("amd64"), PreviousReleaseUpgrade),
	combine(OnArchitecture("ppc64le"), PreviousReleaseUpgrade),
	combine(OnArchitecture("s390x"), PreviousReleaseUpgrade),
	combine(OnArchitecture("arm64"), PreviousReleaseUpgrade),

	combine(OnArchitecture("amd64"), PreviousReleaseUpgrade, MicroReleaseUpgrade),
	combine(OnArchitecture("ppc64le"), PreviousReleaseUpgrade, MicroReleaseUpgrade),
	combine(OnArchitecture("s390x"), PreviousReleaseUpgrade, MicroReleaseUpgrade),
	combine(OnArchitecture("arm64"), PreviousReleaseUpgrade, MicroReleaseUpgrade),

	//	combine(OnArchitecture("amd64"), PreviousReleaseUpgrade, MinorReleaseUpgrade),
	//	combine(OnArchitecture("ppc64le"), PreviousReleaseUpgrade, MinorReleaseUpgrade),
	//	combine(OnArchitecture("s390x"), PreviousReleaseUpgrade, MinorReleaseUpgrade),
	//	combine(OnArchitecture("arm64"), PreviousReleaseUpgrade, MinorReleaseUpgrade),

	combine(ForTopology("single"), OnSDN),
	combine(ForTopology("single"), OnSDN, PreviousReleaseUpgrade),
	combine(ForTopology("single"), OnSDN, PreviousReleaseUpgrade, MicroReleaseUpgrade),
}

// NextBestKey returns the next best key in the query_results.json generated from BigQuery and a bool indicating whether this guesser has an opinion.
// If the bool is false, the key should not be used.
// Returning true doesn't mean the key exists, it just means that the key is worth trying.
type NextBestKey func(in platformidentification.JobType) (platformidentification.JobType, bool)

// MinorReleaseUpgrade if we don't have data for the current fromRelease and it's a micro upgrade, perhaps we have data
// for a minor upgrade.  A 4.11 to 4.11 upgrade will attempt a 4.10 to 4.11 upgrade.
func MinorReleaseUpgrade(in platformidentification.JobType) (platformidentification.JobType, bool) {
	if len(in.FromRelease) == 0 {
		return platformidentification.JobType{}, false
	}

	fromReleaseMinor := getMinor(in.FromRelease)
	toReleaseMajor := getMajor(in.Release)
	toReleaseMinor := getMinor(in.Release)
	// if we're already a minor upgrade, this doesn't apply
	if fromReleaseMinor == (toReleaseMinor - 1) {
		return platformidentification.JobType{}, false
	}

	ret := platformidentification.CloneJobType(in)
	ret.FromRelease = fmt.Sprintf("%d.%d", toReleaseMajor, toReleaseMinor-1)
	return ret, true
}

// MicroReleaseUpgrade if we don't have data for the current fromRelease and it's a minor upgrade, perhaps we have data
// for a micro upgrade.  A 4.10 to 4.11 upgrade will attempt a 4.11 to 4.11 upgrade.
func MicroReleaseUpgrade(in platformidentification.JobType) (platformidentification.JobType, bool) {
	if len(in.FromRelease) == 0 {
		return platformidentification.JobType{}, false
	}

	fromReleaseMinor := getMinor(in.FromRelease)
	toReleaseMajor := getMajor(in.Release)
	toReleaseMinor := getMinor(in.Release)
	// if we're already a micro upgrade, this doesn't apply
	if fromReleaseMinor == toReleaseMinor {
		return platformidentification.JobType{}, false
	}

	ret := platformidentification.CloneJobType(in)
	ret.FromRelease = fmt.Sprintf("%d.%d", toReleaseMajor, toReleaseMinor)
	return ret, true
}

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

// OnOVN maybe we have data on OVN
func OnOVN(in platformidentification.JobType) (platformidentification.JobType, bool) {
	if in.Network == "ovn" {
		return platformidentification.JobType{}, false
	}

	ret := platformidentification.CloneJobType(in)
	ret.Network = "ovn"
	return ret, true
}

// OnSDN maybe we have data on SDN
func OnSDN(in platformidentification.JobType) (platformidentification.JobType, bool) {
	if in.Network == "sdn" {
		return platformidentification.JobType{}, false
	}

	ret := platformidentification.CloneJobType(in)
	ret.Network = "sdn"
	return ret, true
}

// OnArchitecture maybe we match a different architecture
func OnArchitecture(architecture string) func(in platformidentification.JobType) (platformidentification.JobType, bool) {
	return func(in platformidentification.JobType) (platformidentification.JobType, bool) {
		if in.Architecture == architecture {
			return platformidentification.JobType{}, false
		}

		ret := platformidentification.CloneJobType(in)
		ret.Architecture = architecture
		return ret, true
	}
}

// ForTopology we match on exact topology
func ForTopology(topology string) func(in platformidentification.JobType) (platformidentification.JobType, bool) {
	return func(in platformidentification.JobType) (platformidentification.JobType, bool) {
		if in.Topology != topology {
			return platformidentification.JobType{}, false
		}
		return in, true
	}
}

// combine will start with the input and call each guess in order.  It uses the output of the previous NextBestKeyFn
// as the input to the next.  This allows combinations like "previous release upgrade micro" without writing custom
// functions for each.
func combine(nextBestKeys ...NextBestKey) NextBestKey {
	return func(in platformidentification.JobType) (platformidentification.JobType, bool) {
		curr := in
		for _, nextBestKey := range nextBestKeys {
			var ok bool
			curr, ok = nextBestKey(curr)
			if !ok {
				return curr, false
			}
		}
		return curr, true
	}
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
