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

package util

import (
	"sync"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"
	"google.golang.org/grpc/naming"
)

func TestFixedBackendsResolver(t *testing.T) {
	var tests = []struct {
		target  string
		wantErr bool
		want    *fixedBackends
	}{
		{target: "", wantErr: true},
		{target: "a,b,c", want: newFixedBackends([]string{"a", "b", "c"})},
		{target: "alongnamewithaport:111", want: newFixedBackends([]string{"alongnamewithaport:111"})},
	}
	for _, test := range tests {
		fbr := FixedBackendResolver{}
		got, err := fbr.Resolve(test.target)
		if gotErr := (err != nil); gotErr != test.wantErr {
			t.Errorf("FixedBackendResolver.Resolve(%q)=(%v,%v); want (_,err=%v)", test.target, got, err, test.wantErr)
		}
		if err != nil {
			continue
		}
		if diff := pretty.Compare(got, test.want); diff != "" {
			t.Errorf("FixedBackendResolver.Resolve(%q)=%v; want %v, diff -got +want:\n%v", test.target, got, test.want, diff)
		}
	}
}

func TestFixedBackends(t *testing.T) {
	var tests = []struct {
		fb   *fixedBackends
		want []naming.Update
	}{
		{
			fb: newFixedBackends([]string{"a", "b", "c"}),
			want: []naming.Update{
				{Op: naming.Add, Addr: "a"},
				{Op: naming.Add, Addr: "b"},
				{Op: naming.Add, Addr: "c"},
			},
		},
		{
			fb: newFixedBackends([]string{"alongnamewithaport:111"}),
			want: []naming.Update{
				{Op: naming.Add, Addr: "alongnamewithaport:111"},
			},
		},
	}
	for _, test := range tests {
		got, err := test.fb.Next()
		if err != nil {
			t.Errorf("%+v.Next(#1)=(%v,%v); want (_,nil)", test.fb, got, err)
			continue
		}
		if diff := pretty.Compare(got, test.want); diff != "" {
			t.Errorf("%+v.Next(#1)=%v; want %v, diff -got +want:\n%v", test.fb, got, test.want, diff)
		}

		// Second call to Next() should block until Close()
		done := false
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			got, err := test.fb.Next()
			if got != nil || err == nil {
				t.Errorf("%+v.Next(#1)=(%v,%v); want (nil,err)", test.fb, got, err)
			}
			done = true
			wg.Done()
		}()
		time.Sleep(10 * time.Millisecond)
		if done {
			t.Errorf("%+v.Next(#2) completed; want blocked", test.fb)
		}
		test.fb.Close()
		wg.Wait()
		if !done {
			t.Errorf("%+v.Next(#2) blocked; want completed", test.fb)
		}
	}
}
