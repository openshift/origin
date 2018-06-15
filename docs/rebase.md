# Rebase on top of latest stable version of kubernetes

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Rebase process](#rebase-process)
  1. [Latest kubernetes](#latest-kubernetes)
  2. [Kubernetes fork](#kubernetes-fork)
  3. [Cherry picks](#cherry-picks)
  4. [Origin restore deps](#origin-restore-deps)
  5. [Checkout kubernetes](#checkout-kubernetes)
  6. [Kubernetes restore deps](#kubernetes-restore-deps)
  7. [Update deps](#update-deps)
  8. [Review changes](#review-changes)
  9. [Compilation and tests](#compilation-and-tests)



| NOTE |
| ---- |
| Even though all the steps provided in this document apply mostly to kubernetes rebase. Yet still, some parts are easily applicable to other dependencies. |


## Overview

In short the rebase process can be described with the following steps:

1. Pick latest stable kubernetes version you want to base your work on.
2. Create a `release-X.X.X` branch in [our kubernetes fork](https://github.com/openshift/kubernetes/),
   where `X.X.X` reflects the kubernetes version from step 1 and push its
   contents into it.
3. Review *ALL* cherry picks applied to a current version of kubernetes in our fork
   and apply them accordingly in a new PR against our fork targeting branch
   created in previous step.
4. Run `hack/godep-restore.sh` in origin repository to have appropriate state of
   dependencies.
5. Inside kubernetes repository checkout the desired kubernetes level with all
   the necessary cherry picks (iow. result of step 3).
6. Run `hack/godep-restore.sh` inside kubernetes repository to get the new level
   of required dependencies.
7. Run `hack/godep-save.sh` inside origin repository to get the required changes
   inside origin.
8. Review the changes remembering the following rules:
   - some dependencies we have are newer than k8s;
   - some dependencies we have contain our patches, these are usually reflected
   as forks under openshift organization on github;
9. Fix compilation errors and make sure *ALL* tests are running.

Once all of the above is done, you are ready to open a rebase PR, congratulations!


## Prerequisites

It is worth updating all our forks with the carry patches we hold in origin.
To do so run `hack/sync-forks.sh` and push the changes to appropriate repositories.


## Rebase process

Be brave, drink a lot of coffee or other fluid that keeps you energized. Ask for
help at any point in time when needed, and most importantly be patient, rebase
is very tedious and hard task that is very rewarding when it is done.

Good luck!


### 1. Latest kubernetes

As a rule of thumb we take the latest stable version of kubernetes that is released.
Check [their releases](https://github.com/kubernetes/kubernetes/releases) page
to get that information.

### 2. Kubernetes fork

Once you have picked the desired version of kubernetes you are planning to base
your work on to create an accompanying branch named `release-X.X.X` in
[our kubernetes fork](https://github.com/openshift/kubernetes/). Where `X.X.X`
is the exact version of kubernetes you have chosen.

If you do not have necessary access rights ask for that branch to be created for you.

### 3. Cherry-picks

For this step to be easily manageable it is required to run `hack/sync-forks.sh`
script mentioned in the prerequisites. This script will sync all `UPSTREAM` commits
to the current kubernetes version. This in turn helps to get the list of needed
upstream cherry picks you need to apply. There are 3 possible commit t ypes:

- `carry` - we need this, otherwise the entire world will collapse
- `12345` - specific PR number indicating that we picked something that was introduced
  in newer version. You need to double check if the current version of kubernetes
  you are working on has the patch. If not - make sure to carry it over.
- `drop` - these are the trickiest ones, because they require extra check, you
  need to ensure if the change introduced with such a commit is no longer needed.
  Personally, I always double check with commit author about those type of changes.

Once you are done with cherry picking you need to ensure at least that:

1. Kubernetes compiles, iow. `make all` works.
2. All unit tests are working - `make test`.
3. All integration are working - `make test-integration`.

Before moving on, the last commit should contain all the necessary generated
changes, run `make update` and commit the result as a `drop` commit.

Once all that is done, create a new pull request against [our kubernetes fork](https://github.com/openshift/kubernetes/),
targeting the branch created in the previous step.

### 4. Origin restore deps

At this point you are ready to start the actual rebase process. Inside origin
repository you need to run `hack/godep-restore.sh` to get the current level of
all dependencies origin requires. It is crucial to use this script instead of
manually invoking `godep save`, because this script add necessary forks information
for the deps where we carry our own specific patches.

In case of any problems during this step make sure to carefully examine the error
and if needed update the script or fix the contents of `Godeps/Godeps.json` so
that this step finishes cleanly.

### 5. Checkout kubernetes

After previous step go into kubernetes repository and checkout the desired version,
you prepared in step 3 and merged into [our kubernetes fork](https://github.com/openshift/kubernetes/).

### 6. Kubernetes restore deps

While inside kubernetes repository run `hack/godep-restore.sh` to get the new level
of required dependencies coming with newer version.

### 7. Update deps

Now that you have successfully restored origin dependencies and checked out the
new level of kubernetes and updated its new dependencies you are now ready to
update origin deps. To do so, go back to origin repository and run `hack/godep-save.sh`.
This will remove entire contents of `vendor/` and `Godeps` directories and create
new from the dependencies tree you have created in previous step.

### 8. Review changes

This and the next step will take the majority of your time. After updating all of
origin dependencies with changes coming from newer kubernetes it is time to review
the changes and make sure you don't break anything. The following rules apply during
this process

- Some dependencies we have are newer than k8s, keep origin's version.
- Some dependencies we have contain our patches, these are usually reflected
  as forks under openshift organization on github. Double check, if newer version
  of dependency is needed update our dep fork and re-apply patches and verify the
  patches we have, if they are still needed.
- Some dependencies might have been removed, double check if we need them.
- Some dependencies of ours have internal `vendor/` dirs, be extra careful not to
  touch those, since you might break something badly.
- Each transitive dependency is part of k8s bump commit.
- Each direct dependency should be included as a separate commit.

Generally, be extra cautious and careful with every dependency change. Make sure
to understand if the change is actually needed, you will be asked about it during
the review time.

### 9. Compilation and tests

When you finally reached this step all that is needed is to make sure that origin
compiles and all the tests are green. There are no clear guidelines here, do whatever
is needed to update the necessary bits of code and try to group the changes into
reasonable commits. Usually, interesting changes (iow. hard, not obvious, etc)
should be a separate commit. Whereas boring changes (renames, moves) can be easily
squashed into a single commit.


When you complete all of the above steps with success you are now ready to open
a PR with the rebase and answer all the review questions. Good luck!
