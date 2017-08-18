# Reviewing and Merging Pull Requests

This document is a guideline for how core contributors should review and merge
pull requests (PRs). It is intended to outline the lightweight process that
we’ll use for now. It’s assumed that we’ll operate on good faith for now in
situations for which process is not specified.

PRs may only be merged after the following criteria are met:

1. It has been 'LGTM'-ed by 2 different reviewers
1. It has all appropriate corresponding documentation and testcases

## LGTMs

When a reviewer deems a PR good enough to merge, they should add a comment to the PR
thread that simply reads 'LGTM'. If they do not deem it ready for merge,
they should add comments -- either inline or on the PR thread -- that indicate
changes they believe should be made before merge.

## Vetoing

If a reviewer decides that a PR should not be merged in its current state,
even if it has 2 'LGTM' approvals from others, they should mark that PR with
`do-not-merge` label.

This label should only be used by a reviewer when that person believes there
is a fundamental problem with the PR. The reviewer should summarize that problem
in the PR comments and a longer discussion may be required.

We expect this label to be used infrequently.
