package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
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

type Package struct {
	Dir         string
	ImportPath  string
	Imports     []string
	TestImports []string
}

type PackageList struct {
	Packages []Package
}

func (p *PackageList) Add(pkg Package) {
	p.Packages = append(p.Packages, pkg)
}

// getPackageMetadata receives a set of go import paths and execs "go list"
// using each path as an entrypoint.
// Returns a PackageList containing dependency and importPath data for each package.
func getPackageMetadata(entrypoints []string) (*PackageList, error) {
	args := []string{"list", "--json"}
	golist := exec.Command("go", append(args, entrypoints...)...)

	r, w := io.Pipe()
	golist.Stdout = w
	golist.Stderr = os.Stderr

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

func isValidPackagePath(path string) bool {
	return baseRepoRegex.Match([]byte(path))
}
