package gitattributes

import (
	"strings"

	. "gopkg.in/check.v1"
)

func (s *MatcherSuite) TestMatcher_Match(c *C) {
	lines := []string{
		"[attr]binary -diff -merge -text",
		"**/middle/v[uo]l?ano binary text eol=crlf",
		"volcano -eol",
		"foobar diff merge text eol=lf foo=bar",
	}

	ma, err := ReadAttributes(strings.NewReader(strings.Join(lines, "\n")), nil, true)
	c.Assert(err, IsNil)

	m := NewMatcher(ma)
	results, matched := m.Match([]string{"head", "middle", "vulkano"}, nil)

	c.Assert(matched, Equals, true)
	c.Assert(results["binary"].IsSet(), Equals, true)
	c.Assert(results["diff"].IsUnset(), Equals, true)
	c.Assert(results["merge"].IsUnset(), Equals, true)
	c.Assert(results["text"].IsSet(), Equals, true)
	c.Assert(results["eol"].Value(), Equals, "crlf")
}
