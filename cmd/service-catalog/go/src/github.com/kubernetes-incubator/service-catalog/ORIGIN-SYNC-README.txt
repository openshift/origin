This is a how-to for syncing the latest code from service-catalog and merging
it into the openshift/origin repository.

Prerequisite setup:
- git clone of service-catalog repo from https://github.com/kubernetes-incubator/service-catalog.git
- $ git remote add openshift git@github.com:openshift/service-catalog.git
- ensure there aren't any patches in the openshift/origin repo that need to be
  put in the openshift/service-catalog:origin-patches branch (git log
  cmd/service-catalog)

If patches need bringing over from openshift/origin, put them in the
service-catalog:origin-patches branch. Then squash all the changes into the
service-catalog:origin-patches-squashed branch. The reason this is important
to do is because once the subtree merge is performed, anything under
cmd/service-catalog/... will be overwritten. Also, make sure to rebase the
origin-patches branch as needed.

# syncs the openshift/service-catalog repo with the upstream tag
# (in service-catalog repo)
$ TAG=v0.0.10
$ git pull origin
$ git push openshift $TAG

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
