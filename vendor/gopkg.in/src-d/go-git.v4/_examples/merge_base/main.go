package main

import (
	"os"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type exitCode int

const (
	exitCodeSuccess exitCode = iota
	exitCodeNotFound
	exitCodeWrongSyntax
	exitCodeCouldNotOpenRepository
	exitCodeCouldNotParseRevision
	exitCodeUnexpected

	cmdDesc = "Returns the merge-base between two commits:"

	helpShortMsg = `
  usage: %_COMMAND_NAME_% <path> <commitRev> <commitRev>
     or: %_COMMAND_NAME_% <path> --independent <commitRev>...
     or: %_COMMAND_NAME_% <path> --is-ancestor <commitRev> <commitRev>
     or: %_COMMAND_NAME_% --help

 params:
    <path>          path to the git repository
    <commitRev>     git revision as supported by go-git

options:
    (no options)    lists the best common ancestors of the two passed commits
    --independent   list commits not reachable from the others
    --is-ancestor   is the first one ancestor of the other?
    --help          show the full help message of %_COMMAND_NAME_%
`
)

// Command that mimics `git merge-base --all <baseRev> <headRev>`
// Command that mimics `git merge-base --is-ancestor <baseRev> <headRev>`
// Command that mimics `git merge-base --independent <commitRev>...`
func main() {
	if len(os.Args) == 1 {
		helpAndExit("Returns the merge-base between two commits:", helpShortMsg, exitCodeSuccess)
	}

	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		helpAndExit("Returns the merge-base between two commits:", helpLongMsg, exitCodeSuccess)
	}

	if len(os.Args) < 4 {
		helpAndExit("Wrong syntax", helpShortMsg, exitCodeWrongSyntax)
	}

	path := os.Args[1]

	var modeIndependent, modeAncestor bool
	var commitRevs []string
	var res []*object.Commit

	switch os.Args[2] {
	case "--independent":
		modeIndependent = true
		commitRevs = os.Args[3:]
	case "--is-ancestor":
		modeAncestor = true
		commitRevs = os.Args[3:]
		if len(commitRevs) != 2 {
			helpAndExit("Wrong syntax", helpShortMsg, exitCodeWrongSyntax)
		}
	default:
		commitRevs = os.Args[2:]
		if len(commitRevs) != 2 {
			helpAndExit("Wrong syntax", helpShortMsg, exitCodeWrongSyntax)
		}
	}

	// Open a git repository from current directory
	repo, err := git.PlainOpen(path)
	checkIfError(err, exitCodeCouldNotOpenRepository, "not in a git repository")

	// Get the hashes of the passed revisions
	var hashes []*plumbing.Hash
	for _, rev := range commitRevs {
		hash, err := repo.ResolveRevision(plumbing.Revision(rev))
		checkIfError(err, exitCodeCouldNotParseRevision, "could not parse revision '%s'", rev)
		hashes = append(hashes, hash)
	}

	// Get the commits identified by the passed hashes
	var commits []*object.Commit
	for _, hash := range hashes {
		commit, err := repo.CommitObject(*hash)
		checkIfError(err, exitCodeUnexpected, "could not find commit '%s'", hash.String())
		commits = append(commits, commit)
	}

	if modeAncestor {
		isAncestor, err := commits[0].IsAncestor(commits[1])
		checkIfError(err, exitCodeUnexpected, "could not traverse the repository history")

		if !isAncestor {
			os.Exit(int(exitCodeNotFound))
		}

		os.Exit(int(exitCodeSuccess))
	}

	if modeIndependent {
		res, err = object.Independents(commits)
		checkIfError(err, exitCodeUnexpected, "could not traverse the repository history")
	} else {
		res, err = commits[0].MergeBase(commits[1])
		checkIfError(err, exitCodeUnexpected, "could not traverse the repository history")

		if len(res) == 0 {
			os.Exit(int(exitCodeNotFound))
		}
	}

	printCommits(res)
}
