package time

import (
	"fmt"
	"strconv"
	"time"
	"unicode"
)

type (
	// UnitError is generated when an unknown unit is parsed from a duration string
	UnitError struct {
		Unit string
	}

	// FormatError is generated when an invalid duration string is parsed; the format
	// of the duration string is completely unrecognized in this case.
	FormatError struct {
		Duration string
	}
)

func (ue *UnitError) Error() string   { return fmt.Sprintf("unknown duration unit %q", ue.Unit) }
func (fe *FormatError) Error() string { return fmt.Sprintf("invalid duration %q", fe.Duration) }

// ParseDuration parses the given string and returns a numeric Duration. The format of the string must
// be consistent with that expected by the Mesos stout library; the string should consist of two parts,
// a floating-point numeric followed by a unit (no spaces in between).
// The following units are recognized: "ns", "us", "ms", "secs", "mins", "hrs", "days", "weeks".
// Examples of valid input strings are "10ns" and "1.5days".
// see https://github.com/apache/mesos/blob/4d2b1b793e07a9c90b984ca330a3d7bc9e1404cc/3rdparty/libprocess/3rdparty/stout/include/stout/duration.hpp
func ParseDuration(value string) (time.Duration, error) {
	for i, rv := range value {
		if unicode.IsDigit(rv) || rv == '.' {
			continue
		}
		num, err := strconv.ParseFloat(value[:i], 64)
		if err != nil {
			return 0, err
		}
		switch unit := value[i:]; unit {
		case "ns":
			// golang doesn't support fractional nanoseconds so we'll truncate
			return time.Duration(num), nil
		case "us":
			return time.Duration(num * float64(time.Microsecond)), nil
		case "ms":
			return time.Duration(num * float64(time.Millisecond)), nil
		case "secs":
			return time.Duration(num * float64(time.Second)), nil
		case "mins":
			return time.Duration(num * float64(time.Minute)), nil
		case "hrs":
			return time.Duration(num * float64(time.Hour)), nil
		case "days":
			return time.Duration(num * float64(time.Hour*24)), nil
		case "weeks":
			return time.Duration(num * float64(time.Hour*24*7)), nil
		default:
			return 0, &UnitError{unit}
		}
	}
	return 0, &FormatError{value}
}
