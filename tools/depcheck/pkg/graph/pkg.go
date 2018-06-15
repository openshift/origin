package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var (
	// Matches standard goimport format for a package.
	//
	// The following formats will successfully match a valid import path:
	//   - host.tld/repo/pkg
	//   - foo.bar/baz
	//
	// The following formats will fail to match an import path:
	//   - company.com
	//   - company/missing/tld
	//   - fmt
	//   - encoding/json
	baseRepoRegex = regexp.MustCompile("[a-zA-Z0-9]+\\.([a-z0-9])+\\/.+")
)

type PackageError struct {
	ImportStack []string
	Pos         string
	Err         string
}

func (e *PackageError) Error() string {
	return e.Err
}

type Package struct {
	Dir         string
	ImportPath  string
	Imports     []string
	TestImports []string
	Error       *PackageError
}

type PackageList struct {
	Packages []Package
}

func (p *PackageList) Add(pkg Package) {
	p.Packages = append(p.Packages, pkg)
}

// getPackageMetadata receives a set of go import paths and execs "go list"
// using each path as an entrypoint.
// Any errors that occur for import paths listed in ignoredPaths are non-fatal.
// Any errors that occur for any other import paths will result in a fatal error.
// Returns a PackageList containing dependency and importPath data for each package.
func getPackageMetadata(entrypoints, ignoredPaths, buildTags []string) (*PackageList, error) {
	args := []string{"list", "-e", "--json"}
	if len(buildTags) > 0 {
		args = append(args, append([]string{"--tags"}, buildTags...)...)
	}

	golist := exec.Command("go", append(args, entrypoints...)...)

	r, w := io.Pipe()
	golist.Stdout = w
	golist.Stderr = os.Stderr
	defer r.Close()

	done := make(chan bool)

	pkgs := &PackageList{}
	go func(list *PackageList) {
		decoder := json.NewDecoder(r)
		for {
			var pkg Package
			err := decoder.Decode(&pkg)
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				continue
			}

			// handle errors
			if pkg.Error != nil {
				if containsPrefix(pkg.ImportPath, ignoredPaths) {
					fmt.Fprintf(os.Stderr, "warning: error encountered on excluded path %s: %v\n", pkg.ImportPath, pkg.Error)
					continue
				}

				fmt.Fprintf(os.Stderr, "error: %v\n", pkg.Error)
				golist.Process.Kill()

				close(done)
				return
			}

			list.Add(pkg)
		}

		close(done)
	}(pkgs)

	if err := golist.Run(); err != nil {
		w.Close()
		return nil, err
	}
	w.Close()

	// wait for the goroutine to finish to ensure that all
	// packages have been parsed and added before returning
	<-done

	return pkgs, nil
}

// containsPrefix returns true if a needle begins
// with at least one of the given prefixes.
func containsPrefix(needle string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(needle, prefix) {
			return true
		}
	}

	return false
}

func isValidPackagePath(path string) bool {
	return baseRepoRegex.Match([]byte(path))
}
