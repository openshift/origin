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

BASE_GIT_REPO="git@github.com:openshift/ose.git"
DIST_GIT_BRANCH="rhaos-3.1-rhel-7"
#DIST_GIT_BRANCH="rhaos-3.1-rhel-7-candidate"
# format:
# dist-git_name	image_dependency dist-git_branch git_repo git_path
base_images_list="
openshift-enterprise-base-docker None ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/base
openshift-enterprise-pod-docker None ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/pod
openshift-enterprise-openvswitch-docker None ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/openvswitch
openshift-enterprise-keepalived-ipfailover-docker openshift-enterprise-base-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/ipfailover/keepalived
openshift-enterprise-dockerregistry-docker openshift-enterprise-base-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/dockerregistry
openshift-enterprise-docker openshift-enterprise-base-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/ose
openshift-enterprise-haproxy-router-base-docker openshift-enterprise-base-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/router/haproxy-base
openshift-enterprise-recycler-docker openshift-enterprise-base-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/recycler
aos-f5-router-docker openshift-enterprise-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/router/f5
openshift-enterprise-deployer-docker openshift-enterprise-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/deployer
openshift-enterprise-node-docker openshift-enterprise-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/node
openshift-enterprise-sti-builder-docker openshift-enterprise-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/builder/docker/sti-builder
openshift-enterprise-docker-builder-docker openshift-enterprise-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/builder/docker/docker-builder
openshift-enterprise-haproxy-router-docker openshift-enterprise-haproxy-router-base-docker ${DIST_GIT_BRANCH} ${BASE_GIT_REPO} ose/images/router/haproxy
"

# format:
# dist-git_name	image_dependency dist-git_branch git_repo git_path
s2i_images_list="
openshift-sti-base-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/sti-base sti-base
openshift-mongodb-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/mongodb mongodb/2.4
openshift-mysql-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/mysql mysql/5.5
openshift-postgresql-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/postgresql postgresql/9.2
openshift-sti-nodejs-docker openshift-sti-base-docker ${DIST_GIT_BRANCH} https://github.com/openshift/sti-nodejs sti-nodejs/0.10
openshift-sti-perl-docker openshift-sti-base-docker ${DIST_GIT_BRANCH} https://github.com/openshift/sti-perl sti-perl/5.16
openshift-sti-php-docker openshift-sti-base-docker ${DIST_GIT_BRANCH} https://github.com/openshift/sti-php sti-php/5.5
openshift-sti-python-docker openshift-sti-base-docker ${DIST_GIT_BRANCH} https://github.com/openshift/sti-python sti-python/3.3
openshift-sti-ruby-docker openshift-sti-base-docker ${DIST_GIT_BRANCH} https://github.com/openshift/sti-ruby sti-ruby/2.0
"
# format:
# dist-git_name	image_dependency dist-git_branch git_repo git_path
logging_images_list="
logging-auth-proxy-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/sti-base sti-base
logging-deployment-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/mongodb mongodb/2.4
logging-elasticsearch-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/mysql mysql/5.5
logging-fluentd-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/postgresql postgresql/9.2
logging-kibana-docker openshift-sti-base-docker ${DIST_GIT_BRANCH} https://github.com/openshift/sti-nodejs sti-nodejs/0.10
"
# format:
# dist-git_name	image_dependency dist-git_branch git_repo git_path
metrics_images_list="
metrics-cassandra-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/origin-metrics.git origin-metrics/cassandra
metrics-deployer-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/origin-metrics.git origin-metrics/deployer
metrics-hawkular-metrics-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/origin-metrics.git origin-metrics/hawkular-metrics
metrics-heapster-docker None ${DIST_GIT_BRANCH} https://github.com/openshift/origin-metrics.git origin-metrics/heapster
"

usage() {
  echo "Usage `basename $0` [action] [--group <group>] <option>" >&2
  echo >&2
  echo "Actions:" >&2
  echo "  git_update  - Clone git and dist-git, bump release, compare" >&2
  echo "  build_container - Clone dist-git, build containers" >&2
  echo "  everything  - git_update,  build_container" >&2
  echo >&2
  echo "Groups:" >&2
  echo "  base s2i metrics logging" >&2
  echo >&2
  echo "Options:" >&2
  echo "  -h, --help          :: Show this options menu" >&2
  echo "  -v, --verbose       :: Be verbose" >&2
  echo "  -f, --force         :: Force: git_update always select continue" >&2
  echo "  --version [version] :: Change Dockerfile version e.g. 3.1.1.2" >&2
  echo "  --release [version] :: Change Dockerfile release e.g. 3" >&2
  echo "  --rhel [version]    :: Change Dockerfile RHEL version e.g. rhel7.2:7.2-35 or rhel7:latest" >&2
  echo "  --branch [version]  :: Use a certain dist-git branch  default[${DIST_GIT_BRANCH}]" >&2
  popd &>/dev/null
  exit 1
}

update_dockerfile() {
  pushd "${workingdir}/${container}" >/dev/null
  find . -name ".osbs*" -prune -o -name "Dockerfile*" -type f -print | while read line
  do
    if [ "${update_version}" == "TRUE" ] ; then
      sed -i -e "s/Version=\"v[0-9]*.[0-9]*.[0-9]*.[0-9]*\"/Version=\"v${version_version}\"/" ${line}
      sed -i -e "s/FROM \(.*\):v.*/FROM \1:v${version_version}/" ${line}
    fi
    if [ "${update_release}" == "TRUE" ] ; then
      sed -i -e "s/Release=\"[0-9]*\"/Release=\"${release_version}\"/" ${line}
    fi
    if [ "${update_rhel}" == "TRUE" ] ; then
      sed -i -e "s/FROM rhel7.*/FROM ${rhel_version}/" ${line}
    fi
  done
  popd >/dev/null
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
      mv ${line} ${package}.watchlog done/
      echo "::${package}::" >> ${workingdir}/logs/finished
      echo "::${package}::" >> ${workingdir}/logs/buildfailed
    fi
    if grep -q -e "buildContainer (noarch) completed successfully" ${line} ; then
      package=`echo ${line} | cut -d'.' -f1`
      echo "==== ${package} IMAGE COMPLETED ===="
      if grep "No package" ${package}.watchlog ; then
        echo "===== ${package}: ERRORS IN COMPLETED IMAGE see above ====="
        echo "::${package}::" >> ${workingdir}/logs/buildfailed
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
  pushd "${workingdir}/${container}" >/dev/null
  check_build_dependencies
  failedcheck=`grep ${dependency} ${workingdir}/logs/buildfailed`
  if [ "${failedcheck}" == "" ] ; then
    rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-errata.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/sdodson/aos-unsigned.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-enabled-errata.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --scratch --repo http://file.rdu.redhat.com/sdodson/aos-unsigned.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    echo -n "  Waiting for createContainer taskid ."
    taskid=`grep createContainer ${workingdir}/logs/${container}.buildlog | awk '{print $1}' | sort -u`
    while [ "${taskid}" == "" ]
    do
      echo -n "."
      sleep 5
      taskid=`grep createContainer ${workingdir}/logs/${container}.buildlog | awk '{print $1}' | sort -u`
      if grep -q -e "buildContainer (noarch) failed" -e "server startup error" ${workingdir}/logs/${container}.buildlog ; then
        echo " error"
        echo "=== ${container} IMAGE BUILD FAILED ==="
        mv ${workingdir}/logs/${container}.buildlog done/
        echo "::${package}::" >> ${workingdir}/logs/finished
        echo "::${package}::" >> ${workingdir}/logs/buildfailed
        taskid="FAILED"
      fi
    done
    echo " "
    if ! [ "${taskid}" == "FAILED" ] ; then
      brew watch-logs ${taskid} >> ${workingdir}/logs/${container}.watchlog 2>&1 &
    fi
  else
    echo "  depedency error: ${dependency} failed to build"
    echo "=== ${container} IMAGE BUILD FAILED ==="
    echo "::${package}::" >> ${workingdir}/logs/finished
    echo "::${package}::" >> ${workingdir}/logs/buildfailed
  fi
  popd >/dev/null
}

show_git_diffs() {
  pushd "${workingdir}/${container}" >/dev/null
  echo "  ---- Checking Dockerfile changes ----"
  find . -name ".osbs*" -prune -o -name "Dockerfile*" -type f -print | while read line
  do
    diff --label ${line} --label ${path}/${line} -u ${line} ${workingdir}/${path}/${line} >> .osbs-logs/${line}.diff.new
    newdiff=`diff .osbs-logs/${line}.diff .osbs-logs/${line}.diff.new`
    if [ "${newdiff}" == "" ] ; then
      rm -f .osbs-logs/${line}.diff.new
    else
      echo "${newdiff}"
      echo " "
      if [ "${FORCE}" == "TRUE" ] ; then
        echo "  Force Option Selected - Assuming Continue"
        mv -f .osbs-logs/${line}.diff.new .osbs-logs/${line}.diff ; git add .osbs-logs/${line}.diff ; rhpkg commit -p -m "Updating dockerfile diff" > /dev/null
      else
        echo "(c)ontinue [replace old diff], (i)gnore [leave old diff], (q)uit [exit script] : "
        read choice < /dev/tty
        case ${choice} in
          c | C | continue ) mv -f .osbs-logs/${line}.diff.new .osbs-logs/${line}.diff ; git add .osbs-logs/${line}.diff ; rhpkg commit -p -m "Updating dockerfile diff" > /dev/null ;;
          i | I | ignore ) rm -f .osbs-logs/${line}.diff.new ;;
          q | Q | quit ) break;;
          * ) echo "${choice} not and option.  Assuming ignore" ; rm -f .osbs-logs/${line}.diff.new ;;
          #* ) echo "${choice} not and option.  Assuming continue" ;  mv -f .osbs-logs/${line}.diff.new .osbs-logs/${line}.diff ; git add .osbs-logs/${line}.diff ; rhpkg commit -p -m "Updating dockerfile diff" ;;
        esac
      fi
    fi
  done
  echo "  ---- Checking Non-Dockerfile changes ----"
  find . -name ".git*" -prune -o -name ".osbs*" -prune -o -name "Dockerfile*" -prune -o -type f -print | while read line
  do
    diff -u ${line} ${workingdir}/${path}/${line}
  done
  echo "  ---- Checking files added or removed ----"
  diff --brief -r ${workingdir}/${container} ${workingdir}/${path} | grep -v -e Dockerfile -e git -e osbs
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
  update_dockerfile
  show_git_diffs
  popd >/dev/null
}

if [ "$#" -lt 1 ] ; then
  usage
fi

# Get our arguments
while [[ "$#" -ge 1 ]]
do
key="$1"
case $key in
    everything | git_update | build_container)
      export list="${base_images_list}"
      export action="${key}"
      ;;
    --group)
      GROUP="$2"
      shift
      ;;
    --version)
      version_version="$2"
      export update_version="TRUE"
      shift
      ;;
    --release)
      release_version="$2"
      export update_release="TRUE"
      shift
      ;;
    --rhel)
      rhel_version="$2"
      export update_rhel="TRUE"
      shift
      ;;
    --branch)
      DIST_GIT_BRANCH="$2"
      shift
      ;;
      -v|--verbose)
        export VERBOSE="TRUE"
        ;;
      -f|--force)
        export FORCE="TRUE"
        ;;
    -h|--help)
      usage  # unknown option
      ;;
    *)
      echo "Unknown Option: ${key}"
      usage  # unknown option
      exit 4
      ;;
esac
shift # past argument or value
done

case $GROUP in
  base)
    export list="${base_images_list}"
    ;;
  s2i)
    export list="${s2i_images_list}"
    ;;
  logging)
    export list="${logging_images_list}"
    ;;
  metrics)
    export list="${metrics_images_list}"
    ;;
  *)
    echo "Unknown, or no group: ${GROUP}"
    usage  # unknown group
    exit 3
    ;;
esac

workingdir=$(mktemp -d /var/tmp/rebuild-images-XXXXXX)
pushd "${workingdir}" &>/dev/null
mkdir -p logs/done
echo "::None::" >> logs/finished
touch logs/buildfailed
echo "Using working directory: ${workingdir}"

echo "${list}" | while read spec
do
  [ -z "$spec" ] && continue
  export branch="${DIST_GIT_BRANCH}"
  export container=$(echo "$spec" | awk '{print $1}')
  export dependency=$(echo "$spec" | awk '{print $2}')
  export repo=$(echo "$spec" | awk '{print $4}')
  export path=$(echo "$spec" | awk '{print $5}')
  case "$action" in
    build_container )
      build_container
      ;;
    git_update )
       git_update
       ;;
    everything )
      git_update
      build_container
      ;;
    * )
      usage
      exit 2
      ;;
  esac
done

wait_for_all_builds
