//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package utils

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/heketi/tests"
)

func TestNewStatusGroup(t *testing.T) {
	s := NewStatusGroup()
	tests.Assert(t, s != nil)
	tests.Assert(t, s.results != nil)
	tests.Assert(t, len(s.results) == 0)
	tests.Assert(t, s.err == nil)
}

func TestStatusGroupSuccess(t *testing.T) {

	s := NewStatusGroup()

	max := 100
	s.Add(max)

	for i := 0; i < max; i++ {
		go func(value int) {
			defer s.Done()
			time.Sleep(time.Millisecond * 1 * time.Duration(value))
		}(i)
	}

	err := s.Result()
	tests.Assert(t, err == nil)

}

func TestStatusGroupFailure(t *testing.T) {
	s := NewStatusGroup()

	for i := 0; i < 100; i++ {

		s.Add(1)
		go func(value int) {
			defer s.Done()
			time.Sleep(time.Millisecond * 1 * time.Duration(value))
			if value%10 == 0 {
				s.Err(errors.New(fmt.Sprintf("Err: %v", value)))
			}

		}(i)

	}

	err := s.Result()

	tests.Assert(t, err != nil)
	tests.Assert(t, err.Error() == "Err: 90", err)

}
