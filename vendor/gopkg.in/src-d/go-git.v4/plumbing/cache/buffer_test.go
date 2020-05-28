package cache

import (
	"bytes"
	"sync"

	. "gopkg.in/check.v1"
)

type BufferSuite struct {
	c       map[string]Buffer
	aBuffer []byte
	bBuffer []byte
	cBuffer []byte
	dBuffer []byte
	eBuffer []byte
}

var _ = Suite(&BufferSuite{})

func (s *BufferSuite) SetUpTest(c *C) {
	s.aBuffer = []byte("a")
	s.bBuffer = []byte("bbb")
	s.cBuffer = []byte("c")
	s.dBuffer = []byte("d")
	s.eBuffer = []byte("ee")

	s.c = make(map[string]Buffer)
	s.c["two_bytes"] = NewBufferLRU(2 * Byte)
	s.c["default_lru"] = NewBufferLRUDefault()
}

func (s *BufferSuite) TestPutSameBuffer(c *C) {
	for _, o := range s.c {
		o.Put(1, s.aBuffer)
		o.Put(1, s.aBuffer)
		_, ok := o.Get(1)
		c.Assert(ok, Equals, true)
	}
}

func (s *ObjectSuite) TestPutSameBufferWithDifferentSize(c *C) {
	aBuffer := []byte("a")
	bBuffer := []byte("bbb")
	cBuffer := []byte("ccccc")
	dBuffer := []byte("ddddddd")

	cache := NewBufferLRU(7 * Byte)
	cache.Put(1, aBuffer)
	cache.Put(1, bBuffer)
	cache.Put(1, cBuffer)
	cache.Put(1, dBuffer)

	c.Assert(cache.MaxSize, Equals, 7*Byte)
	c.Assert(cache.actualSize, Equals, 7*Byte)
	c.Assert(cache.ll.Len(), Equals, 1)

	buf, ok := cache.Get(1)
	c.Assert(bytes.Equal(buf, dBuffer), Equals, true)
	c.Assert(FileSize(len(buf)), Equals, 7*Byte)
	c.Assert(ok, Equals, true)
}

func (s *BufferSuite) TestPutBigBuffer(c *C) {
	for _, o := range s.c {
		o.Put(1, s.bBuffer)
		_, ok := o.Get(2)
		c.Assert(ok, Equals, false)
	}
}

func (s *BufferSuite) TestPutCacheOverflow(c *C) {
	// this test only works with an specific size
	o := s.c["two_bytes"]

	o.Put(1, s.aBuffer)
	o.Put(2, s.cBuffer)
	o.Put(3, s.dBuffer)

	obj, ok := o.Get(1)
	c.Assert(ok, Equals, false)
	c.Assert(obj, IsNil)
	obj, ok = o.Get(2)
	c.Assert(ok, Equals, true)
	c.Assert(obj, NotNil)
	obj, ok = o.Get(3)
	c.Assert(ok, Equals, true)
	c.Assert(obj, NotNil)
}

func (s *BufferSuite) TestEvictMultipleBuffers(c *C) {
	o := s.c["two_bytes"]

	o.Put(1, s.cBuffer)
	o.Put(2, s.dBuffer) // now cache is full with two objects
	o.Put(3, s.eBuffer) // this put should evict all previous objects

	obj, ok := o.Get(1)
	c.Assert(ok, Equals, false)
	c.Assert(obj, IsNil)
	obj, ok = o.Get(2)
	c.Assert(ok, Equals, false)
	c.Assert(obj, IsNil)
	obj, ok = o.Get(3)
	c.Assert(ok, Equals, true)
	c.Assert(obj, NotNil)
}

func (s *BufferSuite) TestClear(c *C) {
	for _, o := range s.c {
		o.Put(1, s.aBuffer)
		o.Clear()
		obj, ok := o.Get(1)
		c.Assert(ok, Equals, false)
		c.Assert(obj, IsNil)
	}
}

func (s *BufferSuite) TestConcurrentAccess(c *C) {
	for _, o := range s.c {
		var wg sync.WaitGroup

		for i := 0; i < 1000; i++ {
			wg.Add(3)
			go func(i int) {
				o.Put(int64(i), []byte{00})
				wg.Done()
			}(i)

			go func(i int) {
				if i%30 == 0 {
					o.Clear()
				}
				wg.Done()
			}(i)

			go func(i int) {
				o.Get(int64(i))
				wg.Done()
			}(i)
		}

		wg.Wait()
	}
}

func (s *BufferSuite) TestDefaultLRU(c *C) {
	defaultLRU := s.c["default_lru"].(*BufferLRU)

	c.Assert(defaultLRU.MaxSize, Equals, DefaultMaxSize)
}
