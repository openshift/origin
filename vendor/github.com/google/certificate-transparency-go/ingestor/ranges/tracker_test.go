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

package ranges

import (
	"testing"
	"time"
)

func TestEmpty(t *testing.T) {
	tr, err := NewTracker(0, 100)
	if err != nil {
		t.Errorf("NewTracker(%d, %d) err=%v, want nil", 0, 100, err)
	}

	if tr.IsComplete() {
		t.Error("IsComplete()? true, want false")
	}

	if tr.IsPartiallyComplete() {
		t.Error("IsPartiallyComplete()? true, want false")
	}
}

func TestAddingSubRanges(t *testing.T) {
	type toAdd struct {
		first, last           int64
		wantErr               bool
		wantComplete          bool
		wantPartiallyComplete bool
	}

	tests := []struct {
		desc                  string
		start, end            int64
		additions             []toAdd
		wantErr               bool
		wantString, wantDebug string
	}{
		{
			desc:       "Valid input (start == end) but no sub-ranges added.",
			wantString: "<expected range [start 0, end 0] watermarks <lo -1, mid -1, hi -1> #subranges 0 0% complete>",
			wantDebug:  "<expected range [start 0, end 0] watermarks <lo -1, mid -1, hi -1> subranges [] 0% complete>",
		},
		{
			desc:       "Valid input (start < end) but no sub-ranges added.",
			end:        1,
			wantString: "<expected range [start 0, end 1] watermarks <lo -1, mid -1, hi -1> #subranges 0 0% complete>",
			wantDebug:  "<expected range [start 0, end 1] watermarks <lo -1, mid -1, hi -1> subranges [] 0% complete>",
		},
		{
			desc:    "Invalid input (start < 0)",
			start:   -1,
			wantErr: true,
		},
		{
			desc:    "Invalid input (end < start)",
			start:   1,
			wantErr: true,
		},
		{
			desc:       "Invalid subranges - last < first",
			end:        100,
			additions:  []toAdd{{10, 0, true, false, false}},
			wantString: "<expected range [start 0, end 100] watermarks <lo -1, mid -1, hi -1> #subranges 0 0% complete>",
			wantDebug:  "<expected range [start 0, end 100] watermarks <lo -1, mid -1, hi -1> subranges [] 0% complete>",
		},
		{
			desc:       "Invalid subranges - last > end",
			end:        100,
			additions:  []toAdd{{0, 105, true, false, false}},
			wantString: "<expected range [start 0, end 100] watermarks <lo -1, mid -1, hi -1> #subranges 0 0% complete>",
			wantDebug:  "<expected range [start 0, end 100] watermarks <lo -1, mid -1, hi -1> subranges [] 0% complete>",
		},
		{
			desc:       "Invalid subranges - first < start",
			start:      10,
			end:        90,
			additions:  []toAdd{{9, 10, true, false, false}},
			wantString: "<expected range [start 10, end 90] watermarks <lo -1, mid -1, hi -1> #subranges 0 0% complete>",
			wantDebug:  "<expected range [start 10, end 90] watermarks <lo -1, mid -1, hi -1> subranges [] 0% complete>",
		},
		{
			desc: "Invalid subranges - 2nd overlaps 1st",
			end:  100,
			additions: []toAdd{
				{5, 10, false, false, false},
				{8, 15, true, false, false},
			},
			wantString: "<expected range [start 0, end 100] watermarks <lo 5, mid -1, hi 10> #subranges 1 0% complete>",
			wantDebug:  "<expected range [start 0, end 100] watermarks <lo 5, mid -1, hi 10> subranges [<first 5, last 10>] 0% complete>",
		},
		{
			desc:       "Single subrange added located at range start, so we are partially complete.",
			end:        100,
			additions:  []toAdd{{0, 19, false, false, true}},
			wantString: "<expected range [start 0, end 100] watermarks <lo 0, mid 19, hi 19> #subranges 1 20.0% complete, done in 4s>",
			wantDebug:  "<expected range [start 0, end 100] watermarks <lo 0, mid 19, hi 19> subranges [<first 0, last 19>] 20.0% complete, done in 4s>",
		},
		{
			desc: "Two subranges added, partially complete after the second.",
			end:  100,
			additions: []toAdd{
				{10, 19, false, false, false},
				{0, 9, false, false, true},
			},
			wantString: "<expected range [start 0, end 100] watermarks <lo 0, mid 19, hi 19> #subranges 2 20.0% complete, done in 8s>",
			wantDebug:  "<expected range [start 0, end 100] watermarks <lo 0, mid 19, hi 19> subranges [<first 0, last 9> <first 10, last 19>] 20.0% complete, done in 8s>",
		},
		{
			desc: "Four subranges added in fully reverse order, not partially complete until end when it is fully complete.",
			end:  100,
			additions: []toAdd{
				{75, 100, false, false, false},
				{50, 74, false, false, false},
				{25, 49, false, false, false},
				{0, 24, false, true, true},
			},
			wantString: "<expected range [start 0, end 100] watermarks <lo 0, mid 100, hi 100> #subranges 4 100% complete>",
			wantDebug:  "<expected range [start 0, end 100] watermarks <lo 0, mid 100, hi 100> subranges [<first 0, last 24> <first 25, last 49> <first 50, last 74> <first 75, last 100>] 100% complete>",
		},
		{
			desc:       "Single subrange the size of the full range, complete straight away.",
			end:        100,
			additions:  []toAdd{{0, 100, false, true, true}},
			wantString: "<expected range [start 0, end 100] watermarks <lo 0, mid 100, hi 100> #subranges 1 100% complete>",
			wantDebug:  "<expected range [start 0, end 100] watermarks <lo 0, mid 100, hi 100> subranges [<first 0, last 100>] 100% complete>",
		},
		{
			desc: "Lots of subranges of size 1. Partially complete straight away; complete by the end.",
			end:  4,
			additions: []toAdd{
				{0, 0, false, false, true},
				{1, 1, false, false, true},
				{2, 2, false, false, true},
				{3, 3, false, false, true},
				{4, 4, false, true, true},
			},
			wantString: "<expected range [start 0, end 4] watermarks <lo 0, mid 4, hi 4> #subranges 5 100% complete>",
			wantDebug:  "<expected range [start 0, end 4] watermarks <lo 0, mid 4, hi 4> subranges [<first 0, last 0> <first 1, last 1> <first 2, last 2> <first 3, last 3> <first 4, last 4>] 100% complete>",
		},
	}

	for _, test := range tests {
		now := time.Unix(0, 0)
		timeNow = func() time.Time { return now }

		tr, err := NewTracker(test.start, test.end)
		if gotErr := err != nil; gotErr != test.wantErr {
			t.Errorf("%s: NewTracker(%d, %d): got err? %t, want? %t (err %v)", test.desc, test.start, test.end, gotErr, test.wantErr, err)
		}
		if err != nil {
			continue
		}

		for i, a := range test.additions {
			now = now.Add(time.Second)

			err := tr.AddSubRange(a.first, a.last)
			if gotErr := (err != nil); gotErr != a.wantErr {
				t.Errorf("%s: %d: AddSubRange(%d, %d): got err? %t, want? %t (err %v)", test.desc, i, a.first, a.last, gotErr, a.wantErr, err)
			}

			if got := tr.IsComplete(); got != a.wantComplete {
				t.Errorf("%s: %d: IsComplete()? %t, want? %t (Tracker state %v)", test.desc, i, got, a.wantComplete, tr)
			}

			if got := tr.IsPartiallyComplete(); got != a.wantPartiallyComplete {
				t.Errorf("%s: %d: IsPartiallyComplete()? %t, want? %t (Tracker state %v)", test.desc, i, got, a.wantPartiallyComplete, tr)
			}
		}

		if got, want := tr.String(), test.wantString; got != want {
			t.Errorf("%s: String(): got %q, want %q", test.desc, got, want)
		}

		if got, want := tr.DebugString(), test.wantDebug; got != want {
			t.Errorf("%s: DebugString(): got %q, want %q", test.desc, got, want)
		}
	}
}
