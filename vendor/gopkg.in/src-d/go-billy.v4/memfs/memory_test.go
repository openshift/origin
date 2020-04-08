package memfs

import (
	"io"
	"testing"

	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/test"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type MemorySuite struct {
	test.FilesystemSuite
	path string
}

var _ = Suite(&MemorySuite{})

func (s *MemorySuite) SetUpTest(c *C) {
	s.FilesystemSuite = test.NewFilesystemSuite(New())
}

func (s *MemorySuite) TestCapabilities(c *C) {
	_, ok := s.FS.(billy.Capable)
	c.Assert(ok, Equals, true)

	caps := billy.Capabilities(s.FS)
	c.Assert(caps, Equals, billy.DefaultCapabilities&^billy.LockCapability)
}

func (s *MemorySuite) TestNegativeOffsets(c *C) {
	f, err := s.FS.Create("negative")
	c.Assert(err, IsNil)

	buf := make([]byte, 100)
	_, err = f.ReadAt(buf, -100)
	c.Assert(err, ErrorMatches, "readat negative: negative offset")

	_, err = f.Seek(-100, io.SeekCurrent)
	c.Assert(err, IsNil)
	_, err = f.Write(buf)
	c.Assert(err, ErrorMatches, "writeat negative: negative offset")
}
