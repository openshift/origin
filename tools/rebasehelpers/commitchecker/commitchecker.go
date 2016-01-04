package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openshift/origin/tools/rebasehelpers/util"
)

func main() {
	var start, end string
	flag.StringVar(&start, "start", "master", "The start of the revision range for analysis")
	flag.StringVar(&end, "end", "HEAD", "The end of the revision range for analysis")
	flag.Parse()

	commits, err := util.CommitsBetween(start, end)
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("ERROR: couldn't find commits from %s..%s: %v\n", start, end, err))
		os.Exit(1)
	}

	// TODO: Filter out bump commits for now until we decide how to deal with
	// them correctly.
	nonbumpCommits := []util.Commit{}
	for _, commit := range commits {
		if commit.DeclaresUpstreamChange() &&
			!strings.HasPrefix(commit.Summary, "bump(") {
			nonbumpCommits = append(nonbumpCommits, commit)
		}
	}

	errs := []string{}
	for _, validate := range AllValidators {
		err := validate(nonbumpCommits)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		os.Stderr.WriteString(strings.Join(errs, "\n\n"))
		os.Exit(2)
	}
}
