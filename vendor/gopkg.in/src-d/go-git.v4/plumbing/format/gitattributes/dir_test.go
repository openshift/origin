package gitattributes

import (
	"os"
	"os/user"
	"strconv"

	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
)

type MatcherSuite struct {
	GFS  billy.Filesystem // git repository root
	RFS  billy.Filesystem // root that contains user home
	MCFS billy.Filesystem // root that contains user home, but missing ~/.gitattributes
	MEFS billy.Filesystem // root that contains user home, but missing attributesfile entry
	MIFS billy.Filesystem // root that contains user home, but missing .gitattributes

	SFS billy.Filesystem // root that contains /etc/gitattributes
}

var _ = Suite(&MatcherSuite{})

func (s *MatcherSuite) SetUpTest(c *C) {
	// setup root that contains user home
	usr, err := user.Current()
	c.Assert(err, IsNil)

	gitAttributesGlobal := func(fs billy.Filesystem, filename string) {
		f, err := fs.Create(filename)
		c.Assert(err, IsNil)
		_, err = f.Write([]byte("# IntelliJ\n"))
		c.Assert(err, IsNil)
		_, err = f.Write([]byte(".idea/** text\n"))
		c.Assert(err, IsNil)
		_, err = f.Write([]byte("*.iml -text\n"))
		c.Assert(err, IsNil)
		err = f.Close()
		c.Assert(err, IsNil)
	}

	// setup generic git repository root
	fs := memfs.New()
	f, err := fs.Create(".gitattributes")
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("vendor/g*/** foo=bar\n"))
	c.Assert(err, IsNil)
	err = f.Close()
	c.Assert(err, IsNil)

	err = fs.MkdirAll("vendor", os.ModePerm)
	c.Assert(err, IsNil)
	f, err = fs.Create("vendor/.gitattributes")
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("github.com/** -foo\n"))
	c.Assert(err, IsNil)
	err = f.Close()
	c.Assert(err, IsNil)

	fs.MkdirAll("another", os.ModePerm)
	fs.MkdirAll("vendor/github.com", os.ModePerm)
	fs.MkdirAll("vendor/gopkg.in", os.ModePerm)

	gitAttributesGlobal(fs, fs.Join(usr.HomeDir, ".gitattributes_global"))

	s.GFS = fs

	fs = memfs.New()
	err = fs.MkdirAll(usr.HomeDir, os.ModePerm)
	c.Assert(err, IsNil)

	f, err = fs.Create(fs.Join(usr.HomeDir, gitconfigFile))
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("[core]\n"))
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("	attributesfile = " + strconv.Quote(fs.Join(usr.HomeDir, ".gitattributes_global")) + "\n"))
	c.Assert(err, IsNil)
	err = f.Close()
	c.Assert(err, IsNil)

	gitAttributesGlobal(fs, fs.Join(usr.HomeDir, ".gitattributes_global"))

	s.RFS = fs

	// root that contains user home, but missing ~/.gitconfig
	fs = memfs.New()
	gitAttributesGlobal(fs, fs.Join(usr.HomeDir, ".gitattributes_global"))

	s.MCFS = fs

	// setup root that contains user home, but missing attributesfile entry
	fs = memfs.New()
	err = fs.MkdirAll(usr.HomeDir, os.ModePerm)
	c.Assert(err, IsNil)

	f, err = fs.Create(fs.Join(usr.HomeDir, gitconfigFile))
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("[core]\n"))
	c.Assert(err, IsNil)
	err = f.Close()
	c.Assert(err, IsNil)

	gitAttributesGlobal(fs, fs.Join(usr.HomeDir, ".gitattributes_global"))

	s.MEFS = fs

	// setup root that contains user home, but missing .gitattributes
	fs = memfs.New()
	err = fs.MkdirAll(usr.HomeDir, os.ModePerm)
	c.Assert(err, IsNil)

	f, err = fs.Create(fs.Join(usr.HomeDir, gitconfigFile))
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("[core]\n"))
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("	attributesfile = " + strconv.Quote(fs.Join(usr.HomeDir, ".gitattributes_global")) + "\n"))
	c.Assert(err, IsNil)
	err = f.Close()
	c.Assert(err, IsNil)

	s.MIFS = fs

	// setup root that contains user home
	fs = memfs.New()
	err = fs.MkdirAll("etc", os.ModePerm)
	c.Assert(err, IsNil)

	f, err = fs.Create(systemFile)
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("[core]\n"))
	c.Assert(err, IsNil)
	_, err = f.Write([]byte("	attributesfile = /etc/gitattributes_global\n"))
	c.Assert(err, IsNil)
	err = f.Close()
	c.Assert(err, IsNil)

	gitAttributesGlobal(fs, "/etc/gitattributes_global")

	s.SFS = fs
}

func (s *MatcherSuite) TestDir_ReadPatterns(c *C) {
	ps, err := ReadPatterns(s.GFS, nil)
	c.Assert(err, IsNil)
	c.Assert(ps, HasLen, 2)

	m := NewMatcher(ps)
	results, _ := m.Match([]string{"vendor", "gopkg.in", "file"}, nil)
	c.Assert(results["foo"].Value(), Equals, "bar")

	results, _ = m.Match([]string{"vendor", "github.com", "file"}, nil)
	c.Assert(results["foo"].IsUnset(), Equals, false)
}

func (s *MatcherSuite) TestDir_LoadGlobalPatterns(c *C) {
	ps, err := LoadGlobalPatterns(s.RFS)
	c.Assert(err, IsNil)
	c.Assert(ps, HasLen, 2)

	m := NewMatcher(ps)

	results, _ := m.Match([]string{"go-git.v4.iml"}, nil)
	c.Assert(results["text"].IsUnset(), Equals, true)

	results, _ = m.Match([]string{".idea", "file"}, nil)
	c.Assert(results["text"].IsSet(), Equals, true)
}

func (s *MatcherSuite) TestDir_LoadGlobalPatternsMissingGitconfig(c *C) {
	ps, err := LoadGlobalPatterns(s.MCFS)
	c.Assert(err, IsNil)
	c.Assert(ps, HasLen, 0)
}

func (s *MatcherSuite) TestDir_LoadGlobalPatternsMissingAttributesfile(c *C) {
	ps, err := LoadGlobalPatterns(s.MEFS)
	c.Assert(err, IsNil)
	c.Assert(ps, HasLen, 0)
}

func (s *MatcherSuite) TestDir_LoadGlobalPatternsMissingGitattributes(c *C) {
	ps, err := LoadGlobalPatterns(s.MIFS)
	c.Assert(err, IsNil)
	c.Assert(ps, HasLen, 0)
}

func (s *MatcherSuite) TestDir_LoadSystemPatterns(c *C) {
	ps, err := LoadSystemPatterns(s.SFS)
	c.Assert(err, IsNil)
	c.Assert(ps, HasLen, 2)

	m := NewMatcher(ps)
	results, _ := m.Match([]string{"go-git.v4.iml"}, nil)
	c.Assert(results["text"].IsUnset(), Equals, true)

	results, _ = m.Match([]string{".idea", "file"}, nil)
	c.Assert(results["text"].IsSet(), Equals, true)
}
