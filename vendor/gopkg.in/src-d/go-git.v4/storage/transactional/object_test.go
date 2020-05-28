package transactional

import (
	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

var _ = Suite(&ObjectSuite{})

type ObjectSuite struct{}

func (s *ObjectSuite) TestHasEncodedObject(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	os := NewObjectStorage(base, temporal)

	commit := base.NewEncodedObject()
	commit.SetType(plumbing.CommitObject)

	ch, err := base.SetEncodedObject(commit)
	c.Assert(ch.IsZero(), Equals, false)
	c.Assert(err, IsNil)

	tree := base.NewEncodedObject()
	tree.SetType(plumbing.TreeObject)

	th, err := os.SetEncodedObject(tree)
	c.Assert(th.IsZero(), Equals, false)
	c.Assert(err, IsNil)

	err = os.HasEncodedObject(th)
	c.Assert(err, IsNil)

	err = os.HasEncodedObject(ch)
	c.Assert(err, IsNil)

	err = base.HasEncodedObject(th)
	c.Assert(err, Equals, plumbing.ErrObjectNotFound)
}

func (s *ObjectSuite) TestEncodedObjectAndEncodedObjectSize(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	os := NewObjectStorage(base, temporal)

	commit := base.NewEncodedObject()
	commit.SetType(plumbing.CommitObject)

	ch, err := base.SetEncodedObject(commit)
	c.Assert(ch.IsZero(), Equals, false)
	c.Assert(err, IsNil)

	tree := base.NewEncodedObject()
	tree.SetType(plumbing.TreeObject)

	th, err := os.SetEncodedObject(tree)
	c.Assert(th.IsZero(), Equals, false)
	c.Assert(err, IsNil)

	otree, err := os.EncodedObject(plumbing.TreeObject, th)
	c.Assert(err, IsNil)
	c.Assert(otree.Hash(), Equals, tree.Hash())

	treeSz, err := os.EncodedObjectSize(th)
	c.Assert(err, IsNil)
	c.Assert(treeSz, Equals, int64(0))

	ocommit, err := os.EncodedObject(plumbing.CommitObject, ch)
	c.Assert(err, IsNil)
	c.Assert(ocommit.Hash(), Equals, commit.Hash())

	commitSz, err := os.EncodedObjectSize(ch)
	c.Assert(err, IsNil)
	c.Assert(commitSz, Equals, int64(0))

	_, err = base.EncodedObject(plumbing.TreeObject, th)
	c.Assert(err, Equals, plumbing.ErrObjectNotFound)

	_, err = base.EncodedObjectSize(th)
	c.Assert(err, Equals, plumbing.ErrObjectNotFound)
}

func (s *ObjectSuite) TestIterEncodedObjects(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	os := NewObjectStorage(base, temporal)

	commit := base.NewEncodedObject()
	commit.SetType(plumbing.CommitObject)

	ch, err := base.SetEncodedObject(commit)
	c.Assert(ch.IsZero(), Equals, false)
	c.Assert(err, IsNil)

	tree := base.NewEncodedObject()
	tree.SetType(plumbing.TreeObject)

	th, err := os.SetEncodedObject(tree)
	c.Assert(th.IsZero(), Equals, false)
	c.Assert(err, IsNil)

	iter, err := os.IterEncodedObjects(plumbing.AnyObject)
	c.Assert(err, IsNil)

	var hashes []plumbing.Hash
	err = iter.ForEach(func(obj plumbing.EncodedObject) error {
		hashes = append(hashes, obj.Hash())
		return nil
	})

	c.Assert(err, IsNil)
	c.Assert(hashes, HasLen, 2)
	c.Assert(hashes[0], Equals, ch)
	c.Assert(hashes[1], Equals, th)
}

func (s *ObjectSuite) TestCommit(c *C) {
	base := memory.NewStorage()
	temporal := memory.NewStorage()

	os := NewObjectStorage(base, temporal)

	commit := base.NewEncodedObject()
	commit.SetType(plumbing.CommitObject)

	_, err := os.SetEncodedObject(commit)
	c.Assert(err, IsNil)

	tree := base.NewEncodedObject()
	tree.SetType(plumbing.TreeObject)

	_, err = os.SetEncodedObject(tree)
	c.Assert(err, IsNil)

	err = os.Commit()
	c.Assert(err, IsNil)

	iter, err := base.IterEncodedObjects(plumbing.AnyObject)
	c.Assert(err, IsNil)

	var hashes []plumbing.Hash
	err = iter.ForEach(func(obj plumbing.EncodedObject) error {
		hashes = append(hashes, obj.Hash())
		return nil
	})

	c.Assert(err, IsNil)
	c.Assert(hashes, HasLen, 2)
}
