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
	// CheckedPackageRoots are the roots of the package tree
	// that are restricted by this configuration
	CheckedPackageRoots []string `json:"checkedPackageRoots"`
	// CheckedPackages are the specific packages
	// that are restricted by this configuration
	CheckedPackages []string `json:"checkedPackages"`
	// IgnoredSubTrees are roots of sub-trees of the
	// BaseImportPath for which we do not want to enforce
	// any import restrictions whatsoever
	IgnoredSubTrees []string `json:"ignoredSubTrees,omitempty"`
	// AllowedImportPackages are roots of package trees that
	// are allowed to be imported for this restriction
	AllowedImportPackages []string `json:"allowedImportPackages"`
	// AllowedImportPackageRoots are roots of package trees that
	// are allowed to be imported for this restriction
	AllowedImportPackageRoots []string `json:"allowedImportPackageRoots"`
	// ForbiddenImportPackageRoots are roots of package trees that
	// are NOT allowed to be imported for this restriction
	ForbiddenImportPackageRoots []string `json:"forbiddenImportPackageRoots"`
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
func (i *ImportRestriction) isRestrictedPath(packageToCheck string) bool {
	// if its not under our root, then its a built-in.  Everything else is under
	// github.com/openshift/origin or github.com/openshift/origin/vendor
	if !strings.HasPrefix(packageToCheck, rootPackage) {
		return false
	}

	// some subtrees are specifically excluded.  Not sure if we still need this given
	// explicit inclusion
	for _, ignored := range i.IgnoredSubTrees {
		if strings.HasPrefix(packageToCheck, ignored) {
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
	for _, packageToCheck := range append(pkg.Imports, append(pkg.TestImports, pkg.XTestImports...)...) {
		if !i.isAllowed(packageToCheck) {
			forbiddenImportSet[relativePackage(packageToCheck)] = struct{}{}
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
func (i *ImportRestriction) isAllowed(packageToCheck string) bool {
	// if its not under our root, then its a built-in.  Everything else is under
	// github.com/openshift/origin or github.com/openshift/origin/vendor
	if !strings.HasPrefix(packageToCheck, rootPackage) {
		return true
	}
	if i.isIncludedInRestrictedPackages(packageToCheck) {
		return true
	}

	for _, forbiddenPackageRoot := range i.ForbiddenImportPackageRoots {
		if strings.HasPrefix(forbiddenPackageRoot, "vendor") {
			forbiddenPackageRoot = rootPackage + "/" + forbiddenPackageRoot
		}
		if strings.HasPrefix(packageToCheck, forbiddenPackageRoot) {
			return false
		}
	}
	for _, allowedPackage := range i.AllowedImportPackages {
		if strings.HasPrefix(allowedPackage, "vendor") {
			allowedPackage = rootPackage + "/" + allowedPackage
		}
		if packageToCheck == allowedPackage {
			return true
		}
	}
	for _, allowedPackageRoot := range i.AllowedImportPackageRoots {
		if strings.HasPrefix(allowedPackageRoot, "vendor") {
			allowedPackageRoot = rootPackage + "/" + allowedPackageRoot
		}
		if strings.HasPrefix(packageToCheck, allowedPackageRoot) {
			return true
		}
	}

	return false
}

// isIncludedInRestrictedPackages checks to see if a package is included in the list of packages we're
// restricting.  Any package being restricted is assumed to be allowed to import another package being
// restricted since they are grouped
func (i *ImportRestriction) isIncludedInRestrictedPackages(packageToCheck string) bool {
	// some subtrees are specifically excluded.  Not sure if we still need this given
	// explicit inclusion
	for _, ignored := range i.IgnoredSubTrees {
		if strings.HasPrefix(packageToCheck, ignored) {
			return false
		}
	}

	for _, currBase := range i.CheckedPackageRoots {
		if strings.HasPrefix(packageToCheck, currBase) {
			return true
		}
	}
	for _, currPackageName := range i.CheckedPackages {
		if currPackageName == packageToCheck {
			return true
		}
	}
	return false
}

func relativePackage(absolutePackage string) string {
	if strings.HasPrefix(absolutePackage, rootPackage+"/vendor") {
		return absolutePackage[len(rootPackage)+1:]
	}
	return absolutePackage
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

	failedRestrictionCheck := false
	for _, restriction := range importRestrictions {
		packages := []Package{}
		for _, currBase := range restriction.CheckedPackageRoots {
			log.Printf("Inspecting imports under %s...\n", currBase)
			currPackages, err := resolvePackage(currBase + "/...")
			if err != nil {
				log.Fatalf("Failed to resolve package tree %v: %v", currBase, err)
			}
			packages = mergePackages(packages, currPackages)
		}
		for _, currPackageName := range restriction.CheckedPackages {
			log.Printf("Inspecting imports at %s...\n", currPackageName)
			currPackages, err := resolvePackage(currPackageName)
			if err != nil {
				log.Fatalf("Failed to resolve package %v: %v", currPackageName, err)
			}
			packages = mergePackages(packages, currPackages)
		}

		if len(packages) == 0 {
			log.Fatalf("No packages found")
		}
		log.Printf("-- validating imports for %d packages in the tree", len(packages))
		for _, pkg := range packages {
			if forbidden := restriction.ForbiddenImportsFor(pkg); len(forbidden) != 0 {
				logForbiddenPackages(relativePackage(pkg.ImportPath), forbidden)
				failedRestrictionCheck = true
			}
		}

		// make sure that all the allowed imports are used
		if unused := unusedPackageImports(restriction.AllowedImportPackages, packages); len(unused) > 0 {
			log.Printf("-- found unused package imports\n")
			for _, unusedPackage := range unused {
				log.Printf("\t%s\n", unusedPackage)
			}
			failedRestrictionCheck = true
		}
		if unused := unusedPackageImportRoots(restriction.AllowedImportPackageRoots, packages); len(unused) > 0 {
			log.Printf("-- found unused package import roots\n")
			for _, unusedPackage := range unused {
				log.Printf("\t%s\n", unusedPackage)
			}
			failedRestrictionCheck = true
		}

		log.Printf("\n")
	}

	if failedRestrictionCheck {
		os.Exit(1)
	}
}

func unusedPackageImports(allowedPackageImports []string, packages []Package) []string {
	ret := []string{}
	for _, allowedImport := range allowedPackageImports {
		if strings.HasPrefix(allowedImport, "vendor") {
			allowedImport = rootPackage + "/" + allowedImport
		}
		found := false
		for _, pkg := range packages {
			for _, packageToCheck := range append(pkg.Imports, append(pkg.TestImports, pkg.XTestImports...)...) {
				if packageToCheck == allowedImport {
					found = true
					break
				}
			}
		}
		if !found {
			ret = append(ret, relativePackage(allowedImport))
		}
	}

	return ret
}

func unusedPackageImportRoots(allowedPackageImportRoots []string, packages []Package) []string {
	ret := []string{}
	for _, allowedImportRoot := range allowedPackageImportRoots {
		if strings.HasPrefix(allowedImportRoot, "vendor") {
			allowedImportRoot = rootPackage + "/" + allowedImportRoot
		}
		found := false
		for _, pkg := range packages {
			for _, packageToCheck := range append(pkg.Imports, append(pkg.TestImports, pkg.XTestImports...)...) {
				if strings.HasPrefix(packageToCheck, allowedImportRoot) {
					found = true
					break
				}
			}
		}
		if !found {
			ret = append(ret, relativePackage(allowedImportRoot))
		}
	}

	return ret
}

func mergePackages(existingPackages, currPackages []Package) []Package {
	for _, currPackage := range currPackages {
		found := false
		for _, existingPackage := range existingPackages {
			if existingPackage.ImportPath == currPackage.ImportPath {
				log.Printf("-- Skipping: %v", currPackage.ImportPath)
				found = true
			}
		}
		if !found {
			// this was super noisy.
			//log.Printf("-- Adding: %v", currPackage.ImportPath)
			existingPackages = append(existingPackages, currPackage)
		}
	}

	return existingPackages
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

func resolvePackage(targetPackage string) ([]Package, error) {
	cmd := "go"
	args := []string{"list", "-json", targetPackage}
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
