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
# Before working with bugzilla, you need to log in:
#   bugzilla login
#
# Required packages:
#   python-bugzilla
#   rhpkg
#   krb5-workstation


BRANCH_RELEASE=2.0

#set -ex
# format:
# dist-git_name	image_dependency dist-git_branch
base_images_list="
openshift-enterprise-base-docker None rhaos-3.1-rhel-7
openshift-enterprise-pod-docker None rhaos-3.1-rhel-7
openshift-enterprise-keepalived-ipfailover-docker openshift-enterprise-base-docker rhaos-3.1-rhel-7
openshift-enterprise-dockerregistry-docker None openshift-enterprise-base-docker-3.1-rhel-7
openshift-enterprise-docker openshift-enterprise-base-docker rhaos-3.1-rhel-7
openshift-enterprise-haproxy-router-base-docker openshift-enterprise-base-docker rhaos-3.1-rhel-7
openshift-enterprise-deployer-docker openshift-enterprise-docker rhaos-3.1-rhel-7
openshift-enterprise-sti-builder-docker openshift-enterprise-docker rhaos-3.1-rhel-7
openshift-enterprise-docker-builder-docker openshift-enterprise-docker rhaos-3.1-rhel-7
openshift-enterprise-haproxy-router-docker openshift-enterprise-haproxy-router-base-docker rhaos-3.1-rhel-7
"

s2i_images_list="
rh-php56	php-56-rhel7		rh-php56-docker		rhscl-${BRANCH_RELEASE}-rh-php56-rhel-7
rh-python34	python-34-rhel7		rh-python34-docker	rhscl-${BRANCH_RELEASE}-rh-python34-rhel-7
python27	python-27-rhel7		python27-docker		rhscl-${BRANCH_RELEASE}-python27-rhel-7
rh-perl520	perl-520-rhel7		rh-perl520-docker	rhscl-${BRANCH_RELEASE}-rh-perl520-rhel-7
rh-ruby22	ruby-22-rhel7		rh-ruby22-docker	rhscl-${BRANCH_RELEASE}-rh-ruby22-rhel-7
rh-passenger40	passenger-40-rhel7	rh-passenger40-docker	rhscl-${BRANCH_RELEASE}-rh-passenger40-rhel-7
"

usage() {
  echo "Usage `basename $0` <action> <base>" >&2
  echo >&2
  echo "Actions:" >&2
  echo "  rebase_base     - Clone dist-git, bump release, commit, push, build in brew (non s2i images)" >&2
  echo "  rebase_s2i      - Clone dist-git, bump release, commit, push, build in brew (s2i images)" >&2
  echo "  testrebase_base - Clone dist-git, bump release, commit (non-s2i images)" >&2
  echo "  testrebase_s2i  - Clone dist-git, bump release, commit (s2i images)" >&2
  echo "  createbz        - Creates tracker BZ and cloned BZs for every component" >&2
  echo "  testcreatebz    - Reports what BZs would be created" >&2
  echo >&2
  echo "Base:" >&2
  echo "  specific rhel-base release, e.g. rhel7:7.2-30 (won't be used if FROM does not include rhel)" >&2
  popd &>/dev/null
  exit 1
}

bump_release() {
  sed -i -e "s/FROM rhel7.*/FROM $base_image/" Dockerfile
  cur=$(grep -e 'Release="\([0-9]*\)"' Dockerfile | head -n 1 | sed -e 's/^.*Release="\([0-9]*\)".*$/\1/')
  (( cur++ ))
  sed -i -e "s/Release=\"[0-9]*\"/Release=\"$cur\"/" Dockerfile
}

testrebase() {
  if ! klist &>/dev/null ; then
    echo "Error: Kerberos token not found." ; popd &>/dev/null ; exit 1
  fi
  echo "=== $bz_component ==="
  rhpkg clone "$bz_component" &>/dev/null
  cd $bz_component
  rhpkg switch-branch "$branch" &>/dev/null
  bump_release
  tracker_bz=$(bugzilla query -i -c distribution -p 'Red Hat Software Collections' -s "$base_image" | head -n 1)
  [ -z "$tracker_bz" ] && echo "Error: Tracker bug couldn't be guessed" && exit 1
  bug_id=$(bugzilla query -i -c rh-mariadb100-docker --blocked=$tracker_bz | head -n 1)
  [ -z "$bug_id" ] && echo "Error: Component bug couldn't be guessed" && exit 1
  git commit -am "Rebuild on base image update (${base_image})
Resolves: #${bug_id}"
  git --no-pager show
}

setup_dist_git() {
  if ! klist &>/dev/null ; then
    echo "Error: Kerberos token not found." ; popd &>/dev/null ; exit 1
  fi
  echo "=== $collection ==="
  rhpkg clone "$collection" &>/dev/null
  cd $collection
  rhpkg switch-branch "$branch" &>/dev/null
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
  buildcheck=`ls -1 ${workingdir}/logs/*buildlog`
  while ! [ "${buildcheck}" == "" ]
  do
    echo "=== waiting for these builds ==="
    date
    echo "${buildcheck}"
    sleep 120
    check_builds
    buildcheck=`ls -1 ${workingdir}/logs/*buildlog`
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
  rhpkg container-build --scratch --repo http://file.rdu.redhat.com/sdodson/aos-unsigned.repo >> ${workingdir}/logs/${collection}.buildlog 2>&1 &
  echo -n "  Waiting for createContainer taskid ."
  taskid=`grep createContainer ${workingdir}/logs/${collection}.buildlog | awk '{print $1}' | sort -u`
  while [ "${taskid}" == "" ]
  do
    echo -n "."
    sleep 5
    if grep -q -e "buildContainer (noarch) failed" -e "server startup error" ${workingdir}/logs/${collection}.buildlog ; then
      echo " error"
      echo "=== ${collection} IMAGE BUILD FAILED ==="
      echo "  exiting"
      exit 23
    fi
    taskid=`grep createContainer ${workingdir}/logs/${collection}.buildlog | awk '{print $1}' | sort -u`
  done
  echo " "
  brew watch-logs ${taskid} >> ${workingdir}/logs/${collection}.watchlog 2>&1 &

}

rebase() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  build_image
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
  rebase_base|testrebase_base) list="${base_images_list}" ;;
  rebase_s2i|testrebase_s2i) list="${s2i_images_list}" ;;
  createbz|testcreatebz) create_tracker ; list="${base_images_list}
${s2i_images_list}" ;;
  *) usage ;;
esac

echo "$list" | while read spec ; do
  [ -z "$spec" ] && continue
  export collection=$(echo "$spec" | awk '{print $1}')
  export dependency=$(echo "$spec" | awk '{print $2}')
  export branch=$(echo "$spec" | awk '{print $3}')
  case "$action" in
    rebase_base | rebase_s2i ) rebase ;;
    * ) usage ;;
  esac
done

wait_for_all_builds
