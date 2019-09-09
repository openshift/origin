//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/idgen"
)

func TestOpTrackerCounts(t *testing.T) {
	ot := newOpTracker(50)
	var (
		i1 = idgen.GenUUID()
		i2 = idgen.GenUUID()
		i3 = idgen.GenUUID()
		i4 = idgen.GenUUID()
		i5 = idgen.GenUUID()
	)

	ot.Add(i1, TrackNormal)
	ot.Add(i2, TrackNormal)
	ot.Add(i3, TrackNormal)
	tests.Assert(t, ot.Get() == 3, "expected ot.Get() == 3, got", ot.Get())

	ot.Add(i4, TrackNormal)
	ot.Add(i5, TrackNormal)
	ot.Remove(i1)
	ot.Add(i1, TrackNormal)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
}

func TestOpTrackerLimits(t *testing.T) {
	ot := newOpTracker(5)
	var (
		i1 = idgen.GenUUID()
		i2 = idgen.GenUUID()
		i3 = idgen.GenUUID()
		i4 = idgen.GenUUID()
	)

	ot.Add(i1, TrackNormal)
	ot.Add(i2, TrackNormal)
	ot.Add(i3, TrackNormal)
	ot.Add(i4, TrackNormal)

	var (
		r      bool
		token  string
		token2 string
	)
	r, token = ot.ThrottleOrToken()
	tests.Assert(t, r == false, "expected r == false, got", r)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
	tests.Assert(t, token != "", "expected token != \"\"")

	r, token2 = ot.ThrottleOrToken()
	tests.Assert(t, r == true, "expected r == true, got", r)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
	tests.Assert(t, token2 == "", "expected token2 == \"\"")

	ot.Remove(token)

	r, token = ot.ThrottleOrToken()
	tests.Assert(t, r == false, "expected r == false, got", r)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
	tests.Assert(t, token != "", "expected token != \"\"")

	r, token2 = ot.ThrottleOrToken()
	tests.Assert(t, r == true, "expected r == true, got", r)
	tests.Assert(t, ot.Get() == 5, "expected ot.Get() == 5, got", ot.Get())
	tests.Assert(t, token2 == "", "expected token2 == \"\"")
}

func TestOpTrackerLimitBgOps(t *testing.T) {
	ot := newOpTracker(5)
	var (
		b1 = idgen.GenUUID()
		b2 = idgen.GenUUID()
	)

	throttled := ot.ThrottleOrAdd(b1, TrackClean)
	tests.Assert(t, !throttled)
	tests.Assert(t, ot.Get() == 1)

	throttled = ot.ThrottleOrAdd(b2, TrackClean)
	tests.Assert(t, throttled)
	tests.Assert(t, ot.Get() == 1)
}

func TestOpTrackerLimitBgNormalOps(t *testing.T) {
	ot := newOpTracker(5)
	var (
		b1 = idgen.GenUUID()
		b2 = idgen.GenUUID()
	)

	throttled := ot.ThrottleOrAdd(b1, TrackClean)
	tests.Assert(t, !throttled)
	tests.Assert(t, ot.Get() == 1)

	// cannot add a 2nd bg op
	throttled = ot.ThrottleOrAdd(b2, TrackClean)
	tests.Assert(t, throttled)
	tests.Assert(t, ot.Get() == 1)

	// add as many normal ops as it will take
	for i := 0; i < 5; i++ {
		throttled := ot.ThrottleOrAdd(idgen.GenUUID(), TrackNormal)
		tests.Assert(t, !throttled, "expected not trottled on iteration", i)
		tests.Assert(t, int(ot.Get()) == 2+i,
			"expected int(ot.Get()) == 1 + i, got:", ot.Get(), 2+i)
	}

	// normal op is blocked by throttle
	throttled = ot.ThrottleOrAdd(idgen.GenUUID(), TrackNormal)
	tests.Assert(t, throttled, "expected throttled")
	tests.Assert(t, ot.Get() == 6)

	// remove existing bg op
	ot.Remove(b1)
	tests.Assert(t, ot.Get() == 5)

	// still cannot add bg op, now due to max num normal ops
	throttled = ot.ThrottleOrAdd(b2, TrackClean)
	tests.Assert(t, throttled)
	tests.Assert(t, ot.Get() == 5)
}

func TestOpTrackerGetTracked(t *testing.T) {
	ot := newOpTracker(5)
	var (
		i1 = idgen.GenUUID()
		i2 = idgen.GenUUID()
		b1 = idgen.GenUUID()
	)

	throttled := ot.ThrottleOrAdd(i1, TrackNormal)
	tests.Assert(t, !throttled)

	throttled = ot.ThrottleOrAdd(i2, TrackNormal)
	tests.Assert(t, !throttled)

	throttled = ot.ThrottleOrAdd(b1, TrackClean)
	tests.Assert(t, !throttled)

	r := ot.Tracked()
	tests.Assert(t, len(r) == 3)
	tests.Assert(t, r[i1])
	tests.Assert(t, r[i2])
	tests.Assert(t, r[b1])
}

func TestOpTrackerAssertions(t *testing.T) {
	ot := newOpTracker(5)
	var (
		i1 = idgen.GenUUID()
		i2 = idgen.GenUUID()
		b1 = idgen.GenUUID()
	)

	checkPanic := func() {
		r := recover()
		tests.Assert(t, r != nil, "expected r != nil, got:", r)
	}
	t.Run("remove missing i1", func(t *testing.T) {
		defer checkPanic()
		ot.Remove(i1)
		t.Fatalf("Test should not reach this line")
	})
	t.Run("remove missing i2", func(t *testing.T) {
		defer checkPanic()
		ot.Remove(i2)
		t.Fatalf("Test should not reach this line")
	})
	t.Run("remove missing b1", func(t *testing.T) {
		defer checkPanic()
		ot.Remove(b1)
		t.Fatalf("Test should not reach this line")
	})
	t.Run("double add i1", func(t *testing.T) {
		defer checkPanic()
		ot.Add(i1, TrackNormal)
		ot.Add(i1, TrackNormal)
		t.Fatalf("Test should not reach this line")
	})
	t.Run("double add b1", func(t *testing.T) {
		defer checkPanic()
		ot.Add(b1, TrackClean)
		ot.Add(b1, TrackClean)
		t.Fatalf("Test should not reach this line")
	})
	t.Run("double add i2 type swap", func(t *testing.T) {
		defer checkPanic()
		ot.Add(i2, TrackNormal)
		ot.Add(i2, TrackClean)
		t.Fatalf("Test should not reach this line")
	})
	tests.Assert(t, ot.Get() == 3)
}
