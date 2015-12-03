#!/bin/bash
#
# This is a script for making images rebuilds semi-automated.
# It can basically do two things:
#  * create BZs (tracker bug and clones for other components)
#  * rebase itself, that consists of:
#    - bump release and update FROM clause in Dockerfile
#    - commit, push in dist-git
#    - run build in OSBS
#    (BZs need to be created before running rebuild)
#
# Before working , you need your kerberos log in:
#   kinit
#
# Required packages:
#   rhpkg
#   krb5-workstation
#   git


BRANCH_RELEASE=2.0

# format:
# dist-git_name	image_dependency dist-git_branch git_repo git_path
base_images_list="
openshift-enterprise-base-docker None rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/base
openshift-enterprise-pod-docker None rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/pod
openshift-enterprise-keepalived-ipfailover-docker openshift-enterprise-base-docker rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/ipfailover/keepalived
openshift-enterprise-dockerregistry-docker None openshift-enterprise-base-docker-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/dockerregistry
openshift-enterprise-docker openshift-enterprise-base-docker rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/ose
openshift-enterprise-haproxy-router-base-docker openshift-enterprise-base-docker rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/router/haproxy-base
openshift-enterprise-deployer-docker openshift-enterprise-docker rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/deployer
openshift-enterprise-sti-builder-docker openshift-enterprise-docker rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/builder/docker/sti-builder
openshift-enterprise-docker-builder-docker openshift-enterprise-docker rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/builder/docker/docker-builder
openshift-enterprise-haproxy-router-docker openshift-enterprise-haproxy-router-base-docker rhaos-3.1-rhel-7 git@github.com:openshift/ose.git ose/images/router/haproxy
"

# format:
# dist-git_name	image_dependency dist-git_branch git_repo git_path
s2i_images_list="
openshift-sti-base-docker None rhaos-3.1-rhel-7 https://github.com/openshift/sti-base sti-base
openshift-mongodb-docker None rhaos-3.1-rhel-7 https://github.com/openshift/mongodb mongodb/2.4
openshift-mysql-docker None rhaos-3.1-rhel-7 https://github.com/openshift/mysql mysql/5.5
openshift-postgresql-docker None rhaos-3.1-rhel-7 https://github.com/openshift/postgresql postgresql/9.2
openshift-sti-nodejs-docker openshift-sti-base-docker rhaos-3.1-rhel-7 https://github.com/openshift/sti-nodejs sti-nodejs/0.10
openshift-sti-perl-docker openshift-sti-base-docker rhaos-3.1-rhel-7 https://github.com/openshift/sti-perl sti-perl/5.16
openshift-sti-php-docker openshift-sti-base-docker rhaos-3.1-rhel-7 https://github.com/openshift/sti-php sti-php/5.5
openshift-sti-python-docker openshift-sti-base-docker rhaos-3.1-rhel-7 https://github.com/openshift/sti-python sti-python/3.3
openshift-sti-ruby-docker openshift-sti-base-docker rhaos-3.1-rhel-7 https://github.com/openshift/sti-ruby sti-ruby/2.0
"

usage() {
  echo "Usage `basename $0` <action> <version>" >&2
  echo >&2
  echo "Actions:" >&2
  echo "  everything_base  - git_update,  build_container (non s2i images)" >&2
  echo "  everything_s2i   - git_update,  build_container (s2i images)" >&2
  echo "  git_update_base  - Clone git and dist-git, bump release, compare (non-s2i images)" >&2
  echo "  git_update_s2i   - Clone git and dist-git, bump release, compare (s2i images)" >&2
  echo "  build_container_base - Clone dist-git, build containers (non-s2i images)" >&2
  echo "  build_container_s2i  - Clone dist-git, build containers (s2i images)" >&2
  echo >&2
  echo "Version:" >&2
  echo "  specific image version, e.g. 3.1.1.2 or 1.1 (What should be in LABEL Version)" >&2
  popd &>/dev/null
  exit 1
}

bump_release() {
  sed -i -e "s/FROM rhel7.*/FROM $base_image/" Dockerfile
  cur=$(grep -e 'Release="\([0-9]*\)"' Dockerfile | head -n 1 | sed -e 's/^.*Release="\([0-9]*\)".*$/\1/')
  (( cur++ ))
  sed -i -e "s/Release=\"[0-9]*\"/Release=\"$cur\"/" Dockerfile
}

setup_dist_git() {
  if ! klist &>/dev/null ; then
    echo "Error: Kerberos token not found." ; popd &>/dev/null ; exit 1
  fi
  echo "=== ${container} ==="
  rhpkg clone "${container}" &>/dev/null
  pushd ${container} >/dev/null
  rhpkg switch-branch "$branch" &>/dev/null
  popd >/dev/null
}

setup_git_repo() {
  pushd "${workingdir}" >/dev/null
  git clone -q ${repo} 2>/dev/null
  popd >/dev/null

}

check_builds() {
  pushd "${workingdir}/logs" >/dev/null
  ls -1 *buildlog | while read line
  do
    if grep -q -e "buildContainer (noarch) failed" -e "server startup error" ${line} ; then
      package=`echo ${line} | cut -d'.' -f1`
      echo "=== ${package} IMAGE BUILD FAILED ==="
      echo " exiting"
      mv ${line} ${package}.watchlog done/
      exit 23
    fi
    if grep -q -e "buildContainer (noarch) completed successfully" ${line} ; then
      package=`echo ${line} | cut -d'.' -f1`
      echo "=== ${package} IMAGE COMPLETED ==="
      if grep "No package" ${package}.watchlog ; then
        echo "  ERROR IN IMAGE for ${package}"
        # echo "  exiting"
        # exit 25
      fi
      echo "::${package}::" >> ${workingdir}/logs/finished
      mv ${line} ${package}.watchlog done/
    fi
  done
  popd >/dev/null
}

wait_for_all_builds() {
  buildcheck=`ls -1 ${workingdir}/logs/*buildlog 2>/dev/null`
  while ! [ "${buildcheck}" == "" ]
  do
    echo "=== waiting for these builds ==="
    date
    echo "${buildcheck}"
    sleep 120
    check_builds
    buildcheck=`ls -1 ${workingdir}/logs/*buildlog 2>/dev/null`
  done
}

check_build_dependencies() {
  depcheck=`grep ${dependency} ${workingdir}/logs/finished`
  while [ "${depcheck}" == "" ]
  do
    now=`date`
    echo "Waiting for ${dependency} to be built - ${now}"
    sleep 120
    check_builds
    depcheck=`grep ${dependency} ${workingdir}/logs/finished`
  done
}

build_image() {
  check_build_dependencies
  rhpkg container-build --scratch --repo http://file.rdu.redhat.com/sdodson/aos-unsigned.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
  echo -n "  Waiting for createContainer taskid ."
  taskid=`grep createContainer ${workingdir}/logs/${container}.buildlog | awk '{print $1}' | sort -u`
  while [ "${taskid}" == "" ]
  do
    echo -n "."
    sleep 5
    if grep -q -e "buildContainer (noarch) failed" -e "server startup error" ${workingdir}/logs/${container}.buildlog ; then
      echo " error"
      echo "=== ${container} IMAGE BUILD FAILED ==="
      echo "  exiting"
      exit 23
    fi
    taskid=`grep createContainer ${workingdir}/logs/${container}.buildlog | awk '{print $1}' | sort -u`
  done
  echo " "
  brew watch-logs ${taskid} >> ${workingdir}/logs/${container}.watchlog 2>&1 &

}

show_git_diffs() {
  pushd "${workingdir}/${container}" >/dev/null
  find . -name ".git*" -prune -o -name ".osbs*" -prune -o -type f -print | while read line
  do
    updatetime=`git log --date=iso -n 1 --pretty="%ad" ${line}`
    pushd "${workingdir}/${path}" >/dev/null
    newupdates=`git log --oneline --after="${updatetime}" ${line} | wc -l`
    if [ ${newupdates} -gt 0 ] ; then
      echo "=== Change in ${container}/${line} ==="
      lastgit=`git log -1 --pretty="%H" --before="${updatetime}" ${line}`
      git diff ${lastgit} ${line}
    fi
    popd >/dev/null
  done
  popd >/dev/null

}

build_container() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  build_image
  popd >/dev/null
}

git_update() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  setup_git_repo
  show_git_diffs
  popd >/dev/null
}


workingdir=$(mktemp -d /var/tmp/rebuild-images-XXXXXX)
pushd "${workingdir}" &>/dev/null
mkdir -p logs/done
echo "::None::" >> logs/finished
echo "Using working directory: ${workingdir}"

if [ "$#" -ne 2 ] ; then
  usage
fi

export action="$1"
export base_image="$2"

case "$action" in
  everything_base|git_update_base|build_container_base) list="${base_images_list}" ;;
  everything_s2i|git_update_s2i|build_container_s2i) list="${s2i_images_list}" ;;
  everything_all|git_update_all|build_container_all) create_tracker ; list="${base_images_list}
${s2i_images_list}" ;;
  *) usage ;;
esac

echo "$list" | while read spec ; do
  [ -z "$spec" ] && continue
  export container=$(echo "$spec" | awk '{print $1}')
  export dependency=$(echo "$spec" | awk '{print $2}')
  export branch=$(echo "$spec" | awk '{print $3}')
  export repo=$(echo "$spec" | awk '{print $4}')
  export path=$(echo "$spec" | awk '{print $5}')
  case "$action" in
    build_container_base | build_container_s2i )
      build_container
      ;;
    git_update_base | git_update_s2i )
       git_update
       ;;
    everything_base | everything_s2i )
      git_update
      build_container
      ;;
    * ) usage ;;
  esac
done

wait_for_all_builds
