// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package progress

import "fmt"

type prefixedReport struct {
	Report
	prefix string
}

func (r prefixedReport) Detail() string {
	if d := r.Report.Detail(); d != "" {
		return fmt.Sprintf("%s: %s", r.prefix, d)
	}

	return r.prefix
}

func prefixLoop(upstream <-chan Report, downstream chan<- Report, prefix string) {
	defer close(downstream)

	for r := range upstream {
		downstream <- prefixedReport{
			Report: r,
			prefix: prefix,
		}
	}
}

func Prefix(s Sinker, prefix string) Sinker {
	fn := func() chan<- Report {
		upstream := make(chan Report)
		downstream := s.Sink()
		go prefixLoop(upstream, downstream, prefix)
		return upstream
	}

	return SinkFunc(fn)
}
