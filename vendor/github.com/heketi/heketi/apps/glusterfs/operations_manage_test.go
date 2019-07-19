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
)

func TestOpCounterCounts(t *testing.T) {
	oc := &OpCounter{Limit: 50}

	oc.Inc()
	oc.Inc()
	oc.Inc()
	tests.Assert(t, oc.Get() == 3, "expected oc.Get() == 3, got", oc.Get())

	oc.Inc()
	oc.Inc()
	oc.Dec()
	oc.Inc()
	tests.Assert(t, oc.Get() == 5, "expected oc.Get() == 5, got", oc.Get())
}

func TestOpCounterLimits(t *testing.T) {
	oc := &OpCounter{Limit: 5}

	oc.Inc()
	oc.Inc()
	oc.Inc()
	oc.Inc()

	var r bool
	r = oc.ThrottleOrInc()
	tests.Assert(t, r == false, "expected r == false, got", r)
	tests.Assert(t, oc.Get() == 5, "expected oc.Get() == 5, got", oc.Get())

	r = oc.ThrottleOrInc()
	tests.Assert(t, r == true, "expected r == true, got", r)
	tests.Assert(t, oc.Get() == 5, "expected oc.Get() == 5, got", oc.Get())

	oc.Dec()

	r = oc.ThrottleOrInc()
	tests.Assert(t, r == false, "expected r == false, got", r)
	tests.Assert(t, oc.Get() == 5, "expected oc.Get() == 5, got", oc.Get())

	r = oc.ThrottleOrInc()
	tests.Assert(t, r == true, "expected r == true, got", r)
	tests.Assert(t, oc.Get() == 5, "expected oc.Get() == 5, got", oc.Get())
}
