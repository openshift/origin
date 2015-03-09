// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

// Counter is the metric interface used by this package
type Counter interface {
	Inc(i int64)
}

type nopCounter struct{}

func (nopCounter) Inc(_ int64) {}

// These are the stat variables defined by this package.
var (
	StatsForwardCount    Counter = nopCounter{}
	StatsLookupCount     Counter = nopCounter{}
	StatsRequestCount    Counter = nopCounter{}
	StatsDnssecOkCount   Counter = nopCounter{}
	StatsDnssecCacheMiss Counter = nopCounter{}
	StatsNameErrorCount  Counter = nopCounter{}
	StatsNoDataCount     Counter = nopCounter{}
)
