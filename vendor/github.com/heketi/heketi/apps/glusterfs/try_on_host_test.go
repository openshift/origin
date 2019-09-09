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
	"fmt"
	"testing"

	"github.com/heketi/tests"
)

func TestTryOnHosts(t *testing.T) {
	hosts := nodeHosts{
		"foo": "123",
		"bar": "456",
		"baz": "789",
	}

	t.Run("default", func(t *testing.T) {
		checked := map[string]bool{}
		err := newTryOnHosts(hosts).run(func(h string) error {
			checked[h] = true
			return nil
		})
		tests.Assert(t, err == nil)
		tests.Assert(t, len(checked) == 1)
	})
	t.Run("fail", func(t *testing.T) {
		checked := map[string]bool{}
		err := newTryOnHosts(hosts).run(func(h string) error {
			checked[h] = true
			return fmt.Errorf("boop")
		})
		tests.Assert(t, err != nil)
		tests.Assert(t, len(checked) == 3)
	})
	t.Run("customDone", func(t *testing.T) {
		e1 := fmt.Errorf("blarg")
		e2 := fmt.Errorf("blat")
		checked := map[string]bool{}
		toh := newTryOnHosts(hosts)
		toh.done = func(e error) bool {
			return e == e1
		}
		err := toh.run(func(h string) error {
			checked[h] = true
			if len(checked) >= 3 {
				return e1
			}
			return e2
		})
		tests.Assert(t, err != nil)
		tests.Assert(t, err == e1)
		tests.Assert(t, len(checked) == 3)
	})
	t.Run("customDoneEarly", func(t *testing.T) {
		e1 := fmt.Errorf("blarg")
		checked := map[string]bool{}
		toh := newTryOnHosts(hosts)
		toh.done = func(e error) bool {
			return e == e1
		}
		err := toh.run(func(h string) error {
			checked[h] = true
			return e1
		})
		tests.Assert(t, err != nil)
		tests.Assert(t, err == e1)
		tests.Assert(t, len(checked) == 1)
	})
	t.Run("oneUp", func(t *testing.T) {
		uppity := map[string]bool{
			"foo": true,
			"bar": false,
			"baz": false,
		}
		checked := map[string]bool{}
		toh := newTryOnHosts(hosts)
		toh.nodesUp = func() map[string]bool {
			return uppity
		}
		err := toh.run(func(h string) error {
			checked[h] = true
			return nil
		})
		tests.Assert(t, err == nil)
		tests.Assert(t, len(checked) == 1)
		tests.Assert(t, checked["123"], "got", checked)
	})
	t.Run("allDown", func(t *testing.T) {
		uppity := map[string]bool{
			"foo": false,
			"bar": false,
			"baz": false,
		}
		checked := map[string]bool{}
		toh := newTryOnHosts(hosts)
		toh.nodesUp = func() map[string]bool {
			return uppity
		}
		err := toh.run(func(h string) error {
			checked[h] = true
			return nil
		})
		tests.Assert(t, err != nil)
		tests.Assert(t, len(checked) == 0)
	})
	t.Run("missingTreatedAsUp", func(t *testing.T) {
		uppity := map[string]bool{
			"foo": false,
			"baz": false,
		}
		checked := map[string]bool{}
		toh := newTryOnHosts(hosts)
		toh.nodesUp = func() map[string]bool {
			return uppity
		}
		err := toh.run(func(h string) error {
			checked[h] = true
			return nil
		})
		tests.Assert(t, err == nil)
		tests.Assert(t, len(checked) == 1)
		tests.Assert(t, checked["456"], "got", checked)
	})
	t.Run("once", func(t *testing.T) {
		checked := map[string]bool{}
		err := newTryOnHosts(hosts).once().run(func(h string) error {
			checked[h] = true
			return nil
		})
		tests.Assert(t, err == nil)
		tests.Assert(t, len(checked) == 1)
	})
	t.Run("onceOneUp", func(t *testing.T) {
		uppity := map[string]bool{
			"foo": true,
			"bar": false,
			"baz": false,
		}
		checked := map[string]bool{}
		toh := newTryOnHosts(hosts)
		toh.nodesUp = func() map[string]bool {
			return uppity
		}
		err := toh.once().run(func(h string) error {
			checked[h] = true
			return nil
		})
		tests.Assert(t, err == nil)
		tests.Assert(t, len(checked) == 1)
		tests.Assert(t, checked["123"], "got", checked)
	})
	t.Run("onceError", func(t *testing.T) {
		checked := map[string]bool{}
		err := newTryOnHosts(hosts).once().run(func(h string) error {
			checked[h] = true
			return fmt.Errorf("boop")
		})
		tests.Assert(t, err != nil)
		tests.Assert(t, len(checked) == 1)
	})
}
