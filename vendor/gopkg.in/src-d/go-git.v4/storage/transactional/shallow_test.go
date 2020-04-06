package transactional

import (
	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

var _ = Suite(&ShallowSuite{})

type ShallowSuite struct{}

func (s *ShallowSuite) TestShallow(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	rs := NewShallowStorage(base, temporal)

	commitA := plumbing.NewHash("bc9968d75e48de59f0870ffb71f5e160bbbdcf52")
	commitB := plumbing.NewHash("aa9968d75e48de59f0870ffb71f5e160bbbdcf52")

	err := base.SetShallow([]plumbing.Hash{commitA})
	c.Assert(err, IsNil)

	err = rs.SetShallow([]plumbing.Hash{commitB})
	c.Assert(err, IsNil)

	commits, err := rs.Shallow()
	c.Assert(err, IsNil)
	c.Assert(commits, HasLen, 1)
	c.Assert(commits[0], Equals, commitB)

	commits, err = base.Shallow()
	c.Assert(err, IsNil)
	c.Assert(commits, HasLen, 1)
	c.Assert(commits[0], Equals, commitA)
}

func (s *ShallowSuite) TestCommit(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	rs := NewShallowStorage(base, temporal)

	commitA := plumbing.NewHash("bc9968d75e48de59f0870ffb71f5e160bbbdcf52")
	commitB := plumbing.NewHash("aa9968d75e48de59f0870ffb71f5e160bbbdcf52")

	c.Assert(base.SetShallow([]plumbing.Hash{commitA}), IsNil)
	c.Assert(rs.SetShallow([]plumbing.Hash{commitB}), IsNil)

	c.Assert(rs.Commit(), IsNil)

	commits, err := rs.Shallow()
	c.Assert(err, IsNil)
	c.Assert(commits, HasLen, 1)
	c.Assert(commits[0], Equals, commitB)

	commits, err = base.Shallow()
	c.Assert(err, IsNil)
	c.Assert(commits, HasLen, 1)
	c.Assert(commits[0], Equals, commitB)
}
