package serialization

import (
	"fmt"
	"strings"
	"time"
)

// TimeOnly is represents the time part of a date time (time) value.
type TimeOnly struct {
	time time.Time
}

const timeOnlyFormat = "15:04:05.000000000"

var timeOnlyParsingFormats = map[int]string{
	0: "15:04:05", //Go doesn't seem to support optional parameters in time.Parse, which is sad
	1: "15:04:05.0",
	2: "15:04:05.00",
	3: "15:04:05.000",
	4: "15:04:05.0000",
	5: "15:04:05.00000",
	6: "15:04:05.000000",
	7: "15:04:05.0000000",
	8: "15:04:05.00000000",
	9: timeOnlyFormat,
}

// String returns the time only as a string following the RFC3339 standard.
// Uses zero precision (no fractional seconds) by default.
func (t TimeOnly) String() string {
	return t.StringWithPrecision(0)
}

// StringWithPrecision returns the time only as a string with the specified precision.
// precision: number of decimal places for nanoseconds (0-9)
func (t TimeOnly) StringWithPrecision(precision int) string {
	if precision < 0 || precision >= len(timeOnlyParsingFormats) {
		precision = 0
	}
	return t.time.Format(timeOnlyParsingFormats[precision])
}

// ParseTimeOnly parses a string into a TimeOnly following the RFC3339 standard.
func ParseTimeOnly(s string) (*TimeOnly, error) {
	timeOnly, _, err := ParseTimeOnlyWithPrecision(s)
	return timeOnly, err
}

// ParseTimeOnlyWithPrecision parses a string into a TimeOnly and returns the detected precision.
func ParseTimeOnlyWithPrecision(s string) (*TimeOnly, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, 0, nil
	}

	precision := 0
	if parts := strings.Split(s, "."); len(parts) > 1 {
		precision = len(parts[1])
		if precision >= len(timeOnlyParsingFormats) {
			return nil, 0, fmt.Errorf("time precision of %d exceeds maximum allowed of %d", precision, len(timeOnlyParsingFormats)-1)
		}
	}

	timeValue, err := time.Parse(timeOnlyParsingFormats[precision], s)
	if err != nil {
		return nil, 0, err
	}

	return &TimeOnly{time: timeValue}, precision, nil
}

// NewTimeOnly creates a new TimeOnly from a time.Time.
func NewTimeOnly(t time.Time) *TimeOnly {
	return &TimeOnly{time: t}
}

// DetectPrecision determines the precision (number of fractional second digits) from a time.Time.
func DetectPrecision(t time.Time) int {
	nanos := t.Nanosecond()
	if nanos == 0 {
		return 0
	}
	return len(strings.TrimRight(fmt.Sprintf("%09d", nanos), "0"))
}
