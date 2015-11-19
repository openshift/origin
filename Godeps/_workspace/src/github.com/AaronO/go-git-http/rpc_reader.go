package githttp

import (
	"io"
	"regexp"
)

// RpcReader scans for events in the incoming rpc request data
type RpcReader struct {
	// Underlaying reader (to relay calls to)
	io.ReadCloser

	// Rpc type (upload-pack or receive-pack)
	Rpc string

	// List of events RpcReader has picked up through scanning
	// these events do not contain the "Dir" attribute
	Events []Event

	// Tracks first event being scanned
	first bool
}

// Regexes to detect types of actions (fetch, push, etc ...)
var (
	receivePackRegex = regexp.MustCompile("([0-9a-fA-F]{40}) ([0-9a-fA-F]{40}) refs\\/(heads|tags)\\/(.*?)( |00|\u0000)|^(0000)$")
	uploadPackRegex  = regexp.MustCompile("^\\S+ ([0-9a-fA-F]{40})")
)

// Implement the io.Reader interface
func (r *RpcReader) Read(p []byte) (n int, err error) {
	// Relay call
	n, err = r.ReadCloser.Read(p)

	// Scan for events
	if err != nil {
		r.scan(p)
	}

	return n, err
}

func (r *RpcReader) scan(p []byte) {
	events := []Event{}

	switch r.Rpc {
	case "receive-pack":
		events = scanPush(p)
		if !r.first && len(events) == 0 {
			events = scanPushForce(p)
			r.first = true
		}
	case "upload-pack":
		events = scanFetch(p)
	}

	// Add new events
	if len(events) > 0 {
		r.Events = append(r.Events, events...)
	}
}

func scanFetch(data []byte) []Event {
	matches := uploadPackRegex.FindAllStringSubmatch(string(data[:]), -1)

	if matches == nil {
		return []Event{}
	}

	events := []Event{}
	for _, m := range matches {
		events = append(events, Event{
			Type:   FETCH,
			Commit: m[1],
		})
	}

	return events
}

func scanPush(data []byte) []Event {
	matches := receivePackRegex.FindAllStringSubmatch(string(data[:]), -1)

	if matches == nil {
		return []Event{}
	}

	events := []Event{}
	for _, m := range matches {
		e := Event{
			Last:   m[1],
			Commit: m[2],
		}

		// Handle pushes to branches and tags differently
		if m[3] == "heads" {
			e.Type = PUSH
			e.Branch = m[4]
		} else {
			e.Type = TAG
			e.Tag = m[4]
		}

		events = append(events, e)
	}

	return events
}

func scanPushForce(data []byte) []Event {
	return []Event{
		Event{
			Type:   PUSH_FORCE,
			Commit: "HEAD",
		},
	}
}
