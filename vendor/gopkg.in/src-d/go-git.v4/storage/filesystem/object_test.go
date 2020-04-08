package filesystem

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/storage/filesystem/dotgit"

	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git-fixtures.v3"
)

type FsSuite struct {
	fixtures.Suite
}

var objectTypes = []plumbing.ObjectType{
	plumbing.CommitObject,
	plumbing.TagObject,
	plumbing.TreeObject,
	plumbing.BlobObject,
}

var _ = Suite(&FsSuite{})

func (s *FsSuite) TestGetFromObjectFile(c *C) {
	fs := fixtures.ByTag(".git").ByTag("unpacked").One().DotGit()
	o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())

	expected := plumbing.NewHash("f3dfe29d268303fc6e1bbce268605fc99573406e")
	obj, err := o.EncodedObject(plumbing.AnyObject, expected)
	c.Assert(err, IsNil)
	c.Assert(obj.Hash(), Equals, expected)
}

func (s *FsSuite) TestGetFromPackfile(c *C) {
	fixtures.Basic().ByTag(".git").Test(c, func(f *fixtures.Fixture) {
		fs := f.DotGit()
		o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())

		expected := plumbing.NewHash("6ecf0ef2c2dffb796033e5a02219af86ec6584e5")
		obj, err := o.EncodedObject(plumbing.AnyObject, expected)
		c.Assert(err, IsNil)
		c.Assert(obj.Hash(), Equals, expected)
	})
}

func (s *FsSuite) TestGetFromPackfileKeepDescriptors(c *C) {
	fixtures.Basic().ByTag(".git").Test(c, func(f *fixtures.Fixture) {
		fs := f.DotGit()
		dg := dotgit.NewWithOptions(fs, dotgit.Options{KeepDescriptors: true})
		o := NewObjectStorageWithOptions(dg, cache.NewObjectLRUDefault(), Options{KeepDescriptors: true})

		expected := plumbing.NewHash("6ecf0ef2c2dffb796033e5a02219af86ec6584e5")
		obj, err := o.EncodedObject(plumbing.AnyObject, expected)
		c.Assert(err, IsNil)
		c.Assert(obj.Hash(), Equals, expected)

		packfiles, err := dg.ObjectPacks()
		c.Assert(err, IsNil)

		pack1, err := dg.ObjectPack(packfiles[0])
		c.Assert(err, IsNil)

		pack1.Seek(42, os.SEEK_SET)

		err = o.Close()
		c.Assert(err, IsNil)

		pack2, err := dg.ObjectPack(packfiles[0])
		c.Assert(err, IsNil)

		offset, err := pack2.Seek(0, os.SEEK_CUR)
		c.Assert(err, IsNil)
		c.Assert(offset, Equals, int64(0))

		err = o.Close()
		c.Assert(err, IsNil)

	})
}

func (s *FsSuite) TestGetFromPackfileMaxOpenDescriptors(c *C) {
	fs := fixtures.ByTag(".git").ByTag("multi-packfile").One().DotGit()
	o := NewObjectStorageWithOptions(dotgit.New(fs), cache.NewObjectLRUDefault(), Options{MaxOpenDescriptors: 1})

	expected := plumbing.NewHash("8d45a34641d73851e01d3754320b33bb5be3c4d3")
	obj, err := o.getFromPackfile(expected, false)
	c.Assert(err, IsNil)
	c.Assert(obj.Hash(), Equals, expected)

	expected = plumbing.NewHash("e9cfa4c9ca160546efd7e8582ec77952a27b17db")
	obj, err = o.getFromPackfile(expected, false)
	c.Assert(err, IsNil)
	c.Assert(obj.Hash(), Equals, expected)

	err = o.Close()
	c.Assert(err, IsNil)
}

func (s *FsSuite) TestGetSizeOfObjectFile(c *C) {
	fs := fixtures.ByTag(".git").ByTag("unpacked").One().DotGit()
	o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())

	// Get the size of `tree_walker.go`.
	expected := plumbing.NewHash("cbd81c47be12341eb1185b379d1c82675aeded6a")
	size, err := o.EncodedObjectSize(expected)
	c.Assert(err, IsNil)
	c.Assert(size, Equals, int64(2412))
}

func (s *FsSuite) TestGetSizeFromPackfile(c *C) {
	fixtures.Basic().ByTag(".git").Test(c, func(f *fixtures.Fixture) {
		fs := f.DotGit()
		o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())

		// Get the size of `binary.jpg`.
		expected := plumbing.NewHash("d5c0f4ab811897cadf03aec358ae60d21f91c50d")
		size, err := o.EncodedObjectSize(expected)
		c.Assert(err, IsNil)
		c.Assert(size, Equals, int64(76110))
	})
}

func (s *FsSuite) TestGetSizeOfAllObjectFiles(c *C) {
	fs := fixtures.ByTag(".git").One().DotGit()
	o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())

	// Get the size of `tree_walker.go`.
	err := o.ForEachObjectHash(func(h plumbing.Hash) error {
		size, err := o.EncodedObjectSize(h)
		c.Assert(err, IsNil)
		c.Assert(size, Not(Equals), int64(0))
		return nil
	})
	c.Assert(err, IsNil)
}

func (s *FsSuite) TestGetFromPackfileMultiplePackfiles(c *C) {
	fs := fixtures.ByTag(".git").ByTag("multi-packfile").One().DotGit()
	o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())

	expected := plumbing.NewHash("8d45a34641d73851e01d3754320b33bb5be3c4d3")
	obj, err := o.getFromPackfile(expected, false)
	c.Assert(err, IsNil)
	c.Assert(obj.Hash(), Equals, expected)

	expected = plumbing.NewHash("e9cfa4c9ca160546efd7e8582ec77952a27b17db")
	obj, err = o.getFromPackfile(expected, false)
	c.Assert(err, IsNil)
	c.Assert(obj.Hash(), Equals, expected)
}

func (s *FsSuite) TestIter(c *C) {
	fixtures.ByTag(".git").ByTag("packfile").Test(c, func(f *fixtures.Fixture) {
		fs := f.DotGit()
		o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())

		iter, err := o.IterEncodedObjects(plumbing.AnyObject)
		c.Assert(err, IsNil)

		var count int32
		err = iter.ForEach(func(o plumbing.EncodedObject) error {
			count++
			return nil
		})

		c.Assert(err, IsNil)
		c.Assert(count, Equals, f.ObjectsCount)
	})
}

func (s *FsSuite) TestIterWithType(c *C) {
	fixtures.ByTag(".git").Test(c, func(f *fixtures.Fixture) {
		for _, t := range objectTypes {
			fs := f.DotGit()
			o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())

			iter, err := o.IterEncodedObjects(t)
			c.Assert(err, IsNil)

			err = iter.ForEach(func(o plumbing.EncodedObject) error {
				c.Assert(o.Type(), Equals, t)
				return nil
			})

			c.Assert(err, IsNil)
		}

	})
}

func (s *FsSuite) TestPackfileIter(c *C) {
	fixtures.ByTag(".git").Test(c, func(f *fixtures.Fixture) {
		fs := f.DotGit()
		dg := dotgit.New(fs)

		for _, t := range objectTypes {
			ph, err := dg.ObjectPacks()
			c.Assert(err, IsNil)

			for _, h := range ph {
				f, err := dg.ObjectPack(h)
				c.Assert(err, IsNil)

				idxf, err := dg.ObjectPackIdx(h)
				c.Assert(err, IsNil)

				iter, err := NewPackfileIter(fs, f, idxf, t, false)
				c.Assert(err, IsNil)

				err = iter.ForEach(func(o plumbing.EncodedObject) error {
					c.Assert(o.Type(), Equals, t)
					return nil
				})
				c.Assert(err, IsNil)
			}
		}
	})
}

func copyFile(c *C, dstDir, dstFilename string, srcFile *os.File) {
	_, err := srcFile.Seek(0, 0)
	c.Assert(err, IsNil)

	err = os.MkdirAll(dstDir, 0750|os.ModeDir)
	c.Assert(err, IsNil)

	dst, err := os.OpenFile(filepath.Join(dstDir, dstFilename), os.O_CREATE|os.O_WRONLY, 0666)
	c.Assert(err, IsNil)
	defer dst.Close()

	_, err = io.Copy(dst, srcFile)
	c.Assert(err, IsNil)
}

// TestPackfileReindex tests that externally-added packfiles are considered by go-git
// after calling the Reindex method
func (s *FsSuite) TestPackfileReindex(c *C) {
	// obtain a standalone packfile that is not part of any other repository
	// in the fixtures:
	packFixture := fixtures.ByTag("packfile").ByTag("standalone").One()
	packFile := packFixture.Packfile()
	idxFile := packFixture.Idx()
	packFilename := packFixture.PackfileHash.String()
	testObjectHash := plumbing.NewHash("a771b1e94141480861332fd0e4684d33071306c6") // this is an object we know exists in the standalone packfile
	fixtures.ByTag(".git").Test(c, func(f *fixtures.Fixture) {
		fs := f.DotGit()
		storer := NewStorage(fs, cache.NewObjectLRUDefault())

		// check that our test object is NOT found
		_, err := storer.EncodedObject(plumbing.CommitObject, testObjectHash)
		c.Assert(err, Equals, plumbing.ErrObjectNotFound)

		// add the external packfile+idx to the packs folder
		// this simulates a git bundle unbundle command, or a repack, for example.
		copyFile(c, filepath.Join(storer.Filesystem().Root(), "objects", "pack"),
			fmt.Sprintf("pack-%s.pack", packFilename), packFile)
		copyFile(c, filepath.Join(storer.Filesystem().Root(), "objects", "pack"),
			fmt.Sprintf("pack-%s.idx", packFilename), idxFile)

		// check that we cannot still retrieve the test object
		_, err = storer.EncodedObject(plumbing.CommitObject, testObjectHash)
		c.Assert(err, Equals, plumbing.ErrObjectNotFound)

		storer.Reindex() // actually reindex

		// Now check that the test object can be retrieved
		_, err = storer.EncodedObject(plumbing.CommitObject, testObjectHash)
		c.Assert(err, IsNil)

	})
}

func (s *FsSuite) TestPackfileIterKeepDescriptors(c *C) {
	fixtures.ByTag(".git").Test(c, func(f *fixtures.Fixture) {
		fs := f.DotGit()
		ops := dotgit.Options{KeepDescriptors: true}
		dg := dotgit.NewWithOptions(fs, ops)

		for _, t := range objectTypes {
			ph, err := dg.ObjectPacks()
			c.Assert(err, IsNil)

			for _, h := range ph {
				f, err := dg.ObjectPack(h)
				c.Assert(err, IsNil)

				idxf, err := dg.ObjectPackIdx(h)
				c.Assert(err, IsNil)

				iter, err := NewPackfileIter(fs, f, idxf, t, true)
				c.Assert(err, IsNil)

				err = iter.ForEach(func(o plumbing.EncodedObject) error {
					c.Assert(o.Type(), Equals, t)
					return nil
				})
				c.Assert(err, IsNil)

				// test twice to check that packfiles are not closed
				err = iter.ForEach(func(o plumbing.EncodedObject) error {
					c.Assert(o.Type(), Equals, t)
					return nil
				})
				c.Assert(err, IsNil)
			}
		}
	})
}

func (s *FsSuite) TestGetFromObjectFileSharedCache(c *C) {
	f1 := fixtures.ByTag("worktree").One().DotGit()
	f2 := fixtures.ByTag("worktree").ByTag("submodule").One().DotGit()

	ch := cache.NewObjectLRUDefault()
	o1 := NewObjectStorage(dotgit.New(f1), ch)
	o2 := NewObjectStorage(dotgit.New(f2), ch)

	expected := plumbing.NewHash("af2d6a6954d532f8ffb47615169c8fdf9d383a1a")
	obj, err := o1.EncodedObject(plumbing.CommitObject, expected)
	c.Assert(err, IsNil)
	c.Assert(obj.Hash(), Equals, expected)

	obj, err = o2.EncodedObject(plumbing.CommitObject, expected)
	c.Assert(err, Equals, plumbing.ErrObjectNotFound)
}

func BenchmarkPackfileIter(b *testing.B) {
	if err := fixtures.Init(); err != nil {
		b.Fatal(err)
	}

	defer func() {
		if err := fixtures.Clean(); err != nil {
			b.Fatal(err)
		}
	}()

	for _, f := range fixtures.ByTag(".git") {
		b.Run(f.URL, func(b *testing.B) {
			fs := f.DotGit()
			dg := dotgit.New(fs)

			for i := 0; i < b.N; i++ {
				for _, t := range objectTypes {
					ph, err := dg.ObjectPacks()
					if err != nil {
						b.Fatal(err)
					}

					for _, h := range ph {
						f, err := dg.ObjectPack(h)
						if err != nil {
							b.Fatal(err)
						}

						idxf, err := dg.ObjectPackIdx(h)
						if err != nil {
							b.Fatal(err)
						}

						iter, err := NewPackfileIter(fs, f, idxf, t, false)
						if err != nil {
							b.Fatal(err)
						}

						err = iter.ForEach(func(o plumbing.EncodedObject) error {
							if o.Type() != t {
								b.Errorf("expecting %s, got %s", t, o.Type())
							}
							return nil
						})

						if err != nil {
							b.Fatal(err)
						}
					}
				}
			}
		})
	}
}

func BenchmarkPackfileIterReadContent(b *testing.B) {
	if err := fixtures.Init(); err != nil {
		b.Fatal(err)
	}

	defer func() {
		if err := fixtures.Clean(); err != nil {
			b.Fatal(err)
		}
	}()

	for _, f := range fixtures.ByTag(".git") {
		b.Run(f.URL, func(b *testing.B) {
			fs := f.DotGit()
			dg := dotgit.New(fs)

			for i := 0; i < b.N; i++ {
				for _, t := range objectTypes {
					ph, err := dg.ObjectPacks()
					if err != nil {
						b.Fatal(err)
					}

					for _, h := range ph {
						f, err := dg.ObjectPack(h)
						if err != nil {
							b.Fatal(err)
						}

						idxf, err := dg.ObjectPackIdx(h)
						if err != nil {
							b.Fatal(err)
						}

						iter, err := NewPackfileIter(fs, f, idxf, t, false)
						if err != nil {
							b.Fatal(err)
						}

						err = iter.ForEach(func(o plumbing.EncodedObject) error {
							if o.Type() != t {
								b.Errorf("expecting %s, got %s", t, o.Type())
							}

							r, err := o.Reader()
							if err != nil {
								b.Fatal(err)
							}

							if _, err := ioutil.ReadAll(r); err != nil {
								b.Fatal(err)
							}

							return r.Close()
						})

						if err != nil {
							b.Fatal(err)
						}
					}
				}
			}
		})
	}
}

func BenchmarkGetObjectFromPackfile(b *testing.B) {
	if err := fixtures.Init(); err != nil {
		b.Fatal(err)
	}

	defer func() {
		if err := fixtures.Clean(); err != nil {
			b.Fatal(err)
		}
	}()

	for _, f := range fixtures.Basic() {
		b.Run(f.URL, func(b *testing.B) {
			fs := f.DotGit()
			o := NewObjectStorage(dotgit.New(fs), cache.NewObjectLRUDefault())
			for i := 0; i < b.N; i++ {
				expected := plumbing.NewHash("6ecf0ef2c2dffb796033e5a02219af86ec6584e5")
				obj, err := o.EncodedObject(plumbing.AnyObject, expected)
				if err != nil {
					b.Fatal(err)
				}

				if obj.Hash() != expected {
					b.Errorf("expecting %s, got %s", expected, obj.Hash())
				}
			}
		})
	}
}
