package ipmi

import (
	"fmt"
	"time"
)

// RateUnit represents the duration over which a basic unit is given. It lets us
// distinguish between n times per millisecond and n times per day, among other
// values. Rate units are specified in byte 21 of the Full Sensor Record table
// in 37.1 and 43.1 of v1.5 and v2.0 respectively. This is a 3-bit uint on the
// wire.
type RateUnit uint8

const (
	RateUnitNone RateUnit = iota
	RateUnitPerMicrosecond
	RateUnitPerMillisecond
	RateUnitPerSecond
	RateUnitPerMinute
	RateUnitPerHour
	RateUnitPerDay
)

var (
	rateUnitDurations = map[RateUnit]time.Duration{
		RateUnitPerMicrosecond: time.Microsecond,
		RateUnitPerMillisecond: time.Millisecond,
		RateUnitPerSecond:      time.Second,
		RateUnitPerMinute:      time.Minute,
		RateUnitPerHour:        time.Hour,
		RateUnitPerDay:         time.Hour * 24,
	}
)

// Duration returns the underlying "per" period within the rate unit. Returns 0
// if called on an unrecognised rate unit.
func (r RateUnit) Duration() time.Duration {
	if duration, ok := rateUnitDurations[r]; ok {
		return duration
	}
	return 0
}

func (r RateUnit) String() string {
	formatted := "None"
	if duration := r.Duration(); duration > 0 {
		formatted = duration.String()
	}
	return fmt.Sprintf("%#v(%v)", uint8(r), formatted)
}
