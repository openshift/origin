This is a how-to for syncing the latest code from service-catalog and merging
it into the openshift/origin repository.

Prerequisite setup:
- git clone of service-catalog repo from https://github.com/kubernetes-incubator/service-catalog.git
- $ git remote add openshift git@github.com:openshift/service-catalog.git
- ensure there aren't any patches in the openshift/origin repo that need to be
  put in the openshift/service-catalog:origin-patches branch (git log
  cmd/service-catalog)

# syncs the openshift/service-catalog repo with the upstream tag
# (in service-catalog repo)
$ TAG=v0.0.10
$ git pull origin (remote that points to service-catalog upstream)
$ git push openshift $TAG

(in service-catalog repo)
# update master (not used, but looks weird if not updated)
$ git checkout master
$ git merge --ff-only $TAG
$ git push

(let's not worry about the -squashed branch here and remove this)
If patches need bringing over from openshift/origin, put them in the
service-catalog:origin-patches branch. Then squash all the changes into the
service-catalog:origin-patches-squashed branch. The reason this is important
to do is because once the subtree merge is performed, anything under
cmd/service-catalog/... will be overwritten. Also, make sure to rebase the
origin-patches branch as needed.

# Update 9/1 - a better way for handling patches
(catalog repo)
$ git fetch openshift
(origin repo)
$ git pull
$ cd cmd/service-catalog
$ git log .
if patches, go ahead and descend to the path that will match the non-vendored repo
$ cd go/src/github.com/kubernetes-incubator/service-catalog
$ find SHAs needing bringing over (this could be done in one command, but for now do for each one):
$ git format-patch -1 ce7709e81b90e24aebfb5366001645a7e7d78fd8 --relative
$ mv *.patch to catalog repo
$ git am <patch file> (in origin-patches)
#unchecked
$ git rebase $TAG
$ git squash all origin patches into one commit... (technically optional, but looks nicer)
$ git push openshift

(NO LONGER NEEDED, not going to use squashed branch)
# updates code to latest tag and adds origin patches on top
# (in service-catalog repo)
$ git branch $TAG $TAG+origin
$ git checkout $TAG+origin
$ git cherry-pick <sha of origin-patches-squashed>
$ git push openshift

# pulls in code from openshift/service-catalog repo into OpenShift
# (in origin repo)
$ git pull
$ git subtree pull --prefix cmd/service-catalog/go/src/github.com/kubernetes-incubator/service-catalog https://github.com/openshift/service-catalog $TAG+origin --squash -m "Merge version $TAG of Service Catalog from https://github.com/openshift/service-catalog:$TAG+origin"
