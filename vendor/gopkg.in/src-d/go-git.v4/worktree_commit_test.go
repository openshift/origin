package git

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
	"gopkg.in/src-d/go-git.v4/storage/memory"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/errors"
	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-billy.v4/util"
)

func (s *WorktreeSuite) TestCommitInvalidOptions(c *C) {
	r, err := Init(memory.NewStorage(), memfs.New())
	c.Assert(err, IsNil)

	w, err := r.Worktree()
	c.Assert(err, IsNil)

	hash, err := w.Commit("", &CommitOptions{})
	c.Assert(err, Equals, ErrMissingAuthor)
	c.Assert(hash.IsZero(), Equals, true)
}

func (s *WorktreeSuite) TestCommitInitial(c *C) {
	expected := plumbing.NewHash("98c4ac7c29c913f7461eae06e024dc18e80d23a4")

	fs := memfs.New()
	storage := memory.NewStorage()

	r, err := Init(storage, fs)
	c.Assert(err, IsNil)

	w, err := r.Worktree()
	c.Assert(err, IsNil)

	util.WriteFile(fs, "foo", []byte("foo"), 0644)

	_, err = w.Add("foo")
	c.Assert(err, IsNil)

	hash, err := w.Commit("foo\n", &CommitOptions{Author: defaultSignature()})
	c.Assert(hash, Equals, expected)
	c.Assert(err, IsNil)

	assertStorageStatus(c, r, 1, 1, 1, expected)
}

func (s *WorktreeSuite) TestCommitParent(c *C) {
	expected := plumbing.NewHash("ef3ca05477530b37f48564be33ddd48063fc7a22")

	fs := memfs.New()
	w := &Worktree{
		r:          s.Repository,
		Filesystem: fs,
	}

	err := w.Checkout(&CheckoutOptions{})
	c.Assert(err, IsNil)

	util.WriteFile(fs, "foo", []byte("foo"), 0644)

	_, err = w.Add("foo")
	c.Assert(err, IsNil)

	hash, err := w.Commit("foo\n", &CommitOptions{Author: defaultSignature()})
	c.Assert(hash, Equals, expected)
	c.Assert(err, IsNil)

	assertStorageStatus(c, s.Repository, 13, 11, 10, expected)
}

func (s *WorktreeSuite) TestCommitAll(c *C) {
	expected := plumbing.NewHash("aede6f8c9c1c7ec9ca8d287c64b8ed151276fa28")

	fs := memfs.New()
	w := &Worktree{
		r:          s.Repository,
		Filesystem: fs,
	}

	err := w.Checkout(&CheckoutOptions{})
	c.Assert(err, IsNil)

	util.WriteFile(fs, "LICENSE", []byte("foo"), 0644)
	util.WriteFile(fs, "foo", []byte("foo"), 0644)

	hash, err := w.Commit("foo\n", &CommitOptions{
		All:    true,
		Author: defaultSignature(),
	})

	c.Assert(hash, Equals, expected)
	c.Assert(err, IsNil)

	assertStorageStatus(c, s.Repository, 13, 11, 10, expected)
}

func (s *WorktreeSuite) TestRemoveAndCommitAll(c *C) {
	expected := plumbing.NewHash("907cd576c6ced2ecd3dab34a72bf9cf65944b9a9")

	fs := memfs.New()
	w := &Worktree{
		r:          s.Repository,
		Filesystem: fs,
	}

	err := w.Checkout(&CheckoutOptions{})
	c.Assert(err, IsNil)

	util.WriteFile(fs, "foo", []byte("foo"), 0644)
	_, err = w.Add("foo")
	c.Assert(err, IsNil)

	_, errFirst := w.Commit("Add in Repo\n", &CommitOptions{
		Author: defaultSignature(),
	})
	c.Assert(errFirst, IsNil)

	errRemove := fs.Remove("foo")
	c.Assert(errRemove, IsNil)

	hash, errSecond := w.Commit("Remove foo\n", &CommitOptions{
		All:    true,
		Author: defaultSignature(),
	})
	c.Assert(errSecond, IsNil)

	c.Assert(hash, Equals, expected)
	c.Assert(err, IsNil)

	assertStorageStatus(c, s.Repository, 13, 11, 11, expected)
}

func (s *WorktreeSuite) TestCommitSign(c *C) {
	fs := memfs.New()
	storage := memory.NewStorage()

	r, err := Init(storage, fs)
	c.Assert(err, IsNil)

	w, err := r.Worktree()
	c.Assert(err, IsNil)

	util.WriteFile(fs, "foo", []byte("foo"), 0644)

	_, err = w.Add("foo")
	c.Assert(err, IsNil)

	key := commitSignKey(c, true)
	hash, err := w.Commit("foo\n", &CommitOptions{Author: defaultSignature(), SignKey: key})
	c.Assert(err, IsNil)

	// Verify the commit.
	pks := new(bytes.Buffer)
	pkw, err := armor.Encode(pks, openpgp.PublicKeyType, nil)
	c.Assert(err, IsNil)

	err = key.Serialize(pkw)
	c.Assert(err, IsNil)
	err = pkw.Close()
	c.Assert(err, IsNil)

	expectedCommit, err := r.CommitObject(hash)
	c.Assert(err, IsNil)
	actual, err := expectedCommit.Verify(pks.String())
	c.Assert(err, IsNil)
	c.Assert(actual.PrimaryKey, DeepEquals, key.PrimaryKey)
}

func (s *WorktreeSuite) TestCommitSignBadKey(c *C) {
	fs := memfs.New()
	storage := memory.NewStorage()

	r, err := Init(storage, fs)
	c.Assert(err, IsNil)

	w, err := r.Worktree()
	c.Assert(err, IsNil)

	util.WriteFile(fs, "foo", []byte("foo"), 0644)

	_, err = w.Add("foo")
	c.Assert(err, IsNil)

	key := commitSignKey(c, false)
	_, err = w.Commit("foo\n", &CommitOptions{Author: defaultSignature(), SignKey: key})
	c.Assert(err, Equals, errors.InvalidArgumentError("signing key is encrypted"))
}

func (s *WorktreeSuite) TestCommitTreeSort(c *C) {
	path, err := ioutil.TempDir(os.TempDir(), "test-commit-tree-sort")
	c.Assert(err, IsNil)
	fs := osfs.New(path)
	st := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	r, err := Init(st, nil)
	c.Assert(err, IsNil)

	r, _ = Clone(memory.NewStorage(), memfs.New(), &CloneOptions{
		URL: path,
	})

	w, err := r.Worktree()
	c.Assert(err, IsNil)

	mfs := w.Filesystem

	err = mfs.MkdirAll("delta", 0755)
	c.Assert(err, IsNil)

	for _, p := range []string{"delta_last", "Gamma", "delta/middle", "Beta", "delta-first", "alpha"} {
		util.WriteFile(mfs, p, []byte("foo"), 0644)
		_, err = w.Add(p)
		c.Assert(err, IsNil)
	}

	_, err = w.Commit("foo\n", &CommitOptions{
		All:    true,
		Author: defaultSignature(),
	})
	c.Assert(err, IsNil)

	err = r.Push(&PushOptions{})
	c.Assert(err, IsNil)

	cmd := exec.Command("git", "fsck")
	cmd.Dir = path
	cmd.Env = os.Environ()
	buf := &bytes.Buffer{}
	cmd.Stderr = buf
	cmd.Stdout = buf

	err = cmd.Run()

	c.Assert(err, IsNil, Commentf("%s", buf.Bytes()))
}

func assertStorageStatus(
	c *C, r *Repository,
	treesCount, blobCount, commitCount int, head plumbing.Hash,
) {
	trees, err := r.Storer.IterEncodedObjects(plumbing.TreeObject)
	c.Assert(err, IsNil)
	blobs, err := r.Storer.IterEncodedObjects(plumbing.BlobObject)
	c.Assert(err, IsNil)
	commits, err := r.Storer.IterEncodedObjects(plumbing.CommitObject)
	c.Assert(err, IsNil)

	c.Assert(lenIterEncodedObjects(trees), Equals, treesCount)
	c.Assert(lenIterEncodedObjects(blobs), Equals, blobCount)
	c.Assert(lenIterEncodedObjects(commits), Equals, commitCount)

	ref, err := r.Head()
	c.Assert(err, IsNil)
	c.Assert(ref.Hash(), Equals, head)
}

func lenIterEncodedObjects(iter storer.EncodedObjectIter) int {
	count := 0
	iter.ForEach(func(plumbing.EncodedObject) error {
		count++
		return nil
	})

	return count
}

func defaultSignature() *object.Signature {
	when, _ := time.Parse(object.DateFormat, "Thu May 04 00:03:43 2017 +0200")
	return &object.Signature{
		Name:  "foo",
		Email: "foo@foo.foo",
		When:  when,
	}
}

func commitSignKey(c *C, decrypt bool) *openpgp.Entity {
	s := strings.NewReader(armoredKeyRing)
	es, err := openpgp.ReadArmoredKeyRing(s)
	c.Assert(err, IsNil)

	c.Assert(es, HasLen, 1)
	c.Assert(es[0].Identities, HasLen, 1)
	_, ok := es[0].Identities["foo bar <foo@foo.foo>"]
	c.Assert(ok, Equals, true)

	key := es[0]
	if decrypt {
		err = key.PrivateKey.Decrypt([]byte(keyPassphrase))
		c.Assert(err, IsNil)
	}

	return key
}

const armoredKeyRing = `
-----BEGIN PGP PRIVATE KEY BLOCK-----

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
=gIM9
-----END PGP PRIVATE KEY BLOCK-----
`

const keyPassphrase = "abcdef0123456789"
