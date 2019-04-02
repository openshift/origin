//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package idgen

import (
	"strings"
	"testing"

	"github.com/heketi/tests"
)

// test actual generation of a uuid, we can only test the length
// of the output as we are relying on an actual random source here
func TestGenUUID(t *testing.T) {
	uuid := GenUUID()
	tests.Assert(t, len(uuid) == 32, "bad length", len(uuid), 32)
}

// test actual output by specifying our own source of "randomness"
func TestFakeUUID(t *testing.T) {
	r := strings.NewReader("heketiheketiheketi")
	uuid := IdSource{r}.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "bad length", len(uuid), 32)
	tests.Assert(t, uuid == "68656b65746968656b65746968656b65")
}

func TestNonRandomUUID(t *testing.T) {
	s := IdSource{&NonRandom{}}

	uuid := s.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000000", "got:", uuid)
	uuid = s.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000001", "got:", uuid)
	uuid = s.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000002", "got:", uuid)

	for i := 0; i < 106; i++ {
		s.ReadUUID()
	}
	uuid = s.ReadUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "0000000000000000000000000000006d", "got:", uuid)
}

func TestReplaceRandomness(t *testing.T) {
	before := Randomness

	n := &NonRandom{}
	Randomness = n
	defer func() {
		Randomness = before
	}()

	// now we're using a non-random source. we should have predictable values
	uuid := GenUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000000", "got:", uuid)

	uuid = GenUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid == "00000000000000000000000000000001", "got:", uuid)

	// restore the random source back to what it was before
	Randomness = before

	// uuid should NOT be the next non-random number
	uuid = GenUUID()
	tests.Assert(t, len(uuid) == 32, "expected len(uuid) == 32, got:", len(uuid))
	tests.Assert(t, uuid != "00000000000000000000000000000002", "got:", uuid)
}

// NOTE: the Original GenUUID function aborts the applicaion
// when conditions are not met. This was carried over into the
// version with selectable random sources so we dont actually
// do any of that testing here or the unit test runner would abort
