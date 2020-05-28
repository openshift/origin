package revlist

import (
	"testing"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"

	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git-fixtures.v3"
)

func Test(t *testing.T) { TestingT(t) }

type RevListSuite struct {
	fixtures.Suite
	Storer storer.EncodedObjectStorer
}

var _ = Suite(&RevListSuite{})

const (
	initialCommit = "b029517f6300c2da0f4b651b8642506cd6aaf45d"
	secondCommit  = "b8e471f58bcbca63b07bda20e428190409c2db47"

	someCommit            = "918c48b83bd081e863dbe1b80f8998f058cd8294"
	someCommitBranch      = "e8d3ffab552895c19b9fcf7aa264d277cde33881"
	someCommitOtherBranch = "6ecf0ef2c2dffb796033e5a02219af86ec6584e5"
)

// Created using: git log --graph --oneline --all
//
// Basic fixture repository commits tree:
//
// * 6ecf0ef vendor stuff
// | * e8d3ffa some code in a branch
// |/
// * 918c48b some code
// * af2d6a6 some json
// *   1669dce Merge branch 'master'
// |\
// | *   a5b8b09 Merge pull request #1
// | |\
// | | * b8e471f Creating changelog
// | |/
// * | 35e8510 binary file
// |/
// * b029517 Initial commit

func (s *RevListSuite) SetUpTest(c *C) {
	s.Suite.SetUpSuite(c)
	sto := filesystem.NewStorage(fixtures.Basic().One().DotGit(), cache.NewObjectLRUDefault())
	s.Storer = sto
}

func (s *RevListSuite) commit(c *C, h plumbing.Hash) *object.Commit {
	commit, err := object.GetCommit(s.Storer, h)
	c.Assert(err, IsNil)
	return commit
}

func (s *RevListSuite) TestRevListObjects_Submodules(c *C) {
	submodules := map[string]bool{
		"6ecf0ef2c2dffb796033e5a02219af86ec6584e5": true,
	}

	sto := filesystem.NewStorage(fixtures.ByTag("submodule").One().DotGit(), cache.NewObjectLRUDefault())

	ref, err := storer.ResolveReference(sto, plumbing.HEAD)
	c.Assert(err, IsNil)

	revList, err := Objects(sto, []plumbing.Hash{ref.Hash()}, nil)
	c.Assert(err, IsNil)
	for _, h := range revList {
		c.Assert(submodules[h.String()], Equals, false)
	}
}

// ---
// | |\
// | | * b8e471f Creating changelog
// | |/
// * | 35e8510 binary file
// |/
// * b029517 Initial commit
func (s *RevListSuite) TestRevListObjects(c *C) {
	revList := map[string]bool{
		"b8e471f58bcbca63b07bda20e428190409c2db47": true, // second commit
		"c2d30fa8ef288618f65f6eed6e168e0d514886f4": true, // init tree
		"d3ff53e0564a9f87d8e84b6e28e5060e517008aa": true, // CHANGELOG
	}

	localHist, err := Objects(s.Storer,
		[]plumbing.Hash{plumbing.NewHash(initialCommit)}, nil)
	c.Assert(err, IsNil)

	remoteHist, err := Objects(s.Storer,
		[]plumbing.Hash{plumbing.NewHash(secondCommit)}, localHist)
	c.Assert(err, IsNil)

	for _, h := range remoteHist {
		c.Assert(revList[h.String()], Equals, true)
	}
	c.Assert(len(remoteHist), Equals, len(revList))
}

func (s *RevListSuite) TestRevListObjectsTagObject(c *C) {
	sto := filesystem.NewStorage(
		fixtures.ByTag("tags").
			ByURL("https://github.com/git-fixtures/tags.git").One().DotGit(), cache.NewObjectLRUDefault())

	expected := map[string]bool{
		"70846e9a10ef7b41064b40f07713d5b8b9a8fc73": true,
		"e69de29bb2d1d6434b8b29ae775ad8c2e48c5391": true,
		"ad7897c0fb8e7d9a9ba41fa66072cf06095a6cfc": true,
		"f7b877701fbf855b44c0a9e86f3fdce2c298b07f": true,
	}

	hist, err := Objects(sto, []plumbing.Hash{plumbing.NewHash("ad7897c0fb8e7d9a9ba41fa66072cf06095a6cfc")}, nil)
	c.Assert(err, IsNil)

	for _, h := range hist {
		c.Assert(expected[h.String()], Equals, true)
	}

	c.Assert(len(hist), Equals, len(expected))
}

func (s *RevListSuite) TestRevListObjectsWithStorageForIgnores(c *C) {
	sto := filesystem.NewStorage(
		fixtures.ByTag("merge-conflict").One().DotGit(),
		cache.NewObjectLRUDefault())

	// The "merge-conflict" repo has one extra commit in it, with a
	// two files modified in two different subdirs.
	expected := map[string]bool{
		"1980fcf55330d9d94c34abee5ab734afecf96aba": true, // commit
		"73d9cf44e9045254346c73f6646b08f9302c8570": true, // root dir
		"e8435d512a98586bd2e4fcfcdf04101b0bb1b500": true, // go/
		"257cc5642cb1a054f08cc83f2d943e56fd3ebe99": true, // haskal.hs
		"d499a1a0b79b7d87a35155afd0c1cce78b37a91c": true, // example.go
		"d108adc364fb6f21395d011ae2c8a11d96905b0d": true, // haskal/
	}

	hist, err := ObjectsWithStorageForIgnores(sto, s.Storer, []plumbing.Hash{plumbing.NewHash("1980fcf55330d9d94c34abee5ab734afecf96aba")}, []plumbing.Hash{plumbing.NewHash("6ecf0ef2c2dffb796033e5a02219af86ec6584e5")})
	c.Assert(err, IsNil)

	for _, h := range hist {
		c.Assert(expected[h.String()], Equals, true)
	}

	c.Assert(len(hist), Equals, len(expected))
}

// ---
// | |\
// | | * b8e471f Creating changelog
// | |/
// * | 35e8510 binary file
// |/
// * b029517 Initial commit
func (s *RevListSuite) TestRevListObjectsWithBlobsAndTrees(c *C) {
	revList := map[string]bool{
		"b8e471f58bcbca63b07bda20e428190409c2db47": true, // second commit
	}

	localHist, err := Objects(s.Storer,
		[]plumbing.Hash{
			plumbing.NewHash(initialCommit),
			plumbing.NewHash("c2d30fa8ef288618f65f6eed6e168e0d514886f4"),
			plumbing.NewHash("d3ff53e0564a9f87d8e84b6e28e5060e517008aa"),
		}, nil)
	c.Assert(err, IsNil)

	remoteHist, err := Objects(s.Storer,
		[]plumbing.Hash{plumbing.NewHash(secondCommit)}, localHist)
	c.Assert(err, IsNil)

	for _, h := range remoteHist {
		c.Assert(revList[h.String()], Equals, true)
	}
	c.Assert(len(remoteHist), Equals, len(revList))
}

func (s *RevListSuite) TestRevListObjectsReverse(c *C) {

	localHist, err := Objects(s.Storer,
		[]plumbing.Hash{plumbing.NewHash(secondCommit)}, nil)
	c.Assert(err, IsNil)

	remoteHist, err := Objects(s.Storer,
		[]plumbing.Hash{plumbing.NewHash(initialCommit)}, localHist)
	c.Assert(err, IsNil)

	c.Assert(len(remoteHist), Equals, 0)
}

func (s *RevListSuite) TestRevListObjectsSameCommit(c *C) {
	localHist, err := Objects(s.Storer,
		[]plumbing.Hash{plumbing.NewHash(secondCommit)}, nil)
	c.Assert(err, IsNil)

	remoteHist, err := Objects(s.Storer,
		[]plumbing.Hash{plumbing.NewHash(secondCommit)}, localHist)
	c.Assert(err, IsNil)

	c.Assert(len(remoteHist), Equals, 0)
}

// * 6ecf0ef vendor stuff
// | * e8d3ffa some code in a branch
// |/
// * 918c48b some code
// -----
func (s *RevListSuite) TestRevListObjectsNewBranch(c *C) {
	localHist, err := Objects(s.Storer,
		[]plumbing.Hash{plumbing.NewHash(someCommit)}, nil)
	c.Assert(err, IsNil)

	remoteHist, err := Objects(
		s.Storer, []plumbing.Hash{
			plumbing.NewHash(someCommitBranch),
			plumbing.NewHash(someCommitOtherBranch)}, localHist)
	c.Assert(err, IsNil)

	revList := map[string]bool{
		"a8d315b2b1c615d43042c3a62402b8a54288cf5c": true, // init tree
		"cf4aa3b38974fb7d81f367c0830f7d78d65ab86b": true, // vendor folder
		"9dea2395f5403188298c1dabe8bdafe562c491e3": true, // foo.go
		"e8d3ffab552895c19b9fcf7aa264d277cde33881": true, // branch commit
		"dbd3641b371024f44d0e469a9c8f5457b0660de1": true, // init tree
		"7e59600739c96546163833214c36459e324bad0a": true, // README
		"6ecf0ef2c2dffb796033e5a02219af86ec6584e5": true, // otherBranch commit
	}

	for _, h := range remoteHist {
		c.Assert(revList[h.String()], Equals, true)
	}
	c.Assert(len(remoteHist), Equals, len(revList))
}

// This tests will ensure that a5b8b09 and b8e471f will be visited even if
// 35e8510 has already been visited and will not stop iterating until they
// have been as well.
//
// * af2d6a6 some json
// *   1669dce Merge branch 'master'
// |\
// | *   a5b8b09 Merge pull request #1
// | |\
// | | * b8e471f Creating changelog
// | |/
// * | 35e8510 binary file
// |/
// * b029517 Initial commit
func (s *RevListSuite) TestReachableObjectsNoRevisit(c *C) {
	obj, err := s.Storer.EncodedObject(plumbing.CommitObject, plumbing.NewHash("af2d6a6954d532f8ffb47615169c8fdf9d383a1a"))
	c.Assert(err, IsNil)

	do, err := object.DecodeObject(s.Storer, obj)
	c.Assert(err, IsNil)

	commit, ok := do.(*object.Commit)
	c.Assert(ok, Equals, true)

	var visited []plumbing.Hash
	err = reachableObjects(
		commit,
		map[plumbing.Hash]bool{
			plumbing.NewHash("35e85108805c84807bc66a02d91535e1e24b38b9"): true,
		},
		map[plumbing.Hash]bool{
			plumbing.NewHash("35e85108805c84807bc66a02d91535e1e24b38b9"): true,
		},
		nil,
		func(h plumbing.Hash) {
			obj, err := s.Storer.EncodedObject(plumbing.AnyObject, h)
			c.Assert(err, IsNil)

			do, err := object.DecodeObject(s.Storer, obj)
			c.Assert(err, IsNil)

			if _, ok := do.(*object.Commit); ok {
				visited = append(visited, h)
			}
		},
	)
	c.Assert(err, IsNil)

	c.Assert(visited, DeepEquals, []plumbing.Hash{
		plumbing.NewHash("af2d6a6954d532f8ffb47615169c8fdf9d383a1a"),
		plumbing.NewHash("1669dce138d9b841a518c64b10914d88f5e488ea"),
		plumbing.NewHash("a5b8b09e2f8fcb0bb99d3ccb0958157b40890d69"),
		plumbing.NewHash("b029517f6300c2da0f4b651b8642506cd6aaf45d"),
		plumbing.NewHash("b8e471f58bcbca63b07bda20e428190409c2db47"),
	})
}
