package backend

import (
	"fmt"
	"time"
)

// ShutdownResponse holds the result of parsing the
// 'X-OpenShift-Disruption' response header:
//
//	format: shutdown=%t shutdown-delay-duration=%s elapsed=%s host=%s
type ShutdownResponse struct {
	// ShutdownInProgress is true if a graceful shutdown was in
	// progress when the request was received by the server,
	// otherwise this is false.
	ShutdownInProgress bool

	// ShutdownDelayDuration is the value of the --shutdown-delay-duration
	// server run option in use.
	ShutdownDelayDuration time.Duration

	// Elapsed is the amount of time elapsed since the server has
	// received the TERM signal, if the server is not shutting
	// down then this value is zero.
	Elapsed time.Duration

	// Hostname is the hostname of the apiserver process
	Hostname string
}

func (sr ShutdownResponse) String() string {
	return fmt.Sprintf("shutdown=%t shutdown-delay-duration=%s shutdown-elapsed=%s host=%s",
		sr.ShutdownInProgress, sr.ShutdownDelayDuration.Round(time.Second), sr.Elapsed.Round(time.Second), sr.Hostname)
}

func (sr ShutdownResponse) Fields() map[string]interface{} {
	fields := map[string]interface{}{}
	fields["shutdown"] = sr.ShutdownInProgress
	fields["shutdown-delay-duration"] = sr.ShutdownDelayDuration.Round(time.Second)
	fields["shutdown-elapsed"] = sr.Elapsed.Round(time.Second)
	fields["host"] = sr.Hostname

	return fields
}
