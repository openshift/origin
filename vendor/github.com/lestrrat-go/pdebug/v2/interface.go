package pdebug

import "time"

type Clock interface {
	Now() time.Time
}

type ClockFunc func() time.Time

func (cf ClockFunc) Now() time.Time {
	return cf()
}
