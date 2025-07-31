// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package progress

// Tee works like Unix tee; it forwards all progress reports it receives to the
// specified sinks
func Tee(s1, s2 Sinker) Sinker {
	fn := func() chan<- Report {
		d1 := s1.Sink()
		d2 := s2.Sink()
		u := make(chan Report)
		go tee(u, d1, d2)
		return u
	}

	return SinkFunc(fn)
}

func tee(u <-chan Report, d1, d2 chan<- Report) {
	defer close(d1)
	defer close(d2)

	for r := range u {
		d1 <- r
		d2 <- r
	}
}
