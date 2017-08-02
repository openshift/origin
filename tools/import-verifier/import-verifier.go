package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	rootPackage = "github.com/openshift/origin"
)

// Package is a subset of cmd/go.Package
type Package struct {
	ImportPath   string   `json:",omitempty"` // import path of package in dir
	Imports      []string `json:",omitempty"` // import paths used by this package
	TestImports  []string `json:",omitempty"` // imports from TestGoFiles
	XTestImports []string `json:",omitempty"` // imports from XTestGoFiles
}

type ImportRestriction struct {
	// BaseImportPath is the root of the package tree
	// that is restricted by this configuration
	BaseImportPath string `json:"baseImportPath"`
	// IgnoredSubTrees are roots of sub-trees of the
	// BaseImportPath for which we do not want to enforce
	// any import restrictions whatsoever
	IgnoredSubTrees []string `json:"ignoredSubTrees,omitempty"`
	// AllowedImports are roots of package trees that
	// are allowed to be imported from the BaseImportPath
	AllowedImports []string `json:"allowedImports"`
}

// ForbiddenImportsFor determines all of the forbidden
// imports for a package given the import restrictions
func (i *ImportRestriction) ForbiddenImportsFor(pkg Package) []string {
	if !i.isRestrictedPath(pkg.ImportPath) {
		return []string{}
	}

	return i.forbiddenImportsFor(pkg)
}

// isRestrictedPath determines if the import path has
// any restrictions placed on it by this configuration.
// A path will be restricted if:
//   - it falls under the base import path
//   - it does not fall under any of the ignored sub-trees
func (i *ImportRestriction) isRestrictedPath(imp string) bool {
	if !strings.HasPrefix(imp, absolutePackage(i.BaseImportPath)) {
		return false
	}

	for _, ignored := range i.IgnoredSubTrees {
		if strings.HasPrefix(imp, absolutePackage(ignored)) {
			return false
		}
	}

	return true
}

// forbiddenImportsFor determines all of the forbidden
// imports for a package given the import restrictions
// and returns a deduplicated list of them
func (i *ImportRestriction) forbiddenImportsFor(pkg Package) []string {
	forbiddenImportSet := map[string]struct{}{}
	for _, imp := range append(pkg.Imports, append(pkg.TestImports, pkg.XTestImports...)...) {
		if i.isForbidden(imp) {
			forbiddenImportSet[relativePackage(imp)] = struct{}{}
		}
	}

	var forbiddenImports []string
	for imp := range forbiddenImportSet {
		forbiddenImports = append(forbiddenImports, imp)
	}
	return forbiddenImports
}

// isForbidden determines if an import is forbidden,
// which is true when the import is:
//   - of a package under the rootPackage
//   - is not of the base import path or a sub-package of it
//   - is not of an allowed path or a sub-package of one
func (i *ImportRestriction) isForbidden(imp string) bool {
	importsBelowRoot := strings.HasPrefix(imp, rootPackage)
	importsBelowBase := strings.HasPrefix(imp, absolutePackage(i.BaseImportPath))
	importsAllowed := false
	for _, allowed := range i.AllowedImports {
		importsAllowed = importsAllowed || strings.HasPrefix(imp, absolutePackage(allowed))
	}

	return importsBelowRoot && !importsBelowBase && !importsAllowed
}

func absolutePackage(relativePackage string) string {
	return fmt.Sprintf("%s/%s", rootPackage, relativePackage)
}

func relativePackage(absolutePackage string) string {
	return absolutePackage[len(rootPackage)+1:]
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("%s requires the configuration file as it's only argument", os.Args[0])
	}

	configFile := os.Args[1]
	importRestrictions, err := loadImportRestrictions(configFile)
	if err != nil {
		log.Fatalf("Failed to load import restrictions: %v", err)
	}

	foundForbiddenImports := false
	for _, restriction := range importRestrictions {
		log.Printf("Inspecting imports under %s...\n", restriction.BaseImportPath)
		packages, err := resolvePackageTree(absolutePackage(restriction.BaseImportPath))
		if err != nil {
			log.Fatalf("Failed to resolve package tree: %v", err)
		}

		log.Printf("-- validating imports for %d packages in the tree", len(packages))
		for _, pkg := range packages {
			if forbidden := restriction.ForbiddenImportsFor(pkg); len(forbidden) != 0 {
				logForbiddenPackages(relativePackage(pkg.ImportPath), forbidden)
				foundForbiddenImports = true
			}
		}
	}

	if foundForbiddenImports {
		os.Exit(1)
	}
}

func loadImportRestrictions(configFile string) ([]ImportRestriction, error) {
	config, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration from %s: %v", configFile, err)
	}

	var importRestrictions []ImportRestriction
	if err := json.Unmarshal(config, &importRestrictions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal from %s: %v", configFile, err)
	}

	return importRestrictions, nil
}

func resolvePackageTree(treeBase string) ([]Package, error) {
	cmd := "go"
	args := []string{"list", "-json", fmt.Sprintf("%s...", treeBase)}
	stdout, err := exec.Command(cmd, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to run `%s %s`: %v\n", cmd, strings.Join(args, " "), err)
	}

	packages, err := decodePackages(bytes.NewReader(stdout))
	if err != nil {
		return nil, fmt.Errorf("Failed to decode packages: %v", err)
	}

	return packages, nil
}

func decodePackages(r io.Reader) ([]Package, error) {
	// `go list -json` concatenates package definitions
	// instead of emitting a single valid JSON, so we
	// need to stream the output to decode it into the
	// data we are looking for instead of just using a
	// simple JSON decoder on stdout
	var packages []Package
	decoder := json.NewDecoder(r)
	for decoder.More() {
		var pkg Package
		if err := decoder.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("invalid package: %v", err)
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

func logForbiddenPackages(base string, forbidden []string) {
	log.Printf("-- found forbidden imports for %s:\n", base)
	for _, forbiddenPackage := range forbidden {
		log.Printf("\t%s\n", forbiddenPackage)
	}
}
