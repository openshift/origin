package cache

import (
	"fmt"
	"io"
	"sync"
	"testing"

	"gopkg.in/src-d/go-git.v4/plumbing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type ObjectSuite struct {
	c       map[string]Object
	aObject plumbing.EncodedObject
	bObject plumbing.EncodedObject
	cObject plumbing.EncodedObject
	dObject plumbing.EncodedObject
	eObject plumbing.EncodedObject
}

var _ = Suite(&ObjectSuite{})

func (s *ObjectSuite) SetUpTest(c *C) {
	s.aObject = newObject("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1*Byte)
	s.bObject = newObject("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 3*Byte)
	s.cObject = newObject("cccccccccccccccccccccccccccccccccccccccc", 1*Byte)
	s.dObject = newObject("dddddddddddddddddddddddddddddddddddddddd", 1*Byte)
	s.eObject = newObject("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", 2*Byte)

	s.c = make(map[string]Object)
	s.c["two_bytes"] = NewObjectLRU(2 * Byte)
	s.c["default_lru"] = NewObjectLRUDefault()
}

func (s *ObjectSuite) TestPutSameObject(c *C) {
	for _, o := range s.c {
		o.Put(s.aObject)
		o.Put(s.aObject)
		_, ok := o.Get(s.aObject.Hash())
		c.Assert(ok, Equals, true)
	}
}

func (s *ObjectSuite) TestPutSameObjectWithDifferentSize(c *C) {
	const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	cache := NewObjectLRU(7 * Byte)
	cache.Put(newObject(hash, 1*Byte))
	cache.Put(newObject(hash, 3*Byte))
	cache.Put(newObject(hash, 5*Byte))
	cache.Put(newObject(hash, 7*Byte))

	c.Assert(cache.MaxSize, Equals, 7*Byte)
	c.Assert(cache.actualSize, Equals, 7*Byte)
	c.Assert(cache.ll.Len(), Equals, 1)

	obj, ok := cache.Get(plumbing.NewHash(hash))
	c.Assert(obj.Hash(), Equals, plumbing.NewHash(hash))
	c.Assert(FileSize(obj.Size()), Equals, 7*Byte)
	c.Assert(ok, Equals, true)
}

func (s *ObjectSuite) TestPutBigObject(c *C) {
	for _, o := range s.c {
		o.Put(s.bObject)
		_, ok := o.Get(s.aObject.Hash())
		c.Assert(ok, Equals, false)
	}
}

func (s *ObjectSuite) TestPutCacheOverflow(c *C) {
	// this test only works with an specific size
	o := s.c["two_bytes"]

	o.Put(s.aObject)
	o.Put(s.cObject)
	o.Put(s.dObject)

	obj, ok := o.Get(s.aObject.Hash())
	c.Assert(ok, Equals, false)
	c.Assert(obj, IsNil)
	obj, ok = o.Get(s.cObject.Hash())
	c.Assert(ok, Equals, true)
	c.Assert(obj, NotNil)
	obj, ok = o.Get(s.dObject.Hash())
	c.Assert(ok, Equals, true)
	c.Assert(obj, NotNil)
}

func (s *ObjectSuite) TestEvictMultipleObjects(c *C) {
	o := s.c["two_bytes"]

	o.Put(s.cObject)
	o.Put(s.dObject) // now cache is full with two objects
	o.Put(s.eObject) // this put should evict all previous objects

	obj, ok := o.Get(s.cObject.Hash())
	c.Assert(ok, Equals, false)
	c.Assert(obj, IsNil)
	obj, ok = o.Get(s.dObject.Hash())
	c.Assert(ok, Equals, false)
	c.Assert(obj, IsNil)
	obj, ok = o.Get(s.eObject.Hash())
	c.Assert(ok, Equals, true)
	c.Assert(obj, NotNil)
}

func (s *ObjectSuite) TestClear(c *C) {
	for _, o := range s.c {
		o.Put(s.aObject)
		o.Clear()
		obj, ok := o.Get(s.aObject.Hash())
		c.Assert(ok, Equals, false)
		c.Assert(obj, IsNil)
	}
}

func (s *ObjectSuite) TestConcurrentAccess(c *C) {
	for _, o := range s.c {
		var wg sync.WaitGroup

		for i := 0; i < 1000; i++ {
			wg.Add(3)
			go func(i int) {
				o.Put(newObject(fmt.Sprint(i), FileSize(i)))
				wg.Done()
			}(i)

			go func(i int) {
				if i%30 == 0 {
					o.Clear()
				}
				wg.Done()
			}(i)

			go func(i int) {
				o.Get(plumbing.NewHash(fmt.Sprint(i)))
				wg.Done()
			}(i)
		}

		wg.Wait()
	}
}

func (s *ObjectSuite) TestDefaultLRU(c *C) {
	defaultLRU := s.c["default_lru"].(*ObjectLRU)

	c.Assert(defaultLRU.MaxSize, Equals, DefaultMaxSize)
}

func (s *ObjectSuite) TestObjectUpdateOverflow(c *C) {
	o := NewObjectLRU(9 * Byte)

	a1 := newObject(s.aObject.Hash().String(), 9*Byte)
	a2 := newObject(s.aObject.Hash().String(), 1*Byte)
	b := newObject(s.bObject.Hash().String(), 1*Byte)

	o.Put(a1)
	a1.SetSize(-5)
	o.Put(a2)
	o.Put(b)
}

type dummyObject struct {
	hash plumbing.Hash
	size FileSize
}

func newObject(hash string, size FileSize) plumbing.EncodedObject {
	return &dummyObject{
		hash: plumbing.NewHash(hash),
		size: size,
	}
}

func (d *dummyObject) Hash() plumbing.Hash           { return d.hash }
func (*dummyObject) Type() plumbing.ObjectType       { return plumbing.InvalidObject }
func (*dummyObject) SetType(plumbing.ObjectType)     {}
func (d *dummyObject) Size() int64                   { return int64(d.size) }
func (d *dummyObject) SetSize(s int64)               { d.size = FileSize(s) }
func (*dummyObject) Reader() (io.ReadCloser, error)  { return nil, nil }
func (*dummyObject) Writer() (io.WriteCloser, error) { return nil, nil }
