package ignore

import (
	"bufio"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

type Matcher interface {
	Match(path string, isDir bool) bool
}

type matcher struct {
	m gitignore.Matcher
}

func (m matcher) Match(path string, isDir bool) bool {
	return m.m.Match(strings.Split(path, string(filepath.Separator)), isDir)
}

func NewMatcher(root fs.FS, ignoreFile string) (Matcher, error) {
	patterns := []gitignore.Pattern{}
	if err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() != ignoreFile || d.IsDir() {
			return nil
		}
		ps, err := loadPatterns(root, path)
		if err != nil {
			return err
		}
		patterns = append(patterns, ps...)
		return nil
	}); err != nil {
		return nil, err
	}
	return &matcher{gitignore.NewMatcher(patterns)}, nil
}

func loadPatterns(root fs.FS, path string) ([]gitignore.Pattern, error) {
	domain := strings.Split(filepath.Dir(path), string(filepath.Separator))
	if len(domain) == 1 && domain[0] == "." {
		domain = []string{}
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	patterns := []gitignore.Pattern{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, domain))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return patterns, nil
}
