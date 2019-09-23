//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package remoteexec

import (
	"fmt"
	"testing"

	"github.com/heketi/tests"
)

func TestResultOk(t *testing.T) {
	var r Result

	// a command that ran successfully
	r = Result{
		Completed:  true,
		Output:     "Foo",
		ExitStatus: 0,
	}
	tests.Assert(t, r.Ok(), "expected r.Ok() to be true")

	// an incomplete result
	r = Result{}
	tests.Assert(t, !r.Ok(), "expected r.Ok() to be false")

	// a command that ran & completed with error
	r = Result{
		Completed:  true,
		ErrOutput:  "Bar bar bar",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	}
	tests.Assert(t, !r.Ok(), "expected r.Ok() to be false")

	// only error set. possibly indicating a conn error
	r = Result{
		Completed: true,
		Err:       fmt.Errorf("something broke"),
	}
	tests.Assert(t, !r.Ok(), "expected r.Ok() to be false")
}

func TestResultError(t *testing.T) {
	var r Result

	// a command that ran successfully
	r = Result{
		Completed:  true,
		Output:     "Foo",
		ExitStatus: 0,
	}
	tests.Assert(t, r.Error() == "", "expected \"\" got:", r.Error())

	// an incomplete result
	r = Result{}
	tests.Assert(t, r.Error() == "", "expected \"\" got:", r.Error())

	// a command that ran & completed with error
	r = Result{
		Completed:  true,
		ErrOutput:  "Bar bar bar",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	}
	tests.Assert(t, r.Error() == "Bar bar bar",
		"expected \"Bar bar bar\" got:", r.Error())

	// only error set. possibly indicating a conn error
	r = Result{
		Completed: true,
		Err:       fmt.Errorf("something broke"),
	}
	tests.Assert(t, r.Error() == "something broke",
		"expected \"something broke\" got:", r.Error())
}

func TestSquashErrors(t *testing.T) {
	var rs Results

	rs = append(rs, Result{
		Completed:  true,
		Output:     "Foo",
		ExitStatus: 0,
	})
	o, e := rs.SquashErrors()
	tests.Assert(t, e == nil, "expected e == nil")
	tests.Assert(t, len(o) == 1, "expected len(o) == 1, got:", len(o))
	tests.Assert(t, o[0] == "Foo", `expected o[0] == "Foo", got:`, o[0])

	rs = append(rs, Result{
		Completed:  true,
		Output:     "Flup",
		ExitStatus: 0,
	})
	o, e = rs.SquashErrors()
	tests.Assert(t, e == nil, "expected e == nil")
	tests.Assert(t, len(o) == 2, "expected len(o) == 2, got:", len(o))
	tests.Assert(t, o[1] == "Flup", `expected o[1] == "Flup", got:`, o[1])

	rs = append(rs, Result{
		Completed:  true,
		ErrOutput:  "Bar bar bar",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	})
	o, e = rs.SquashErrors()
	tests.Assert(t, e != nil, "expected e != nil")
	tests.Assert(t, e.Error() == "Bar bar bar",
		"expected \"Bar bar bar\" got:", e.Error())

	// check empty array behavior
	rs = Results{}
	o, e = rs.SquashErrors()
	tests.Assert(t, e == nil, "expected e == nil")
	tests.Assert(t, len(o) == 0, "expected len(o) == 0, got:", len(o))

	// check short array
	rs = make(Results, 2)
	rs[0] = Result{
		Completed:  true,
		Output:     "Foo",
		ExitStatus: 0,
	}
	o, e = rs.SquashErrors()
	tests.Assert(t, e == nil, "expected e == nil")
	tests.Assert(t, len(o) == 2, "expected len(o) == 2, got:", len(o))
	tests.Assert(t, o[0] == "Foo", `expected o[0] == "Foo", got:`, o[0])
	tests.Assert(t, o[1] == "", `expected o[0] == "", got:`, o[0])
}

func TestResultsOk(t *testing.T) {
	var rs Results

	// empty is always Ok
	tests.Assert(t, rs.Ok(), "expected r.Ok() to be true")

	rs = make(Results, 5)
	rs[0] = Result{
		Completed:  true,
		Output:     "Foo",
		ExitStatus: 0,
	}

	// its short, but no errors, thus true
	tests.Assert(t, rs.Ok(), "expected r.Ok() to be true")

	rs[1] = Result{
		Completed:  true,
		Output:     "Flup",
		ExitStatus: 0,
	}

	// its short, but no errors, thus true
	tests.Assert(t, rs.Ok(), "expected r.Ok() to be true")

	rs[2] = Result{
		Completed:  true,
		ErrOutput:  "Bar bar bar",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	}

	// now an error is present, its false
	tests.Assert(t, !rs.Ok(), "expected rs.Ok() to be false")
}

func TestResultsFirstErrorIndexed(t *testing.T) {
	var rs Results

	// empty is always Ok
	ei, e := rs.FirstErrorIndexed()
	tests.Assert(t, ei == -1, "expected ei == -1, got:", ei)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	rs = make(Results, 5)
	rs[0] = Result{
		Completed:  true,
		Output:     "Foo",
		ExitStatus: 0,
	}

	ei, e = rs.FirstErrorIndexed()
	tests.Assert(t, ei == -1, "expected ei == -1, got:", ei)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	rs[1] = Result{
		Completed:  true,
		Output:     "Flup",
		ExitStatus: 0,
	}

	ei, e = rs.FirstErrorIndexed()
	tests.Assert(t, ei == -1, "expected ei == -1, got:", ei)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	rs[2] = Result{
		Completed:  true,
		ErrOutput:  "Bar bar bar",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	}

	ei, e = rs.FirstErrorIndexed()
	tests.Assert(t, ei == 2, "expected ei == 2, got:", ei)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Bar bar bar",
		"expected \"Bar bar bar\" got:", e.Error())

	// it ignores "holes"
	rs[1] = Result{}

	ei, e = rs.FirstErrorIndexed()
	tests.Assert(t, ei == 2, "expected ei == 2, got:", ei)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Bar bar bar",
		"expected \"Bar bar bar\" got:", e.Error())

	// put error earlier
	rs[1] = Result{
		Completed:  true,
		ErrOutput:  "Robble robble",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	}

	ei, e = rs.FirstErrorIndexed()
	tests.Assert(t, ei == 1, "expected ei == 1, got:", ei)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Robble robble",
		"expected \"Robble robble\" got:", e.Error())
}

func TestResultsFirstError(t *testing.T) {
	var rs Results

	// empty is always Ok
	e := rs.FirstError()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	rs = make(Results, 5)
	rs[0] = Result{
		Completed:  true,
		Output:     "Foo",
		ExitStatus: 0,
	}

	e = rs.FirstError()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	rs[1] = Result{
		Completed:  true,
		Output:     "Flup",
		ExitStatus: 0,
	}

	e = rs.FirstError()
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	rs[2] = Result{
		Completed:  true,
		ErrOutput:  "Bar bar bar",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	}

	e = rs.FirstError()
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Bar bar bar",
		"expected \"Bar bar bar\" got:", e.Error())

	// it ignores "holes"
	rs[1] = Result{}

	e = rs.FirstError()
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Bar bar bar",
		"expected \"Bar bar bar\" got:", e.Error())

	// put error earlier
	rs[1] = Result{
		Completed:  true,
		ErrOutput:  "Robble robble",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	}

	e = rs.FirstError()
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Robble robble",
		"expected \"Robble robble\" got:", e.Error())
}

func TestAnyError(t *testing.T) {
	var rs Results

	// empty is always Ok
	e := AnyError(rs, nil)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	rs = make(Results, 5)
	rs[0] = Result{
		Completed:  true,
		Output:     "Foo",
		ExitStatus: 0,
	}

	e = AnyError(rs, nil)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	rs[1] = Result{
		Completed:  true,
		Output:     "Flup",
		ExitStatus: 0,
	}

	e = AnyError(rs, nil)
	tests.Assert(t, e == nil, "expected e == nil, got:", e)

	e = AnyError(rs, fmt.Errorf("Robble robble"))
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Robble robble",
		"expected \"Robble robble\" got:", e.Error())

	rs[2] = Result{
		Completed:  true,
		ErrOutput:  "Bar bar bar",
		Err:        fmt.Errorf("command exited 1"),
		ExitStatus: 1,
	}

	e = AnyError(rs, nil)
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Bar bar bar",
		"expected \"Bar bar bar\" got:", e.Error())

	e = AnyError(rs, fmt.Errorf("Robble robble"))
	tests.Assert(t, e != nil, "expected e != nil, got:", e)
	tests.Assert(t, e.Error() == "Robble robble",
		"expected \"Robble robble\" got:", e.Error())
}
