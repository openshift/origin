package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/openshift/origin/tools/rebasehelpers/util"
)

var CommitSummaryErrorTemplate = `
The following UPSTREAM commits have invalid summaries:

{{ range .Commits }}  [{{ .Sha }}] {{ .Summary }}
{{ end }}
UPSTREAM commit summaries should look like:

  UPSTREAM: [non-kube-repo/name: ]<PR number|carry|drop>: description

UPSTREAM commits which revert previous UPSTREAM commits should look like:

  UPSTREAM: revert: <sha>: <normal upstream format>

UPSTREAM commits are validated against the following regular expression:

  {{ .Pattern }}

Examples of valid summaries:

  UPSTREAM: 12345: A kube fix
  UPSTREAM: coreos/etcd: 12345: An etcd fix
  UPSTREAM: <carry>: A carried kube change
  UPSTREAM: <drop>: A dropped kube change
  UPSTREAM: revert: abcd123: coreos/etcd: 12345: An etcd fix
  UPSTREAM: k8s.io/heapster: 12345: A heapster fix

`

var AllValidators = []func([]util.Commit) error{
	ValidateUpstreamCommitSummaries,
	ValidateUpstreamCommitsWithoutGodepsChanges,
	ValidateUpstreamCommitModifiesSingleGodepsRepo,
	ValidateUpstreamCommitModifiesOnlyGodeps,
	ValidateUpstreamCommitModifiesOnlyDeclaredGodepRepo,
}

// ValidateUpstreamCommitsWithoutGodepsChanges returns an error if any
// upstream commits have no Godeps changes.
func ValidateUpstreamCommitsWithoutGodepsChanges(commits []util.Commit) error {
	problemCommits := []util.Commit{}
	for _, commit := range commits {
		if commit.HasGodepsChanges() && !commit.DeclaresUpstreamChange() {
			problemCommits = append(problemCommits, commit)
		}
	}
	if len(problemCommits) > 0 {
		label := "The following commits contain Godeps changes but aren't declared as UPSTREAM"
		msg := renderGodepFilesError(label, problemCommits, RenderOnlyGodepsFiles)
		return fmt.Errorf(msg)
	}
	return nil
}

// ValidateUpstreamCommitModifiesSingleGodepsRepo returns an error if any
// upstream commits have changes that span more than one Godeps repo.
func ValidateUpstreamCommitModifiesSingleGodepsRepo(commits []util.Commit) error {
	problemCommits := []util.Commit{}
	for _, commit := range commits {
		godepsChanges, err := commit.GodepsReposChanged()
		if err != nil {
			return err
		}
		if len(godepsChanges) > 1 {
			problemCommits = append(problemCommits, commit)
		}
	}
	if len(problemCommits) > 0 {
		label := "The following UPSTREAM commits modify more than one repo in their changelist"
		msg := renderGodepFilesError(label, problemCommits, RenderOnlyGodepsFiles)
		return fmt.Errorf(msg)
	}
	return nil
}

// ValidateUpstreamCommitSummaries ensures that any commits which declare to
// be upstream match the regular expressions for UPSTREAM summaries.
func ValidateUpstreamCommitSummaries(commits []util.Commit) error {
	problemCommits := []util.Commit{}
	for _, commit := range commits {
		if commit.DeclaresUpstreamChange() && !commit.MatchesUpstreamSummaryPattern() {
			problemCommits = append(problemCommits, commit)
		}
	}
	if len(problemCommits) > 0 {
		tmpl, _ := template.New("problems").Parse(CommitSummaryErrorTemplate)
		data := struct {
			Pattern *regexp.Regexp
			Commits []util.Commit
		}{
			Pattern: util.UpstreamSummaryPattern,
			Commits: problemCommits,
		}
		buffer := &bytes.Buffer{}
		tmpl.Execute(buffer, data)
		return fmt.Errorf(buffer.String())
	}
	return nil
}

// ValidateUpstreamCommitModifiesOnlyGodeps ensures that any Godeps commits
// modify ONLY Godeps files.
func ValidateUpstreamCommitModifiesOnlyGodeps(commits []util.Commit) error {
	problemCommits := []util.Commit{}
	for _, commit := range commits {
		if commit.HasGodepsChanges() && commit.HasNonGodepsChanges() {
			problemCommits = append(problemCommits, commit)
		}
	}
	if len(problemCommits) > 0 {
		label := "The following UPSTREAM commits modify files outside Godeps"
		msg := renderGodepFilesError(label, problemCommits, RenderAllFiles)
		return fmt.Errorf(msg)
	}
	return nil
}

// ValidateUpstreamCommitModifiesOnlyDeclaredGodepRepo ensures that an
// upstream commit only modifies the Godep repo the summary declares.
func ValidateUpstreamCommitModifiesOnlyDeclaredGodepRepo(commits []util.Commit) error {
	problemCommits := []util.Commit{}
	for _, commit := range commits {
		if commit.DeclaresUpstreamChange() {
			declaredRepo, err := commit.DeclaredUpstreamRepo()
			if err != nil {
				return err
			}
			reposChanged, err := commit.GodepsReposChanged()
			if err != nil {
				return err
			}
			for _, changedRepo := range reposChanged {
				if !strings.Contains(changedRepo, declaredRepo) {
					problemCommits = append(problemCommits, commit)
				}
			}
		}
	}
	if len(problemCommits) > 0 {
		label := "The following UPSTREAM commits modify Godeps repos other than the repo the commit declares"
		msg := renderGodepFilesError(label, problemCommits, RenderAllFiles)
		return fmt.Errorf(msg)
	}
	return nil
}

type CommitFilesRenderOption int

const (
	RenderNoFiles CommitFilesRenderOption = iota
	RenderOnlyGodepsFiles
	RenderOnlyNonGodepsFiles
	RenderAllFiles
)

// renderGodepFilesError formats commits and their file lists into readable
// output prefixed with label.
func renderGodepFilesError(label string, commits []util.Commit, opt CommitFilesRenderOption) string {
	msg := fmt.Sprintf("%s:\n\n", label)
	for _, commit := range commits {
		msg += fmt.Sprintf("[%s] %s\n", commit.Sha, commit.Summary)
		if opt == RenderNoFiles {
			continue
		}
		for _, file := range commit.Files {
			if opt == RenderAllFiles ||
				(opt == RenderOnlyGodepsFiles && file.HasGodepsChanges()) ||
				(opt == RenderOnlyNonGodepsFiles && !file.HasGodepsChanges()) {
				msg += fmt.Sprintf("  - %s\n", file)
			}
		}
	}
	return msg
}
