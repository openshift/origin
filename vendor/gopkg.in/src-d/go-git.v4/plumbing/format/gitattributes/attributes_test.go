package gitattributes

import (
	"strings"

	. "gopkg.in/check.v1"
)

type AttributesSuite struct{}

var _ = Suite(&AttributesSuite{})

func (s *AttributesSuite) TestAttributes_ReadAttributes(c *C) {
	lines := []string{
		"[attr]sub -a",
		"[attr]add a",
		"* sub a",
		"* !a foo=bar -b c",
	}

	mas, err := ReadAttributes(strings.NewReader(strings.Join(lines, "\n")), nil, true)
	c.Assert(err, IsNil)
	c.Assert(len(mas), Equals, 4)

	c.Assert(mas[0].Name, Equals, "sub")
	c.Assert(mas[0].Pattern, IsNil)
	c.Assert(mas[0].Attributes[0].IsUnset(), Equals, true)

	c.Assert(mas[1].Name, Equals, "add")
	c.Assert(mas[1].Pattern, IsNil)
	c.Assert(mas[1].Attributes[0].IsSet(), Equals, true)

	c.Assert(mas[2].Name, Equals, "*")
	c.Assert(mas[2].Pattern, NotNil)
	c.Assert(mas[2].Attributes[0].IsSet(), Equals, true)

	c.Assert(mas[3].Name, Equals, "*")
	c.Assert(mas[3].Pattern, NotNil)
	c.Assert(mas[3].Attributes[0].IsUnspecified(), Equals, true)
	c.Assert(mas[3].Attributes[1].IsValueSet(), Equals, true)
	c.Assert(mas[3].Attributes[1].Value(), Equals, "bar")
	c.Assert(mas[3].Attributes[2].IsUnset(), Equals, true)
	c.Assert(mas[3].Attributes[3].IsSet(), Equals, true)
	c.Assert(mas[3].Attributes[0].String(), Equals, "a: unspecified")
	c.Assert(mas[3].Attributes[1].String(), Equals, "foo: bar")
	c.Assert(mas[3].Attributes[2].String(), Equals, "b: unset")
	c.Assert(mas[3].Attributes[3].String(), Equals, "c: set")
}

func (s *AttributesSuite) TestAttributes_ReadAttributesDisallowMacro(c *C) {
	lines := []string{
		"[attr]sub -a",
		"* a add",
	}

	_, err := ReadAttributes(strings.NewReader(strings.Join(lines, "\n")), nil, false)
	c.Assert(err, Equals, ErrMacroNotAllowed)
}

func (s *AttributesSuite) TestAttributes_ReadAttributesInvalidName(c *C) {
	lines := []string{
		"[attr]foo!bar -a",
	}

	_, err := ReadAttributes(strings.NewReader(strings.Join(lines, "\n")), nil, true)
	c.Assert(err, Equals, ErrInvalidAttributeName)
}
