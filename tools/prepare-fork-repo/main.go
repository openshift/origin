package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"gopkg.in/src-d/go-billy.v3/memfs"
	git "gopkg.in/src-d/go-git.v4"
	gitconfig "gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

var (
	errBranchNotFound error
	repoPrefix        = "git@github.com:openshift/"
)

// getRemoteBranch iterates over remote branches and return a reference to a provided named branch.
func getRemoteBranch(s storer.ReferenceStorer, branchName string) (*plumbing.Reference, error) {
	refs, err := s.IterReferences()
	if err != nil {
		return nil, err
	}
	var result *plumbing.Reference
	storer.NewReferenceFilteredIter(func(ref *plumbing.Reference) bool {
		return ref.Name().IsRemote()
	}, refs).ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().Short() == branchName {
			result = ref
		}
		return nil
	})
	if result == nil {
		return nil, errBranchNotFound
	}
	return result, nil
}

// This tool prepares the OpenShift fork repository for automated commit synchronization from
// openshift/origin repository done by publishing bot.
// It will create the new branch and push a starting commit with Origin-commit in commit message.
func main() {
	repoName := flag.String("repo", "", "origin fork repository name (eg. 'containers-image')")
	baseBranchName := flag.String("base-branch", "", "base branch name (eg. 'openshift-3.9')")
	branchName := flag.String("branch", "", "new branch name (eg. 'openshift-3.10')")
	commitMessage := flag.String("commit", "", "a starting point commit hash (eg. 'Origin-commit: FOO')")
	flag.Parse()
	if len(*repoName) == 0 {
		log.Fatalf("flag -repo must be set")
	}
	if len(*branchName) == 0 {
		log.Fatalf("flag -branch must be set")
	}
	if len(*commitMessage) == 0 {
		log.Fatalf("flag -commit must be set")
	}
	if len(*baseBranchName) == 0 {
		log.Fatalf("flag -base-branch must be set")
	}

	log.Printf("--> cloning openshift/%s ...", *repoName)
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL: repoPrefix + *repoName,
	})
	if err != nil {
		log.Fatalf("error cloning %s: %v", *repoName, err)
	}
	dstBranch, err := getRemoteBranch(r.Storer, "origin/"+*branchName)
	if err != nil && err != errBranchNotFound {
		log.Fatalf("unable to resolve branch %s: %v", *branchName, err)
	}
	baseBranch, err := getRemoteBranch(r.Storer, "origin/"+*baseBranchName)
	if err != nil {
		log.Fatalf("unable to resolve base branch %s: %v", *baseBranchName, err)
	}
	if dstBranch == nil {
		log.Printf("--> creating new branch %s", *branchName)
		dstBranch = plumbing.NewHashReference(plumbing.ReferenceName(fmt.Sprintf("refs/heads/origin/%s", *branchName)), baseBranch.Hash())
	}

	if err := r.Storer.SetReference(dstBranch); err != nil {
		log.Fatalf("unable to create %s branch: %v", *branchName, err)
	}

	w, _ := r.Worktree()
	commit, err := w.Commit(*commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "k8s-publishing-bot",
			Email: "bot@openshift.io",
			When:  time.Now(),
		},
		Committer: &object.Signature{
			Name:  "k8s-publishing-bot",
			Email: "bot@openshift.io",
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Fatalf("unable to make commit: %v", err)
	}
	obj, err := r.CommitObject(commit)
	if err != nil {
		log.Fatalf("unable to commit: %v", err)
	}
	log.Printf("--> adding commit %v (%s)", obj.Hash, *commitMessage)

	log.Printf("--> pushing branch %s ...", *branchName)
	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec("refs/heads/origin:refs/heads/origin")},
	})
	if err != nil {
		log.Fatalf("error pushing: %v", err)
	}
}
