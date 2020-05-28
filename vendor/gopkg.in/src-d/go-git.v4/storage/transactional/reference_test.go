package transactional

import (
	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

var _ = Suite(&ReferenceSuite{})

type ReferenceSuite struct{}

func (s *ReferenceSuite) TestReference(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	rs := NewReferenceStorage(base, temporal)

	refA := plumbing.NewReferenceFromStrings("refs/a", "bc9968d75e48de59f0870ffb71f5e160bbbdcf52")
	refB := plumbing.NewReferenceFromStrings("refs/b", "bc9968d75e48de59f0870ffb71f5e160bbbdcf52")

	err := base.SetReference(refA)
	c.Assert(err, IsNil)

	err = rs.SetReference(refB)
	c.Assert(err, IsNil)

	_, err = rs.Reference("refs/a")
	c.Assert(err, IsNil)

	_, err = rs.Reference("refs/b")
	c.Assert(err, IsNil)

	_, err = base.Reference("refs/b")
	c.Assert(err, Equals, plumbing.ErrReferenceNotFound)
}

func (s *ReferenceSuite) TestRemoveReferenceTemporal(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	ref := plumbing.NewReferenceFromStrings("refs/a", "bc9968d75e48de59f0870ffb71f5e160bbbdcf52")

	rs := NewReferenceStorage(base, temporal)
	err := rs.SetReference(ref)
	c.Assert(err, IsNil)

	err = rs.RemoveReference("refs/a")
	c.Assert(err, IsNil)

	_, err = rs.Reference("refs/a")
	c.Assert(err, Equals, plumbing.ErrReferenceNotFound)
}

func (s *ReferenceSuite) TestRemoveReferenceBase(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	ref := plumbing.NewReferenceFromStrings("refs/a", "bc9968d75e48de59f0870ffb71f5e160bbbdcf52")

	rs := NewReferenceStorage(base, temporal)
	err := base.SetReference(ref)
	c.Assert(err, IsNil)

	err = rs.RemoveReference("refs/a")
	c.Assert(err, IsNil)

	_, err = rs.Reference("refs/a")
	c.Assert(err, Equals, plumbing.ErrReferenceNotFound)
}

func (s *ReferenceSuite) TestCheckAndSetReferenceInBase(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()
	rs := NewReferenceStorage(base, temporal)

	err := base.SetReference(
		plumbing.NewReferenceFromStrings("foo", "482e0eada5de4039e6f216b45b3c9b683b83bfa"),
	)
	c.Assert(err, IsNil)

	err = rs.CheckAndSetReference(
		plumbing.NewReferenceFromStrings("foo", "bc9968d75e48de59f0870ffb71f5e160bbbdcf52"),
		plumbing.NewReferenceFromStrings("foo", "482e0eada5de4039e6f216b45b3c9b683b83bfa"),
	)
	c.Assert(err, IsNil)

	e, err := rs.Reference(plumbing.ReferenceName("foo"))
	c.Assert(err, IsNil)
	c.Assert(e.Hash().String(), Equals, "bc9968d75e48de59f0870ffb71f5e160bbbdcf52")
}

func (s *ReferenceSuite) TestCommit(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	refA := plumbing.NewReferenceFromStrings("refs/a", "bc9968d75e48de59f0870ffb71f5e160bbbdcf52")
	refB := plumbing.NewReferenceFromStrings("refs/b", "b66c08ba28aa1f81eb06a1127aa3936ff77e5e2c")
	refC := plumbing.NewReferenceFromStrings("refs/c", "c3f4688a08fd86f1bf8e055724c84b7a40a09733")

	rs := NewReferenceStorage(base, temporal)
	c.Assert(rs.SetReference(refA), IsNil)
	c.Assert(rs.SetReference(refB), IsNil)
	c.Assert(rs.SetReference(refC), IsNil)

	err := rs.Commit()
	c.Assert(err, IsNil)

	iter, err := base.IterReferences()
	c.Assert(err, IsNil)

	var count int
	iter.ForEach(func(ref *plumbing.Reference) error {
		count++
		return nil
	})

	c.Assert(count, Equals, 3)
}

func (s *ReferenceSuite) TestCommitDelete(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	refA := plumbing.NewReferenceFromStrings("refs/a", "bc9968d75e48de59f0870ffb71f5e160bbbdcf52")
	refB := plumbing.NewReferenceFromStrings("refs/b", "b66c08ba28aa1f81eb06a1127aa3936ff77e5e2c")
	refC := plumbing.NewReferenceFromStrings("refs/c", "c3f4688a08fd86f1bf8e055724c84b7a40a09733")

	rs := NewReferenceStorage(base, temporal)
	c.Assert(base.SetReference(refA), IsNil)
	c.Assert(base.SetReference(refB), IsNil)
	c.Assert(base.SetReference(refC), IsNil)

	c.Assert(rs.RemoveReference(refA.Name()), IsNil)
	c.Assert(rs.RemoveReference(refB.Name()), IsNil)
	c.Assert(rs.RemoveReference(refC.Name()), IsNil)
	c.Assert(rs.SetReference(refC), IsNil)

	err := rs.Commit()
	c.Assert(err, IsNil)

	iter, err := base.IterReferences()
	c.Assert(err, IsNil)

	var count int
	iter.ForEach(func(ref *plumbing.Reference) error {
		count++
		return nil
	})

	c.Assert(count, Equals, 1)

	ref, err := rs.Reference(refC.Name())
	c.Assert(err, IsNil)
	c.Assert(ref.Hash().String(), Equals, "c3f4688a08fd86f1bf8e055724c84b7a40a09733")

}
