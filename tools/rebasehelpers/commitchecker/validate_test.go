package main

import (
	"testing"

	"github.com/openshift/origin/tools/rebasehelpers/util"
)

func TestValidateUpstreamCommitsWithoutGodepsChanges(t *testing.T) {
	tests := []struct {
		name          string
		commits       []util.Commit
		errorExpected bool
	}{
		{
			name: "test 1",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "commit 2",
					Files:   []util.File{"Godeps/_workspace/src/file1", "pkg/file2"},
				},
			},
			errorExpected: true,
		},
		{
			name: "test 2",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: commit 2",
					Files:   []util.File{"Godeps/_workspace/src/file1", "pkg/file2"},
				},
			},
			errorExpected: false,
		},
	}
	for _, test := range tests {
		t.Logf("evaluating test %q", test.name)
		err := ValidateUpstreamCommitsWithoutGodepsChanges(test.commits)
		if err != nil {
			if test.errorExpected {
				t.Logf("got expected error:\n%s", err)
				continue
			} else {
				t.Fatalf("unexpected error:\n%s", err)
			}
		} else {
			if test.errorExpected {
				t.Fatalf("expected an error, got none")
			}
		}
	}
}

func TestValidateUpstreamCommitModifiesSingleGodepsRepo(t *testing.T) {
	tests := []struct {
		name          string
		commits       []util.Commit
		errorExpected bool
	}{
		{
			name: "test 1",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "commit 2",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/kubernetes/file1",
						"Godeps/_workspace/src/k8s.io/kubernetes/file2",
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "test 1 - vendor",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "commit 2",
					Files: []util.File{
						"vendor/k8s.io/kubernetes/file1",
						"vendor/k8s.io/kubernetes/file2",
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "test 2",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: commit 2",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/kubernetes/file1",
						"Godeps/_workspace/src/k8s.io/kubernetes/file2",
						"Godeps/_workspace/src/github.com/coreos/etcd/file",
					},
				},
				{
					Sha:     "aaa0002",
					Summary: "UPSTREAM: commit 3",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/heapster/file1",
						"Godeps/_workspace/src/github.com/coreos/etcd/file1",
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "test 2 - vendor",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: commit 2",
					Files: []util.File{
						"vendor/k8s.io/kubernetes/file1",
						"vendor/k8s.io/kubernetes/file2",
						"vendor/github.com/coreos/etcd/file",
					},
				},
				{
					Sha:     "aaa0002",
					Summary: "UPSTREAM: commit 3",
					Files: []util.File{
						"vendor/k8s.io/heapster/file1",
						"vendor/github.com/coreos/etcd/file1",
					},
				},
			},
			errorExpected: true,
		},
	}
	for _, test := range tests {
		t.Logf("evaluating test %q", test.name)
		err := ValidateUpstreamCommitModifiesSingleGodepsRepo(test.commits)
		if err != nil {
			if test.errorExpected {
				t.Logf("got expected error:\n%s", err)
				continue
			} else {
				t.Fatalf("unexpected error:\n%s", err)
			}
		} else {
			if test.errorExpected {
				t.Fatalf("expected an error, got none")
			}
		}
	}
}

func TestValidateUpstreamCommitModifiesOnlyGodeps(t *testing.T) {
	tests := []struct {
		name          string
		commits       []util.Commit
		errorExpected bool
	}{
		{
			name: "test 1",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: commit 2",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/kubernetes/file1",
						"Godeps/_workspace/src/k8s.io/kubernetes/file2",
						"pkg/some_file",
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "test 1 - vendor",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: commit 2",
					Files: []util.File{
						"vendor/k8s.io/kubernetes/file1",
						"vendor/k8s.io/kubernetes/file2",
						"pkg/some_file",
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "test 2",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: commit 2",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/kubernetes/file1",
						"Godeps/_workspace/src/k8s.io/kubernetes/file2",
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "test 2 - vendor",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: commit 2",
					Files: []util.File{
						"vendor/k8s.io/kubernetes/file1",
						"vendor/k8s.io/kubernetes/file2",
					},
				},
			},
			errorExpected: false,
		},
	}
	for _, test := range tests {
		t.Logf("evaluating test %q", test.name)
		err := ValidateUpstreamCommitModifiesOnlyGodeps(test.commits)
		if err != nil {
			if test.errorExpected {
				t.Logf("got expected error:\n%s", err)
				continue
			} else {
				t.Fatalf("unexpected error:\n%s", err)
			}
		} else {
			if test.errorExpected {
				t.Fatalf("expected an error, got none")
			}
		}
	}
}

func TestValidateUpstreamCommitSummaries(t *testing.T) {
	tests := []struct {
		summary string
		valid   bool
	}{
		{valid: true, summary: "UPSTREAM: 12345: a change"},
		{valid: true, summary: "UPSTREAM: k8s.io/heapster: 12345: a change"},
		{valid: true, summary: "UPSTREAM: <carry>: a change"},
		{valid: true, summary: "UPSTREAM: <drop>: a change"},
		{valid: true, summary: "UPSTREAM: coreos/etcd: <carry>: a change"},
		{valid: true, summary: "UPSTREAM: coreos/etcd: <drop>: a change"},
		{valid: true, summary: "UPSTREAM: revert: abcd123: 12345: a change"},
		{valid: true, summary: "UPSTREAM: revert: abcd123: k8s.io/heapster: 12345: a change"},
		{valid: true, summary: "UPSTREAM: revert: abcd123: <carry>: a change"},
		{valid: true, summary: "UPSTREAM: revert: abcd123: <drop>: a change"},
		{valid: true, summary: "UPSTREAM: revert: abcd123: coreos/etcd: <carry>: a change"},
		{valid: true, summary: "UPSTREAM: revert: abcd123: coreos/etcd: <drop>: a change"},
		{valid: false, summary: "UPSTREAM: whoopsie daisy"},
		{valid: true, summary: "UPSTREAM: gopkg.in/ldap.v2: 51: exposed better API for paged search"},
	}
	for _, test := range tests {
		commit := util.Commit{Summary: test.summary, Sha: "abcd000"}
		err := ValidateUpstreamCommitSummaries([]util.Commit{commit})
		if err != nil {
			if test.valid {
				t.Fatalf("unexpected error:\n%s", err)
			} else {
				t.Logf("got expected error:\n%s", err)
			}
		} else {
			if !test.valid {
				t.Fatalf("expected an error, got none; summary: %s", test.summary)
			}
		}
	}
}

func TestValidateUpstreamCommitModifiesOnlyDeclaredGodepRepo(t *testing.T) {
	tests := []struct {
		name          string
		commits       []util.Commit
		errorExpected bool
	}{
		{
			name: "test 1",
			commits: []util.Commit{
				{
					Sha:     "aaa0000",
					Summary: "commit 1",
					Files:   []util.File{"file1", "pkg/file2"},
				},
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: coreos/etcd: 12345: a change",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/kubernetes/file1",
						"Godeps/_workspace/src/k8s.io/kubernetes/file2",
						"Godeps/_workspace/src/github.com/coreos/etcd/file1",
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "test 2",
			commits: []util.Commit{
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: coreos/etcd: 12345: a change",
					Files: []util.File{
						"Godeps/_workspace/src/github.com/coreos/etcd/file1",
						"Godeps/_workspace/src/github.com/coreos/etcd/file2",
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "test three segments",
			commits: []util.Commit{
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: coreos/etcd: 12345: a change",
					Files: []util.File{
						"Godeps/_workspace/src/github.com/coreos/etcd/a/file1",
						"Godeps/_workspace/src/github.com/coreos/etcd/b/file2",
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "test 3",
			commits: []util.Commit{
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: 12345: a change",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/kubernetes/file1",
						"Godeps/_workspace/src/k8s.io/kubernetes/file2",
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "test 4",
			commits: []util.Commit{
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: 12345: a change",
					Files: []util.File{
						"Godeps/_workspace/src/github.com/coreos/etcd/file1",
						"Godeps/_workspace/src/github.com/coreos/etcd/file2",
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "test 5",
			commits: []util.Commit{
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: revert: abcd000: 12345: a change",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/kubernetes/file1",
						"Godeps/_workspace/src/k8s.io/kubernetes/file2",
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "test 6",
			commits: []util.Commit{
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: revert: abcd000: coreos/etcd: 12345: a change",
					Files: []util.File{
						"Godeps/_workspace/src/k8s.io/kubernetes/file1",
						"Godeps/_workspace/src/k8s.io/kubernetes/file2",
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "test 7",
			commits: []util.Commit{
				{
					Sha:     "aaa0001",
					Summary: "UPSTREAM: revert: abcd000: coreos/etcd: 12345: a change",
					Files: []util.File{
						"Godeps/_workspace/src/github.com/coreos/etcd/file1",
						"Godeps/_workspace/src/github.com/coreos/etcd/file2",
					},
				},
			},
			errorExpected: false,
		},
	}
	for _, test := range tests {
		t.Logf("evaluating test %q", test.name)
		err := ValidateUpstreamCommitModifiesOnlyDeclaredGodepRepo(test.commits)
		if err != nil {
			if test.errorExpected {
				t.Logf("got expected error:\n%s", err)
				continue
			} else {
				t.Fatalf("unexpected error:\n%s", err)
			}
		} else {
			if test.errorExpected {
				t.Fatalf("expected an error, got none")
			}
		}
	}
}
