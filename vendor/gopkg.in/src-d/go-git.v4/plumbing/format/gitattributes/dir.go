package gitattributes

import (
	"os"
	"os/user"

	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/format/config"
	gioutil "gopkg.in/src-d/go-git.v4/utils/ioutil"
)

const (
	coreSection       = "core"
	attributesfile    = "attributesfile"
	gitDir            = ".git"
	gitattributesFile = ".gitattributes"
	gitconfigFile     = ".gitconfig"
	systemFile        = "/etc/gitconfig"
)

func ReadAttributesFile(fs billy.Filesystem, path []string, attributesFile string, allowMacro bool) ([]MatchAttribute, error) {
	f, err := fs.Open(fs.Join(append(path, attributesFile)...))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return ReadAttributes(f, path, allowMacro)
}

// ReadPatterns reads gitattributes patterns recursively through the directory
// structure. The result is in ascending order of priority (last higher).
//
// The .gitattribute file in the root directory will allow custom macro
// definitions. Custom macro definitions in other directories .gitattributes
// will return an error.
func ReadPatterns(fs billy.Filesystem, path []string) (attributes []MatchAttribute, err error) {
	attributes, err = ReadAttributesFile(fs, path, gitattributesFile, true)
	if err != nil {
		return
	}

	attrs, err := walkDirectory(fs, path)
	return append(attributes, attrs...), err
}

func walkDirectory(fs billy.Filesystem, root []string) (attributes []MatchAttribute, err error) {
	fis, err := fs.ReadDir(fs.Join(root...))
	if err != nil {
		return attributes, err
	}

	for _, fi := range fis {
		if !fi.IsDir() || fi.Name() == ".git" {
			continue
		}

		path := append(root, fi.Name())

		dirAttributes, err := ReadAttributesFile(fs, path, gitattributesFile, false)
		if err != nil {
			return attributes, err
		}

		subAttributes, err := walkDirectory(fs, path)
		if err != nil {
			return attributes, err
		}

		attributes = append(attributes, append(dirAttributes, subAttributes...)...)
	}

	return
}

func loadPatterns(fs billy.Filesystem, path string) ([]MatchAttribute, error) {
	f, err := fs.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer gioutil.CheckClose(f, &err)

	raw := config.New()
	if err = config.NewDecoder(f).Decode(raw); err != nil {
		return nil, nil
	}

	path = raw.Section(coreSection).Options.Get(attributesfile)
	if path == "" {
		return nil, nil
	}

	return ReadAttributesFile(fs, nil, path, true)
}

// LoadGlobalPatterns loads gitattributes patterns and attributes from the
// gitattributes file declared in a user's ~/.gitconfig file.  If the
// ~/.gitconfig file does not exist the function will return nil. If the
// core.attributesFile property is not declared, the function will return nil.
// If the file pointed to by the core.attributesfile property does not exist,
// the function will return nil. The function assumes fs is rooted at the root
// filesystem.
func LoadGlobalPatterns(fs billy.Filesystem) (attributes []MatchAttribute, err error) {
	usr, err := user.Current()
	if err != nil {
		return
	}

	return loadPatterns(fs, fs.Join(usr.HomeDir, gitconfigFile))
}

// LoadSystemPatterns loads gitattributes patterns and attributes from the
// gitattributes file declared in a system's /etc/gitconfig file.  If the
// /etc/gitconfig file does not exist the function will return nil. If the
// core.attributesfile property is not declared, the function will return nil.
// If the file pointed to by the core.attributesfile property does not exist,
// the function will return nil. The function assumes fs is rooted at the root
// filesystem.
func LoadSystemPatterns(fs billy.Filesystem) (attributes []MatchAttribute, err error) {
	return loadPatterns(fs, systemFile)
}
