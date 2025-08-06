// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package progress

type scaledReport struct {
	Report
	n int
	i int
}

func (r scaledReport) Percentage() float32 {
	b := 100 * float32(r.i) / float32(r.n)
	return b + (r.Report.Percentage() / float32(r.n))
}

type scaleOne struct {
	s Sinker
	n int
	i int
}

func (s scaleOne) Sink() chan<- Report {
	upstream := make(chan Report)
	downstream := s.s.Sink()
	go s.loop(upstream, downstream)
	return upstream
}

func (s scaleOne) loop(upstream <-chan Report, downstream chan<- Report) {
	defer close(downstream)

	for r := range upstream {
		downstream <- scaledReport{
			Report: r,
			n:      s.n,
			i:      s.i,
		}
	}
}

type scaleMany struct {
	s Sinker
	n int
	i int
}

func Scale(s Sinker, n int) Sinker {
	return &scaleMany{
		s: s,
		n: n,
	}
}

func (s *scaleMany) Sink() chan<- Report {
	if s.i == s.n {
		s.n++
	}

	ch := scaleOne{s: s.s, n: s.n, i: s.i}.Sink()
	s.i++
	return ch
}
