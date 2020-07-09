// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestStateString(t *testing.T) {
	t.Parallel()
	started := time.Now().Add(-3 * time.Hour)
	tests := []struct {
		name     string
		input    State
		expected string
	}{
		{"paused", State{Running: true, Paused: true, StartedAt: started}, "Up 3 hours (Paused)"},
		{"restarting", State{Running: true, Restarting: true, ExitCode: 7, FinishedAt: started}, "Restarting (7) 3 hours ago"},
		{"up", State{Running: true, StartedAt: started}, "Up 3 hours"},
		{"being removed", State{RemovalInProgress: true}, "Removal In Progress"},
		{"dead", State{Dead: true}, "Dead"},
		{"created", State{}, "Created"},
		{"no creation info", State{StartedAt: started}, ""},
		{"erro code", State{ExitCode: 7, StartedAt: started, FinishedAt: started}, "Exited (7) 3 hours ago"},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.input.String(); got != test.expected {
				t.Errorf("State.String(): wrong result. Want %q. Got %q.", test.expected, got)
			}
		})
	}
}

func TestStateStateString(t *testing.T) {
	t.Parallel()
	started := time.Now().Add(-3 * time.Hour)
	tests := []struct {
		input    State
		expected string
	}{
		{State{Running: true, Paused: true}, "paused"},
		{State{Running: true, Restarting: true}, "restarting"},
		{State{Running: true}, "running"},
		{State{Dead: true}, "dead"},
		{State{}, "created"},
		{State{StartedAt: started}, "exited"},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.expected, func(t *testing.T) {
			t.Parallel()
			if got := test.input.StateString(); got != test.expected {
				t.Errorf("State.String(): wrong result. Want %q. Got %q.", test.expected, got)
			}
		})
	}
}

// sleepyRoundTripper implements the http.RoundTripper interface. It sleeps
// for the 'sleep' duration and then returns an error for RoundTrip method.
type sleepyRoudTripper struct {
	sleepDuration time.Duration
}

func (rt *sleepyRoudTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	time.Sleep(rt.sleepDuration)
	return nil, errors.New("Can't complete round trip")
}

func TestNoSuchContainerError(t *testing.T) {
	t.Parallel()
	err := &NoSuchContainer{ID: "i345"}
	expected := "No such container: i345"
	if got := err.Error(); got != expected {
		t.Errorf("NoSuchContainer: wrong message. Want %q. Got %q.", expected, got)
	}
}

func TestNoSuchContainerErrorMessage(t *testing.T) {
	t.Parallel()
	err := &NoSuchContainer{ID: "i345", Err: errors.New("some advanced error info")}
	expected := "some advanced error info"
	if got := err.Error(); got != expected {
		t.Errorf("NoSuchContainer: wrong message. Want %q. Got %q.", expected, got)
	}
}
