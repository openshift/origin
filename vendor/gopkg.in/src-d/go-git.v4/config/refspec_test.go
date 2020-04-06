package config

import (
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type RefSpecSuite struct{}

var _ = Suite(&RefSpecSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *RefSpecSuite) TestRefSpecIsValid(c *C) {
	spec := RefSpec("+refs/heads/*:refs/remotes/origin/*")
	c.Assert(spec.Validate(), Equals, nil)

	spec = RefSpec("refs/heads/*:refs/remotes/origin/")
	c.Assert(spec.Validate(), Equals, ErrRefSpecMalformedWildcard)

	spec = RefSpec("refs/heads/master:refs/remotes/origin/master")
	c.Assert(spec.Validate(), Equals, nil)

	spec = RefSpec(":refs/heads/master")
	c.Assert(spec.Validate(), Equals, nil)

	spec = RefSpec(":refs/heads/*")
	c.Assert(spec.Validate(), Equals, ErrRefSpecMalformedWildcard)

	spec = RefSpec(":*")
	c.Assert(spec.Validate(), Equals, ErrRefSpecMalformedWildcard)

	spec = RefSpec("refs/heads/*")
	c.Assert(spec.Validate(), Equals, ErrRefSpecMalformedSeparator)

	spec = RefSpec("refs/heads:")
	c.Assert(spec.Validate(), Equals, ErrRefSpecMalformedSeparator)
}

func (s *RefSpecSuite) TestRefSpecIsForceUpdate(c *C) {
	spec := RefSpec("+refs/heads/*:refs/remotes/origin/*")
	c.Assert(spec.IsForceUpdate(), Equals, true)

	spec = RefSpec("refs/heads/*:refs/remotes/origin/*")
	c.Assert(spec.IsForceUpdate(), Equals, false)
}

func (s *RefSpecSuite) TestRefSpecIsDelete(c *C) {
	spec := RefSpec(":refs/heads/master")
	c.Assert(spec.IsDelete(), Equals, true)

	spec = RefSpec("+refs/heads/*:refs/remotes/origin/*")
	c.Assert(spec.IsDelete(), Equals, false)

	spec = RefSpec("refs/heads/*:refs/remotes/origin/*")
	c.Assert(spec.IsDelete(), Equals, false)
}

func (s *RefSpecSuite) TestRefSpecSrc(c *C) {
	spec := RefSpec("refs/heads/*:refs/remotes/origin/*")
	c.Assert(spec.Src(), Equals, "refs/heads/*")

	spec = RefSpec("+refs/heads/*:refs/remotes/origin/*")
	c.Assert(spec.Src(), Equals, "refs/heads/*")

	spec = RefSpec(":refs/heads/master")
	c.Assert(spec.Src(), Equals, "")

	spec = RefSpec("refs/heads/love+hate:refs/heads/love+hate")
	c.Assert(spec.Src(), Equals, "refs/heads/love+hate")

	spec = RefSpec("+refs/heads/love+hate:refs/heads/love+hate")
	c.Assert(spec.Src(), Equals, "refs/heads/love+hate")
}

func (s *RefSpecSuite) TestRefSpecMatch(c *C) {
	spec := RefSpec("refs/heads/master:refs/remotes/origin/master")
	c.Assert(spec.Match(plumbing.ReferenceName("refs/heads/foo")), Equals, false)
	c.Assert(spec.Match(plumbing.ReferenceName("refs/heads/master")), Equals, true)

	spec = RefSpec("+refs/heads/master:refs/remotes/origin/master")
	c.Assert(spec.Match(plumbing.ReferenceName("refs/heads/foo")), Equals, false)
	c.Assert(spec.Match(plumbing.ReferenceName("refs/heads/master")), Equals, true)

	spec = RefSpec(":refs/heads/master")
	c.Assert(spec.Match(plumbing.ReferenceName("")), Equals, true)
	c.Assert(spec.Match(plumbing.ReferenceName("refs/heads/master")), Equals, false)

	spec = RefSpec("refs/heads/love+hate:heads/love+hate")
	c.Assert(spec.Match(plumbing.ReferenceName("refs/heads/love+hate")), Equals, true)

	spec = RefSpec("+refs/heads/love+hate:heads/love+hate")
	c.Assert(spec.Match(plumbing.ReferenceName("refs/heads/love+hate")), Equals, true)
}

func (s *RefSpecSuite) TestRefSpecMatchGlob(c *C) {
	tests := map[string]map[string]bool{
		"refs/heads/*:refs/remotes/origin/*": {
			"refs/tag/foo":   false,
			"refs/heads/foo": true,
		},
		"refs/heads/*bc:refs/remotes/origin/*bc": {
			"refs/heads/abc": true,
			"refs/heads/bc":  true,
			"refs/heads/abx": false,
		},
		"refs/heads/a*c:refs/remotes/origin/a*c": {
			"refs/heads/abc": true,
			"refs/heads/ac":  true,
			"refs/heads/abx": false,
		},
		"refs/heads/ab*:refs/remotes/origin/ab*": {
			"refs/heads/abc": true,
			"refs/heads/ab":  true,
			"refs/heads/xbc": false,
		},
	}

	for specStr, data := range tests {
		spec := RefSpec(specStr)
		for ref, matches := range data {
			c.Assert(spec.Match(plumbing.ReferenceName(ref)),
				Equals,
				matches,
				Commentf("while matching spec %q against ref %q", specStr, ref),
			)
		}
	}
}

func (s *RefSpecSuite) TestRefSpecDst(c *C) {
	spec := RefSpec("refs/heads/master:refs/remotes/origin/master")
	c.Assert(
		spec.Dst(plumbing.ReferenceName("refs/heads/master")).String(), Equals,
		"refs/remotes/origin/master",
	)
}

func (s *RefSpecSuite) TestRefSpecDstBlob(c *C) {
	ref := "refs/heads/abc"
	tests := map[string]string{
		"refs/heads/*:refs/remotes/origin/*":       "refs/remotes/origin/abc",
		"refs/heads/*bc:refs/remotes/origin/*":     "refs/remotes/origin/a",
		"refs/heads/*bc:refs/remotes/origin/*bc":   "refs/remotes/origin/abc",
		"refs/heads/a*c:refs/remotes/origin/*":     "refs/remotes/origin/b",
		"refs/heads/a*c:refs/remotes/origin/a*c":   "refs/remotes/origin/abc",
		"refs/heads/ab*:refs/remotes/origin/*":     "refs/remotes/origin/c",
		"refs/heads/ab*:refs/remotes/origin/ab*":   "refs/remotes/origin/abc",
		"refs/heads/*abc:refs/remotes/origin/*abc": "refs/remotes/origin/abc",
		"refs/heads/abc*:refs/remotes/origin/abc*": "refs/remotes/origin/abc",
		// for these two cases, git specifically logs:
		// error: * Ignoring funny ref 'refs/remotes/origin/' locally
		// and ignores the ref; go-git does not currently do this validation,
		// but probably should.
		// "refs/heads/*abc:refs/remotes/origin/*": "",
		// "refs/heads/abc*:refs/remotes/origin/*": "",
	}

	for specStr, dst := range tests {
		spec := RefSpec(specStr)
		c.Assert(spec.Dst(plumbing.ReferenceName(ref)).String(),
			Equals,
			dst,
			Commentf("while getting dst from spec %q with ref %q", specStr, ref),
		)
	}
}

func (s *RefSpecSuite) TestRefSpecReverse(c *C) {
	spec := RefSpec("refs/heads/*:refs/remotes/origin/*")
	c.Assert(
		spec.Reverse(), Equals,
		RefSpec("refs/remotes/origin/*:refs/heads/*"),
	)
}

func (s *RefSpecSuite) TestMatchAny(c *C) {
	specs := []RefSpec{
		"refs/heads/bar:refs/remotes/origin/foo",
		"refs/heads/foo:refs/remotes/origin/bar",
	}

	c.Assert(MatchAny(specs, plumbing.ReferenceName("refs/heads/foo")), Equals, true)
	c.Assert(MatchAny(specs, plumbing.ReferenceName("refs/heads/bar")), Equals, true)
	c.Assert(MatchAny(specs, plumbing.ReferenceName("refs/heads/master")), Equals, false)
}
