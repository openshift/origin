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
	"testing"

	"github.com/heketi/tests"

	"github.com/heketi/heketi/pkg/glusterfs/api"
)

type testTaggable struct {
	T map[string]string
}

func (t *testTaggable) AllTags() map[string]string {
	return t.T
}

func (t *testTaggable) SetTags(tags map[string]string) error {
	t.T = tags
	return nil
}

func TestApplyTagsSet(t *testing.T) {
	tt := &testTaggable{}
	r := api.TagsChangeRequest{
		Tags: map[string]string{
			"testcase": "TestApplyTagsSet",
			"success":  "yes, please",
		},
		Change: api.SetTags,
	}
	ApplyTags(tt, r)
	tests.Assert(t, len(tt.T) == 2, "expected len(tt.T) == 2, got:", len(tt.T))
	tests.Assert(t, tt.T["testcase"] == "TestApplyTagsSet",
		`expected tt.T["testcase"] == "TestApplyTagsSet", got:`,
		tt.T["testcase"])
}

func TestApplyTagsUpdate(t *testing.T) {
	tt := &testTaggable{}
	r := api.TagsChangeRequest{
		Tags: map[string]string{
			"testcase": "TestApplyTagsUpdate",
			"success":  "yes, please",
		},
		Change: api.UpdateTags,
	}
	ApplyTags(tt, r)
	tests.Assert(t, len(tt.T) == 2, "expected len(tt.T) == 2, got:", len(tt.T))
	tests.Assert(t, tt.T["testcase"] == "TestApplyTagsUpdate",
		`expected tt.T["testcase"] == "TestApplyTagsUpdate", got:`,
		tt.T["testcase"])

	r = api.TagsChangeRequest{
		Tags: map[string]string{
			"testcase": "i already told you",
			"king":     "buzzo",
		},
		Change: api.UpdateTags,
	}
	ApplyTags(tt, r)
	tests.Assert(t, len(tt.T) == 3, "expected len(tt.T) == 3, got:", len(tt.T))
	tests.Assert(t, tt.T["testcase"] == "i already told you",
		`expected tt.T["testcase"] == "i already told you", got:`,
		tt.T["testcase"])
}

func TestApplyTagsDelete(t *testing.T) {
	tt := &testTaggable{}
	r := api.TagsChangeRequest{
		Tags: map[string]string{
			"rock": "",
		},
		Change: api.DeleteTags,
	}
	ApplyTags(tt, r)
	tests.Assert(t, len(tt.T) == 0, "expected len(tt.T) == 0, got:", len(tt.T))

	r = api.TagsChangeRequest{
		Tags: map[string]string{
			"rock":     "yes",
			"roll":     "yes",
			"testcase": "TestApplyTagsDelete",
		},
		Change: api.UpdateTags,
	}
	ApplyTags(tt, r)
	tests.Assert(t, len(tt.T) == 3, "expected len(tt.T) == 3, got:", len(tt.T))

	r = api.TagsChangeRequest{
		Tags: map[string]string{
			"rock": "",
			"roll": "xxx",
		},
		Change: api.DeleteTags,
	}
	ApplyTags(tt, r)
	tests.Assert(t, len(tt.T) == 1, "expected len(tt.T) == 1, got:", len(tt.T))
}

func TestMergeTags(t *testing.T) {
	tt1 := &testTaggable{map[string]string{
		"king": "buzzo",
		"rock": "n roll",
		"road": "bull",
	}}
	tt2 := &testTaggable{map[string]string{
		"king": "elvis",
		"rock": "igneous",
	}}

	// entries in tt2 "have priority"
	m := MergeTags(tt1, tt2)
	tests.Assert(t, len(m) == 3, "expected len(m) == 3, got:", len(m))
	tests.Assert(t, m["king"] == "elvis",
		`expected m["king"] == "elvis", got:`, m["king"])
	tests.Assert(t, m["rock"] == "igneous",
		`expected m["rock"] == "igneous", got:`, m["rock"])
	tests.Assert(t, m["road"] == "bull",
		`expected m["road"] == "bull", got:`, m["road"])

	// entries in tt1 "have priority"
	m = MergeTags(tt2, tt1)
	tests.Assert(t, len(m) == 3, "expected len(m) == 3, got:", len(m))
	tests.Assert(t, m["king"] == "buzzo",
		`expected m["king"] == "buzzo", got:`, m["king"])
	tests.Assert(t, m["rock"] == "n roll",
		`expected m["rock"] == "n roll", got:`, m["rock"])
	tests.Assert(t, m["road"] == "bull",
		`expected m["road"] == "bull", got:`, m["road"])

	// entries in tt1 "have priority" + dummy items
	ttx := &testTaggable{map[string]string{}}
	m = MergeTags(tt2, ttx, tt1, ttx)
	tests.Assert(t, len(m) == 3, "expected len(m) == 3, got:", len(m))
	tests.Assert(t, m["king"] == "buzzo",
		`expected m["king"] == "buzzo", got:`, m["king"])
	tests.Assert(t, m["rock"] == "n roll",
		`expected m["rock"] == "n roll", got:`, m["rock"])
	tests.Assert(t, m["road"] == "bull",
		`expected m["road"] == "bull", got:`, m["road"])
}

func TestCopyTags(t *testing.T) {
	tags := copyTags(nil)
	tests.Assert(t, tags != nil, "expected tags != nil")
	tests.Assert(t, len(tags) == 0, "expected len(tags) == 0, got:", len(tags))

	tags = copyTags(map[string]string{})
	tests.Assert(t, tags != nil, "expected tags != nil")
	tests.Assert(t, len(tags) == 0, "expected len(tags) == 0, got:", len(tags))

	tags = copyTags(map[string]string{
		"foo":  "bar",
		"blip": "blap",
	})
	tests.Assert(t, tags != nil, "expected tags != nil")
	tests.Assert(t, len(tags) == 2, "expected len(tags) == 2, got:", len(tags))

	tags2 := copyTags(tags)
	tests.Assert(t, tags != nil, "expected tags != nil")
	tests.Assert(t, len(tags) == 2, "expected len(tags) == 2, got:", len(tags))
	tags["foo"] = "oof"
	tags["mumble"] = "jumble"

	tests.Assert(t, len(tags) == 3, "expected len(tags) == 3, got:", len(tags))
	tests.Assert(t, len(tags2) == 2, "expected len(tags) == 2, got:", len(tags))
	tests.Assert(t, tags["foo"] == "oof",
		`expected tags["foo"] == "oof", got:`, tags["foo"])
	tests.Assert(t, tags2["foo"] == "bar",
		`expected tags2["foo"] == "bar", got:`, tags2["foo"])
}

func TestArbiterTag(t *testing.T) {
	tags := map[string]string{
		"flim": "flam",
	}

	// no explicit arbiter tag
	a := ArbiterTag(tags)
	tests.Assert(t, a == TAG_VAL_ARBITER_SUPPORTED,
		"expected a == TAG_VAL_ARBITER_SUPPORTED, got", a)

	// explicit arbiter tag (supported)
	tags["arbiter"] = "supported"
	a = ArbiterTag(tags)
	tests.Assert(t, a == TAG_VAL_ARBITER_SUPPORTED,
		"expected a == TAG_VAL_ARBITER_SUPPORTED, got", a)

	// explicit arbiter tag (disabled)
	tags["arbiter"] = "disabled"
	a = ArbiterTag(tags)
	tests.Assert(t, a == TAG_VAL_ARBITER_DISABLED,
		"expected a == TAG_VAL_ARBITER_DISABLED, got", a)

	// explicit arbiter tag (supported)
	tags["arbiter"] = "required"
	a = ArbiterTag(tags)
	tests.Assert(t, a == TAG_VAL_ARBITER_REQUIRED,
		"expected a == TAG_VAL_ARBITER_REQUIRED, got", a)

	// garbage value
	tags["arbiter"] = "retibra"
	a = ArbiterTag(tags)
	tests.Assert(t, a == TAG_VAL_ARBITER_SUPPORTED,
		"expected a == TAG_VAL_ARBITER_SUPPORTED, got", a)
}
