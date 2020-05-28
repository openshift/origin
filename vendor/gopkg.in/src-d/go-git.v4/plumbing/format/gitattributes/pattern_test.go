package gitattributes

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type PatternSuite struct{}

var _ = Suite(&PatternSuite{})

func (s *PatternSuite) TestMatch_domainLonger_mismatch(c *C) {
	p := ParsePattern("value", []string{"head", "middle", "tail"})
	r := p.Match([]string{"head", "middle"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestMatch_domainSameLength_mismatch(c *C) {
	p := ParsePattern("value", []string{"head", "middle", "tail"})
	r := p.Match([]string{"head", "middle", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestMatch_domainMismatch_mismatch(c *C) {
	p := ParsePattern("value", []string{"head", "middle", "tail"})
	r := p.Match([]string{"head", "middle", "_tail_", "value"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestSimpleMatch_match(c *C) {
	p := ParsePattern("vul?ano", nil)
	r := p.Match([]string{"value", "vulkano"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestSimpleMatch_withDomain(c *C) {
	p := ParsePattern("middle/tail", []string{"value", "volcano"})
	r := p.Match([]string{"value", "volcano", "middle", "tail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestSimpleMatch_onlyMatchInDomain_mismatch(c *C) {
	p := ParsePattern("value/volcano", []string{"value", "volcano"})
	r := p.Match([]string{"value", "volcano", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestSimpleMatch_atStart(c *C) {
	p := ParsePattern("value", nil)
	r := p.Match([]string{"value", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestSimpleMatch_inTheMiddle(c *C) {
	p := ParsePattern("value", nil)
	r := p.Match([]string{"head", "value", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestSimpleMatch_atEnd(c *C) {
	p := ParsePattern("value", nil)
	r := p.Match([]string{"head", "value"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestSimpleMatch_mismatch(c *C) {
	p := ParsePattern("value", nil)
	r := p.Match([]string{"head", "val", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestSimpleMatch_valueLonger_mismatch(c *C) {
	p := ParsePattern("tai", nil)
	r := p.Match([]string{"head", "value", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestSimpleMatch_withAsterisk(c *C) {
	p := ParsePattern("t*l", nil)
	r := p.Match([]string{"value", "vulkano", "tail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestSimpleMatch_withQuestionMark(c *C) {
	p := ParsePattern("ta?l", nil)
	r := p.Match([]string{"value", "vulkano", "tail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestSimpleMatch_magicChars(c *C) {
	p := ParsePattern("v[ou]l[kc]ano", nil)
	r := p.Match([]string{"value", "volcano"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestSimpleMatch_wrongPattern_mismatch(c *C) {
	p := ParsePattern("v[ou]l[", nil)
	r := p.Match([]string{"value", "vol["})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_fromRootWithSlash(c *C) {
	p := ParsePattern("/value/vul?ano/tail", nil)
	r := p.Match([]string{"value", "vulkano", "tail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_withDomain(c *C) {
	p := ParsePattern("middle/tail", []string{"value", "volcano"})
	r := p.Match([]string{"value", "volcano", "middle", "tail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_onlyMatchInDomain_mismatch(c *C) {
	p := ParsePattern("volcano/tail", []string{"value", "volcano"})
	r := p.Match([]string{"value", "volcano", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_fromRootWithoutSlash(c *C) {
	p := ParsePattern("value/vul?ano/tail", nil)
	r := p.Match([]string{"value", "vulkano", "tail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_fromRoot_mismatch(c *C) {
	p := ParsePattern("value/vulkano", nil)
	r := p.Match([]string{"value", "volcano"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_fromRoot_tooShort_mismatch(c *C) {
	p := ParsePattern("value/vul?ano", nil)
	r := p.Match([]string{"value"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_fromRoot_notAtRoot_mismatch(c *C) {
	p := ParsePattern("/value/volcano", nil)
	r := p.Match([]string{"value", "value", "volcano"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_leadingAsterisks_atStart(c *C) {
	p := ParsePattern("**/*lue/vol?ano/ta?l", nil)
	r := p.Match([]string{"value", "volcano", "tail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_leadingAsterisks_notAtStart(c *C) {
	p := ParsePattern("**/*lue/vol?ano/tail", nil)
	r := p.Match([]string{"head", "value", "volcano", "tail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_leadingAsterisks_mismatch(c *C) {
	p := ParsePattern("**/*lue/vol?ano/tail", nil)
	r := p.Match([]string{"head", "value", "Volcano", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_tailingAsterisks(c *C) {
	p := ParsePattern("/*lue/vol?ano/**", nil)
	r := p.Match([]string{"value", "volcano", "tail", "moretail"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_tailingAsterisks_single(c *C) {
	p := ParsePattern("/*lue/**", nil)
	r := p.Match([]string{"value", "volcano"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_tailingAsterisks_exactMatch(c *C) {
	p := ParsePattern("/*lue/vol?ano/**", nil)
	r := p.Match([]string{"value", "volcano"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_middleAsterisks_emptyMatch(c *C) {
	p := ParsePattern("/*lue/**/vol?ano", nil)
	r := p.Match([]string{"value", "volcano"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_middleAsterisks_oneMatch(c *C) {
	p := ParsePattern("/*lue/**/vol?ano", nil)
	r := p.Match([]string{"value", "middle", "volcano"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_middleAsterisks_multiMatch(c *C) {
	p := ParsePattern("/*lue/**/vol?ano", nil)
	r := p.Match([]string{"value", "middle1", "middle2", "volcano"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_wrongDoubleAsterisk_mismatch(c *C) {
	p := ParsePattern("/*lue/**foo/vol?ano/tail", nil)
	r := p.Match([]string{"value", "foo", "volcano", "tail"})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_magicChars(c *C) {
	p := ParsePattern("**/head/v[ou]l[kc]ano", nil)
	r := p.Match([]string{"value", "head", "volcano"})
	c.Assert(r, Equals, true)
}

func (s *PatternSuite) TestGlobMatch_wrongPattern_noTraversal_mismatch(c *C) {
	p := ParsePattern("**/head/v[ou]l[", nil)
	r := p.Match([]string{"value", "head", "vol["})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_wrongPattern_onTraversal_mismatch(c *C) {
	p := ParsePattern("/value/**/v[ou]l[", nil)
	r := p.Match([]string{"value", "head", "vol["})
	c.Assert(r, Equals, false)
}

func (s *PatternSuite) TestGlobMatch_issue_923(c *C) {
	p := ParsePattern("**/android/**/GeneratedPluginRegistrant.java", nil)
	r := p.Match([]string{"packages", "flutter_tools", "lib", "src", "android", "gradle.dart"})
	c.Assert(r, Equals, false)
}
