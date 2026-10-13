// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package progress

// Sinker defines what is expected of a type that can act as a sink for
// progress reports. The semantics are as follows. If you call Sink(), you are
// responsible for closing the returned channel. Closing this channel means
// that the related task is done, or resulted in error.
type Sinker interface {
	Sink() chan<- Report
}

// SinkFunc defines a function that returns a progress report channel.
type SinkFunc func() chan<- Report

// Sink makes the SinkFunc implement the Sinker interface.
func (fn SinkFunc) Sink() chan<- Report {
	return fn()
}
