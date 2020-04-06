package commitgraph_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	. "gopkg.in/check.v1"
	fixtures "gopkg.in/src-d/go-git-fixtures.v3"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/commitgraph"
)

func Test(t *testing.T) { TestingT(t) }

type CommitgraphSuite struct {
	fixtures.Suite
}

var _ = Suite(&CommitgraphSuite{})

func testDecodeHelper(c *C, path string) {
	reader, err := os.Open(path)
	c.Assert(err, IsNil)
	defer reader.Close()
	index, err := commitgraph.OpenFileIndex(reader)
	c.Assert(err, IsNil)

	// Root commit
	nodeIndex, err := index.GetIndexByHash(plumbing.NewHash("347c91919944a68e9413581a1bc15519550a3afe"))
	c.Assert(err, IsNil)
	commitData, err := index.GetCommitDataByIndex(nodeIndex)
	c.Assert(err, IsNil)
	c.Assert(len(commitData.ParentIndexes), Equals, 0)
	c.Assert(len(commitData.ParentHashes), Equals, 0)

	// Regular commit
	nodeIndex, err = index.GetIndexByHash(plumbing.NewHash("e713b52d7e13807e87a002e812041f248db3f643"))
	c.Assert(err, IsNil)
	commitData, err = index.GetCommitDataByIndex(nodeIndex)
	c.Assert(err, IsNil)
	c.Assert(len(commitData.ParentIndexes), Equals, 1)
	c.Assert(len(commitData.ParentHashes), Equals, 1)
	c.Assert(commitData.ParentHashes[0].String(), Equals, "347c91919944a68e9413581a1bc15519550a3afe")

	// Merge commit
	nodeIndex, err = index.GetIndexByHash(plumbing.NewHash("b29328491a0682c259bcce28741eac71f3499f7d"))
	c.Assert(err, IsNil)
	commitData, err = index.GetCommitDataByIndex(nodeIndex)
	c.Assert(err, IsNil)
	c.Assert(len(commitData.ParentIndexes), Equals, 2)
	c.Assert(len(commitData.ParentHashes), Equals, 2)
	c.Assert(commitData.ParentHashes[0].String(), Equals, "e713b52d7e13807e87a002e812041f248db3f643")
	c.Assert(commitData.ParentHashes[1].String(), Equals, "03d2c021ff68954cf3ef0a36825e194a4b98f981")

	// Octopus merge commit
	nodeIndex, err = index.GetIndexByHash(plumbing.NewHash("6f6c5d2be7852c782be1dd13e36496dd7ad39560"))
	c.Assert(err, IsNil)
	commitData, err = index.GetCommitDataByIndex(nodeIndex)
	c.Assert(err, IsNil)
	c.Assert(len(commitData.ParentIndexes), Equals, 3)
	c.Assert(len(commitData.ParentHashes), Equals, 3)
	c.Assert(commitData.ParentHashes[0].String(), Equals, "ce275064ad67d51e99f026084e20827901a8361c")
	c.Assert(commitData.ParentHashes[1].String(), Equals, "bb13916df33ed23004c3ce9ed3b8487528e655c1")
	c.Assert(commitData.ParentHashes[2].String(), Equals, "a45273fe2d63300e1962a9e26a6b15c276cd7082")

	// Check all hashes
	hashes := index.Hashes()
	c.Assert(len(hashes), Equals, 11)
	c.Assert(hashes[0].String(), Equals, "03d2c021ff68954cf3ef0a36825e194a4b98f981")
	c.Assert(hashes[10].String(), Equals, "e713b52d7e13807e87a002e812041f248db3f643")
}

func (s *CommitgraphSuite) TestDecode(c *C) {
	fixtures.ByTag("commit-graph").Test(c, func(f *fixtures.Fixture) {
		dotgit := f.DotGit()
		testDecodeHelper(c, path.Join(dotgit.Root(), "objects", "info", "commit-graph"))
	})
}

func (s *CommitgraphSuite) TestReencode(c *C) {
	fixtures.ByTag("commit-graph").Test(c, func(f *fixtures.Fixture) {
		dotgit := f.DotGit()

		reader, err := os.Open(path.Join(dotgit.Root(), "objects", "info", "commit-graph"))
		c.Assert(err, IsNil)
		defer reader.Close()
		index, err := commitgraph.OpenFileIndex(reader)
		c.Assert(err, IsNil)

		writer, err := ioutil.TempFile(dotgit.Root(), "commit-graph")
		c.Assert(err, IsNil)
		tmpName := writer.Name()
		defer os.Remove(tmpName)
		encoder := commitgraph.NewEncoder(writer)
		err = encoder.Encode(index)
		c.Assert(err, IsNil)
		writer.Close()

		testDecodeHelper(c, tmpName)
	})
}

func (s *CommitgraphSuite) TestReencodeInMemory(c *C) {
	fixtures.ByTag("commit-graph").Test(c, func(f *fixtures.Fixture) {
		dotgit := f.DotGit()

		reader, err := os.Open(path.Join(dotgit.Root(), "objects", "info", "commit-graph"))
		c.Assert(err, IsNil)
		index, err := commitgraph.OpenFileIndex(reader)
		c.Assert(err, IsNil)
		memoryIndex := commitgraph.NewMemoryIndex()
		for i, hash := range index.Hashes() {
			commitData, err := index.GetCommitDataByIndex(i)
			c.Assert(err, IsNil)
			memoryIndex.Add(hash, commitData)
		}
		reader.Close()

		writer, err := ioutil.TempFile(dotgit.Root(), "commit-graph")
		c.Assert(err, IsNil)
		tmpName := writer.Name()
		defer os.Remove(tmpName)
		encoder := commitgraph.NewEncoder(writer)
		err = encoder.Encode(memoryIndex)
		c.Assert(err, IsNil)
		writer.Close()

		testDecodeHelper(c, tmpName)
	})
}
