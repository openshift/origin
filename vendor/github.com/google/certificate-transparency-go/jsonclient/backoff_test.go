// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jsonclient

import (
	"math"
	"testing"
	"time"
)

const testLeeway = 25 * time.Microsecond

func fuzzyTimeEquals(a, b time.Time, leeway time.Duration) bool {
	diff := math.Abs(float64(a.Sub(b).Nanoseconds()))
	if diff < float64(leeway.Nanoseconds()) {
		return true
	}
	return false
}

func fuzzyDurationEquals(a, b time.Duration, leeway time.Duration) bool {
	diff := math.Abs(float64(a.Nanoseconds() - b.Nanoseconds()))
	if diff < float64(leeway.Nanoseconds()) {
		return true
	}
	return false
}

func TestBackoff(t *testing.T) {
	b := backoff{}

	// Test that the interval increases as expected
	for i := uint(0); i < maxMultiplier; i++ {
		n := time.Now()
		interval := b.set(nil)
		if interval != time.Second*(1<<i) {
			t.Fatalf("backoff.set(nil)=%v; want %v", interval, time.Second*(1<<i))
		}
		expected := n.Add(interval)
		until := b.until()
		if !fuzzyTimeEquals(expected, until, time.Millisecond) {
			t.Fatalf("backoff.until()=%v; want %v (+ 0-250ms)", expected, until)
		}

		// reset notBefore
		b.notBefore = time.Time{}
	}

	// Test that multiplier doesn't go above maxMultiplier
	b.multiplier = maxMultiplier
	b.notBefore = time.Time{}
	interval := b.set(nil)
	if b.multiplier > maxMultiplier {
		t.Fatalf("backoff.multiplier=%v; want %v", b.multiplier, maxMultiplier)
	}
	if interval > time.Second*(1<<(maxMultiplier-1)) {
		t.Fatalf("backoff.set(nil)=%v; want %v", interval, 1<<(maxMultiplier-1)*time.Second)
	}

	// Test decreaseMultiplier properly decreases the multiplier
	b.multiplier = 1
	b.notBefore = time.Time{}
	b.decreaseMultiplier()
	if b.multiplier != 0 {
		t.Fatalf("backoff.multiplier=%v; want %v", b.multiplier, 0)
	}

	// Test decreaseMultiplier doesn't reduce multiplier below 0
	b.decreaseMultiplier()
	if b.multiplier != 0 {
		t.Fatalf("backoff.multiplier=%v; want %v", b.multiplier, 0)
	}
}

func TestBackoffOverride(t *testing.T) {
	b := backoff{}
	for _, tc := range []struct {
		notBefore        time.Time
		override         time.Duration
		expectedInterval time.Duration
	}{
		{
			notBefore:        time.Now().Add(time.Hour),
			override:         time.Second * 1800,
			expectedInterval: time.Hour,
		},
		{
			notBefore:        time.Now().Add(time.Hour),
			override:         time.Second * 7200,
			expectedInterval: 2 * time.Hour,
		},
		{
			notBefore:        time.Time{},
			override:         time.Second * 7200,
			expectedInterval: 2 * time.Hour,
		},
	} {
		b.multiplier = 0
		b.notBefore = tc.notBefore
		interval := b.set(&tc.override)
		if !fuzzyDurationEquals(tc.expectedInterval, interval, testLeeway) {
			t.Fatalf("backoff.set(%v)=%v; want %v", tc.override, interval, tc.expectedInterval)
		}
	}
}
