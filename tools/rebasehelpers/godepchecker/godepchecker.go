package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/tools/rebasehelpers/util"
)

var gopath = os.Getenv("GOPATH")

func main() {

	fmt.Println(`
  Assumes the following:
  - $GOPATH is set to a single directory (not the godepsified path)
  - "godeps save ./..." has not yet been run on origin
  - The desired level of kubernetes is checked out
`)
	var self, other string
	var checkoutNewer, examineForks bool
	flag.StringVar(&self, "self", filepath.Join(gopath, "src/github.com/openshift/origin/Godeps/Godeps.json"), "The first file to compare")
	flag.StringVar(&other, "other", filepath.Join(gopath, "src/k8s.io/kubernetes/Godeps/Godeps.json"), "The other file to compare")
	flag.BoolVar(&checkoutNewer, "checkout", checkoutNewer, "Check out the newer commit when there is a mismatch between the Godeps")
	flag.BoolVar(&examineForks, "examine-forks", examineForks, "Print out git logs from OpenShift forks or upstream dependencies when there is a mismatch in revisions between Kubernetes and Origin")
	flag.Parse()

	// List packages imported by origin Godeps
	originGodeps, err := loadGodeps(self)
	if err != nil {
		exit(fmt.Sprintf("Error loading %s:", self), err)
	}

	// List packages imported by kubernetes Godeps
	k8sGodeps, err := loadGodeps(other)
	if err != nil {
		exit(fmt.Sprintf("Error loading %s:", other), err)
	}

	// List packages imported by origin
	_, errs := loadImports(".")
	if len(errs) > 0 {
		exit("Error loading imports:", errs...)
	}

	mine := []string{}
	yours := []string{}
	ours := []string{}
	for k := range originGodeps {
		if _, exists := k8sGodeps[k]; exists {
			ours = append(ours, k)
		} else {
			mine = append(mine, k)
		}
	}
	for k := range k8sGodeps {
		if _, exists := originGodeps[k]; !exists {
			yours = append(yours, k)
		}
	}

	sort.Strings(mine)
	sort.Strings(yours)
	sort.Strings(ours)

	// Check for missing k8s deps
	if len(yours) > 0 {
		fmt.Println("k8s-only godep imports (may need adding to origin):")
		for _, k := range yours {
			fmt.Println(k)
		}
		fmt.Printf("\n\n\n")
	}

	// Check `mine` for unused local deps (might be used transitively by other Godeps)

	// Check `ours` for different levels
	openshiftForks := sets.NewString(
		"github.com/docker/distribution",
		"github.com/skynetservices/skydns",
		"github.com/coreos/etcd",
		"github.com/emicklei/go-restful",
		"github.com/golang/glog",
		"github.com/cloudflare/cfssl",
		"github.com/google/certificate-transparency",
		"github.com/RangelReale/osin",
		"github.com/google/cadvisor",
	)

	lastMismatch := ""
	for _, k := range ours {
		if oRev, kRev := originGodeps[k].Rev, k8sGodeps[k].Rev; oRev != kRev {
			if lastMismatch == oRev {
				// don't show consecutive mismatches if oRev is the same
				continue
			}
			lastMismatch = oRev

			fmt.Printf("Mismatch on %s:\n", k)
			newerCommit := ""
			repoPath := filepath.Join(gopath, "src", k)

			oDecorator := ""
			kDecorator := ""
			currentRev, err := util.CurrentRev(repoPath)
			if err == nil {
				if currentRev == oRev {
					kDecorator = " "
					oDecorator = "*"
				}
				if currentRev == kRev {
					kDecorator = "*"
					oDecorator = " "
				}
			}

			oDate, oDateErr := util.CommitDate(oRev, repoPath)
			if oDateErr != nil {
				oDate = "unknown"
			}
			kDate, kDateErr := util.CommitDate(kRev, repoPath)
			if kDateErr != nil {
				kDate = "unknown"
			}

			if err := util.FetchRepo(repoPath); err != nil {
				fmt.Printf("    Error fetching %q: %v\n", repoPath, err)
			}
			openShiftNewer := false
			if older, err := util.IsAncestor(oRev, kRev, repoPath); older && err == nil {
				fmt.Printf("    Origin: %s%s (%s)\n", oDecorator, oRev, oDate)
				fmt.Printf("    K8s:    %s%s (%s, fast-forward)\n", kDecorator, kRev, kDate)
				newerCommit = kRev
			} else if newer, err := util.IsAncestor(kRev, oRev, repoPath); newer && err == nil {
				fmt.Printf("    Origin: %s%s (%s, fast-forward)\n", oDecorator, oRev, oDate)
				fmt.Printf("    K8s:    %s%s (%s)\n", kDecorator, kRev, kDate)
				newerCommit = oRev
				openShiftNewer = true
			} else if oDateErr == nil && kDateErr == nil {
				fmt.Printf("    Origin: %s%s (%s, discontinuous)\n", oDecorator, oRev, oDate)
				fmt.Printf("    K8s:    %s%s (%s, discontinuous)\n", kDecorator, kRev, kDate)
				if oDate > kDate {
					newerCommit = oRev
				} else {
					newerCommit = kRev
				}
			} else {
				fmt.Printf("    Origin: %s%s (%s)\n", oDecorator, oRev, oDate)
				fmt.Printf("    K8s:    %s%s (%s)\n", kDecorator, kRev, kDate)
				if oDateErr != nil {
					fmt.Printf("    %s\n", oDateErr)
				}
				if kDateErr != nil {
					fmt.Printf("    %s\n", kDateErr)
				}
			}

			if len(newerCommit) > 0 && newerCommit != currentRev {
				if checkoutNewer {
					fmt.Printf("    Checking out:\n")
					fmt.Printf("    cd %s && git checkout %s\n", repoPath, newerCommit)
					if err := util.Checkout(newerCommit, repoPath); err != nil {
						fmt.Printf("    %s\n", err)
					}
				} else {
					fmt.Printf("    To check out newest:\n")
					fmt.Printf("    cd %s && git checkout %s\n", repoPath, newerCommit)
				}
			}

			if !openShiftNewer {
				// only proceed to examine forks if OpenShift's commit is newer than
				// Kube's
				continue
			}
			if !examineForks {
				continue
			}
			if currentRev == oRev {
				// we're at OpenShift's commit, so there's not really any need to show
				// the commits
				continue
			}
			if !strings.HasPrefix(k, "github.com/") {
				continue
			}

			parts := strings.SplitN(k, "/", 4)
			repo := fmt.Sprintf("github.com/%s/%s", parts[1], parts[2])
			if !openshiftForks.Has(repo) {
				continue
			}

			fmt.Printf("\n    Fork info:\n")

			if err := func() error {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				defer os.Chdir(cwd)

				if err := os.Chdir(repoPath); err != nil {
					return err
				}

				commits, err := util.CommitsBetween(kRev+"^", oRev)
				if err != nil {
					return err
				}
				for _, commit := range commits {
					fmt.Printf("      %s %s\n", commit.Sha, commit.Summary)
				}
				return nil
			}(); err != nil {
				fmt.Printf("    Error examining fork: %v\n", err)
			}

		}
	}
}

func exit(reason string, errors ...error) {
	fmt.Fprintf(os.Stderr, "%s\n", reason)
	for _, err := range errors {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	os.Exit(2)
}

func loadImports(root string) (map[string]bool, []error) {
	imports := map[string]bool{}
	errs := []error{}
	fset := &token.FileSet{}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// Don't walk godeps
		if info.Name() == "Godeps" && info.IsDir() {
			return filepath.SkipDir
		}

		if strings.HasSuffix(info.Name(), ".go") && info.Mode().IsRegular() {
			if fileAST, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly); err != nil {
				errs = append(errs, err)
			} else {
				for i := range fileAST.Imports {
					pkg := fileAST.Imports[i].Path.Value
					imports[pkg[1:len(pkg)-2]] = true
				}
			}
		}
		return nil
	})
	return imports, errs
}

type Godep struct {
	Deps []Dep
}
type Dep struct {
	ImportPath string
	Comment    string
	Rev        string
}

func loadGodeps(file string) (map[string]Dep, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	godeps := &Godep{}
	if err := json.Unmarshal(data, godeps); err != nil {
		return nil, err
	}

	depmap := map[string]Dep{}
	for i := range godeps.Deps {
		dep := godeps.Deps[i]
		if _, exists := depmap[dep.ImportPath]; exists {
			return nil, fmt.Errorf("imports %q multiple times", dep.ImportPath)
		}
		depmap[dep.ImportPath] = dep
	}
	return depmap, nil
}
