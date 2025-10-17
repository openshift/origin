package utility

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/sirupsen/logrus"
)

// SystemdJournalLogTime returns Now if there is trouble reading the time.  This will stack the event intervals without
// parsable times at the end of the run, which will be more clearly visible as a problem than not reporting them.
func SystemdJournalLogTime(logLine string, year int) time.Time {
	var kubeletTimeRegex = regexp.MustCompile(`^(?P<MONTH>\S+)\s(?P<DAY>\S+)\s(?P<TIME>\S+)`)
	kubeletTimeRegex.MatchString(logLine)
	if !kubeletTimeRegex.MatchString(logLine) {
		return time.Now()
	}

	month := ""
	day := ""
	yearStr := fmt.Sprintf("%d", year)
	timeOfDay := ""
	subMatches := kubeletTimeRegex.FindStringSubmatch(logLine)
	subNames := kubeletTimeRegex.SubexpNames()
	for i, name := range subNames {
		switch name {
		case "MONTH":
			month = subMatches[i]
		case "DAY":
			day = subMatches[i]
		case "TIME":
			timeOfDay = subMatches[i]
		}
	}

	timeString := fmt.Sprintf("%s %s %s %s UTC", day, month, yearStr, timeOfDay)
	ret, err := time.Parse("02 Jan 2006 15:04:05.999999999 MST", timeString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failure parsing time format: %v for %q\n", err, timeString)
		return time.Now()
	}

	return ret
}

// FindOverlap finds any intervals that overlap with the given interval.
func FindOverlap(intervals monitorapi.Intervals, overlapsWith monitorapi.Interval) monitorapi.Intervals {
	overlappingIntervals := monitorapi.Intervals{}
	for i := range intervals {
		interval := intervals[i]
		if IntervalsOverlap(interval, overlapsWith) {
			overlappingIntervals = append(overlappingIntervals, interval)
		}
	}

	return overlappingIntervals
}

// IntervalsOverlap checks if two intervals overlap in time
func IntervalsOverlap(interval1, interval2 monitorapi.Interval) bool {
	// If either interval has a zero end time, treat it as ongoing to the end of time
	end1 := interval1.To
	if end1.IsZero() {
		end1 = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}

	end2 := interval2.To
	if end2.IsZero() {
		end2 = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}

	// Check for overlap
	return (interval1.From.Before(end2)) && (interval2.From.Before(end1))
}

// CreateTcpdumpHookIfEnabled creates a tcpdump hook if DEBUG_ENABLE_TCPDUMP is enabled.
func CreateTcpdumpHookIfEnabled() *backenddisruption.TcpdumpSamplerHook {
	// Check if tcpdump collection is enabled via environment variable
	if os.Getenv("DEBUG_ENABLE_TCPDUMP") == "true" {
		return backenddisruption.NewTcpdumpSamplerHook()
	}
	return nil
}

// StopTcpdumpCollection stops tcpdump collection if the hook is not nil.
// This should be called in the CollectData method of monitor tests.
func StopTcpdumpCollection(tcpdumpHook *backenddisruption.TcpdumpSamplerHook) {
	if tcpdumpHook != nil {
		tcpdumpHook.StopCollection()
	}
}

// MoveTcpdumpToStorage moves tcpdump pcap files to the storage directory if the hook is not nil.
// This should be called in the WriteContentToStorage method of monitor tests.
// It logs any errors but doesn't fail the entire storage operation.
func MoveTcpdumpToStorage(tcpdumpHook *backenddisruption.TcpdumpSamplerHook, storageDir string) {
	if tcpdumpHook != nil {
		if err := tcpdumpHook.MoveToStorage(storageDir); err != nil {
			// Log error but don't fail the entire WriteContentToStorage operation
			logrus.WithError(err).Warn("Failed to move tcpdump pcap file to storage")
		}
	}
}
