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
SCRATCH_OPTION=""
BUILD_REPO="http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-enabled.repo"
COMMIT_MESSAGE="3.1.1 Signed Image Release"
#DIST_GIT_BRANCH="rhaos-3.1-rhel-7-candidate"

declare -A packagekey
# format:
# dist-git_name	image_dependency git_repo git_path
packagekey['aos-f5-router-docker']="aos-f5-router-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/router/f5"
packagekey['image-inspector-docker']="image-inspector-docker None None None"
packagekey['logging-auth-proxy-docker']="logging-auth-proxy-docker None None None"
packagekey['logging-deployment-docker']="logging-deployment-docker None None None"
packagekey['logging-elasticsearch-docker']="logging-elasticsearch-docker None None None"
packagekey['logging-fluentd-docker']="logging-fluentd-docker None None None"
packagekey['logging-kibana-docker']="logging-kibana-docker None None None"
packagekey['metrics-cassandra-docker']="metrics-cassandra-docker None https://github.com/openshift/origin-metrics.git origin-metrics/cassandra"
packagekey['metrics-deployer-docker']="metrics-deployer-docker None https://github.com/openshift/origin-metrics.git origin-metrics/deployer"
packagekey['metrics-hawkular-metrics-docker']="metrics-hawkular-metrics-docker None https://github.com/openshift/origin-metrics.git origin-metrics/hawkular-metrics"
packagekey['metrics-heapster-docker']="metrics-heapster-docker None https://github.com/openshift/origin-metrics.git origin-metrics/heapster"
packagekey['openshift-enterprise-base-docker']="openshift-enterprise-base-docker None ${BASE_GIT_REPO} ose/images/base"
packagekey['openshift-enterprise-deployer-docker']="openshift-enterprise-deployer-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/deployer"
packagekey['openshift-enterprise-docker']="openshift-enterprise-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/ose"
packagekey['openshift-enterprise-docker-builder-docker']="openshift-enterprise-docker-builder-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/builder/docker/docker-builder"
packagekey['openshift-enterprise-dockerregistry-docker']="openshift-enterprise-dockerregistry-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/dockerregistry"
packagekey['openshift-enterprise-haproxy-router-base-docker']="openshift-enterprise-haproxy-router-base-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/router/haproxy-base"
packagekey['openshift-enterprise-haproxy-router-docker']="openshift-enterprise-haproxy-router-docker openshift-enterprise-haproxy-router-base-docker ${BASE_GIT_REPO} ose/images/router/haproxy"
packagekey['openshift-enterprise-keepalived-ipfailover-docker']="openshift-enterprise-keepalived-ipfailover-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/ipfailover/keepalived"
packagekey['openshift-enterprise-node-docker']="openshift-enterprise-node-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/node"
packagekey['openshift-enterprise-openvswitch-docker']="openshift-enterprise-openvswitch-docker None ${BASE_GIT_REPO} ose/images/openvswitch"
packagekey['openshift-enterprise-pod-docker']="openshift-enterprise-pod-docker None ${BASE_GIT_REPO} ose/images/pod"
packagekey['openshift-enterprise-recycler-docker']="openshift-enterprise-recycler-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/recycler"
packagekey['openshift-enterprise-sti-builder-docker']="openshift-enterprise-sti-builder-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/builder/docker/sti-builder"
packagekey['openshift-jenkins-docker']="openshift-jenkins-docker None https://github.com/openshift/mongodb mongodb/2.4"
packagekey['openshift-mongodb-docker']="openshift-mongodb-docker None https://github.com/openshift/mongodb mongodb/2.4"
packagekey['openshift-mysql-docker']="openshift-mysql-docker None https://github.com/openshift/mysql mysql/5.5"
packagekey['openshift-postgresql-docker']="openshift-postgresql-docker None https://github.com/openshift/postgresql postgresql/9.2"
packagekey['openshift-sti-base-docker']="openshift-sti-base-docker None https://github.com/openshift/sti-base sti-base"
packagekey['openshift-sti-nodejs-docker']="openshift-sti-nodejs-docker openshift-sti-base-docker https://github.com/openshift/sti-nodejs sti-nodejs/0.10"
packagekey['openshift-sti-perl-docker']="openshift-sti-perl-docker openshift-sti-base-docker https://github.com/openshift/sti-perl sti-perl/5.16"
packagekey['openshift-sti-php-docker']="openshift-sti-php-docker openshift-sti-base-docker https://github.com/openshift/sti-php sti-php/5.5"
packagekey['openshift-sti-python-docker']="openshift-sti-python-docker openshift-sti-base-docker https://github.com/openshift/sti-python sti-python/3.3"
packagekey['openshift-sti-ruby-docker']="openshift-sti-ruby-docker openshift-sti-base-docker https://github.com/openshift/sti-ruby sti-ruby/2.0"

base_images_list="
openshift-enterprise-base-docker
openshift-enterprise-openvswitch-docker
openshift-enterprise-pod-docker
openshift-enterprise-docker
openshift-enterprise-haproxy-router-base-docker
openshift-enterprise-dockerregistry-docker
openshift-enterprise-keepalived-ipfailover-docker
openshift-enterprise-recycler-docker
aos-f5-router-docker
openshift-enterprise-deployer-docker
openshift-enterprise-node-docker
openshift-enterprise-sti-builder-docker
openshift-enterprise-docker-builder-docker
openshift-enterprise-haproxy-router-docker"

s2i_images_list="
openshift-sti-base-docker
image-inspector-docker
openshift-jenkins-docker
openshift-mongodb-docker
openshift-mysql-docker
openshift-postgresql-docker
openshift-sti-nodejs-docker
openshift-sti-perl-docker
openshift-sti-php-docker
openshift-sti-python-docker
openshift-sti-ruby-docker"

logging_images_list="
logging-auth-proxy-docker
logging-deployment-docker
logging-elasticsearch-docker
logging-fluentd-docker
logging-kibana-docker"

metrics_images_list="metrics-cassandra-docker
metrics-deployer-docker
metrics-hawkular-metrics-docker
metrics-heapster-docker"

usage() {
  echo "Usage `basename $0` [action] <options>" >&2
  echo >&2
  echo "Actions:" >&2
  echo "  docker_update   :: Clone dist-git, bump version, release, or rhel" >&2
  echo "  git_compare     :: Clone dist-git and git, compare files and Dockerfile" >&2
  echo "  build_container :: Clone dist-git, build containers" >&2
  echo "  test            :: Display what packages would be worked on" >&2
  echo >&2
  echo "Options:" >&2
  echo "  -h, --help          :: Show this options menu" >&2
  echo "  -v, --verbose       :: Be verbose" >&2
  echo "  -f, --force         :: Force: always do dist-git commits " >&2
  echo "  -i, --ignore        :: Ignore: do not do dist-git commits " >&2
  echo "  --scrach            :: Do a scratch build" >&2
  echo "  --group [group]     :: Which group list to use (base s2i metrics logging)" >&2
  echo "  --package [package] :: Which package to use e.g. openshift-enterprise-pod-docker" >&2
  echo "  --version [version] :: Change Dockerfile version e.g. 3.1.1.2" >&2
  echo "  --release [version] :: Change Dockerfile release e.g. 3" >&2
  echo "  --rhel [version]    :: Change Dockerfile RHEL version e.g. rhel7.2:7.2-35 or rhel7:latest" >&2
  echo "  --branch [version]  :: Use a certain dist-git branch  default[${DIST_GIT_BRANCH}]" >&2
  echo "  --repo [Repo URL]   :: Use a certain yum repo  default[${BUILD_REPO}]" >&2
  echo >&2
  echo "Note: --group and --package can be used multiple times" >&2
  popd &>/dev/null
  exit 1
}

add_to_list() {
  NEWLINE=$'\n'
  export list+="${packagekey[${1}]}${NEWLINE}"
  if [ "${VERBOSE}" == "TRUE" ] ; then
    echo "----------"
    echo ${list}
  fi
}

add_group_to_list() {
  case ${1} in
    base)
      add_to_list openshift-enterprise-base-docker
      add_to_list openshift-enterprise-openvswitch-docker
      add_to_list openshift-enterprise-pod-docker
      add_to_list openshift-enterprise-docker
      add_to_list openshift-enterprise-haproxy-router-base-docker
      add_to_list openshift-enterprise-dockerregistry-docker
      add_to_list openshift-enterprise-keepalived-ipfailover-docker
      add_to_list openshift-enterprise-recycler-docker
      add_to_list aos-f5-router-docker
      add_to_list openshift-enterprise-deployer-docker
      add_to_list openshift-enterprise-node-docker
      add_to_list openshift-enterprise-sti-builder-docker
      add_to_list openshift-enterprise-docker-builder-docker
      add_to_list openshift-enterprise-haproxy-router-docker
      ;;
    s2i)
      add_to_list openshift-sti-base-docker
      add_to_list image-inspector-docker
      add_to_list openshift-jenkins-docker
      add_to_list openshift-mongodb-docker
      add_to_list openshift-mysql-docker
      add_to_list openshift-postgresql-docker
      add_to_list openshift-sti-nodejs-docker
      add_to_list openshift-sti-perl-docker
      add_to_list openshift-sti-php-docker
      add_to_list openshift-sti-python-docker
      add_to_list openshift-sti-ruby-docker
      ;;
    logging)
      add_to_list logging-auth-proxy-docker
      add_to_list logging-deployment-docker
      add_to_list logging-elasticsearch-docker
      add_to_list logging-fluentd-docker
      add_to_list logging-kibana-docker
      ;;
    metrics)
      add_to_list metrics-cassandra-docker
      add_to_list metrics-deployer-docker
      add_to_list metrics-hawkular-metrics-docker
      add_to_list metrics-heapster-docker
      ;;
  esac
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
    else
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
  depcheck=`grep ::${dependency}:: ${workingdir}/logs/finished`
  while [ "${depcheck}" == "" ]
  do
    now=`date`
    echo "Waiting for ${dependency} to be built - ${now}"
    sleep 120
    check_builds
    depcheck=`grep ::${dependency}:: ${workingdir}/logs/finished`
  done
}

build_image() {
  pushd "${workingdir}/${container}" >/dev/null
  check_build_dependencies
  failedcheck=`grep ::${dependency}:: ${workingdir}/logs/buildfailed`
  if [ "${failedcheck}" == "" ] ; then
    rhpkg container-build ${SCRATCH_OPTION} --repo ${BUILD_REPO} >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-errata.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/sdodson/aos-unsigned.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-enabled.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
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
        echo "::${container}::" >> ${workingdir}/logs/finished
        echo "::${container}::" >> ${workingdir}/logs/buildfailed
        taskid="FAILED"
      fi
    done
    echo " "
    if ! [ "${taskid}" == "FAILED" ] ; then
      brew watch-logs ${taskid} >> ${workingdir}/logs/${container}.watchlog 2>&1 &
    fi
  else
    echo "  dependency error: ${dependency} failed to build"
    echo "=== ${container} IMAGE BUILD FAILED ==="
    echo "::${container}::" >> ${workingdir}/logs/finished
    echo "::${container}::" >> ${workingdir}/logs/buildfailed
  fi
  popd >/dev/null
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

show_git_diffs() {
  pushd "${workingdir}/${container}" >/dev/null
  echo "  ---- Checking files added or removed ----"
  diff --brief -r ${workingdir}/${container} ${workingdir}/${path} | grep -v -e Dockerfile -e git -e osbs
  echo "  ---- Checking Non-Dockerfile changes ----"
  find . -name ".git*" -prune -o -name ".osbs*" -prune -o -name "Dockerfile*" -prune -o -type f -print | while read line
  do
    diff -u ${line} ${workingdir}/${path}/${line}
  done
  echo "  ---- Checking Dockerfile changes ----"
  diff --label Dockerfile --label ${path}/Dockerfile -u Dockerfile ${workingdir}/${path}/Dockerfile >> .osbs-logs/Dockerfile.diff.new
  newdiff=`diff .osbs-logs/Dockerfile.diff .osbs-logs/Dockerfile.diff.new`
  if [ "${newdiff}" == "" ] ; then
    rm -f .osbs-logs/Dockerfile.diff.new
  else
    echo "${newdiff}"
    echo " "
    if [ "${FORCE}" == "TRUE" ] ; then
      echo "  Force Option Selected - Assuming Continue"
      mv -f .osbs-logs/Dockerfile.diff.new .osbs-logs/Dockerfile.diff ; git add .osbs-logs/Dockerfile.diff ; rhpkg commit -p -m "${COMMIT_MESSAGE} ${version_version} ${release_version} ${rhel_version}" > /dev/null
    else
      echo "(c)ontinue [replace old diff], (i)gnore [leave old diff], (q)uit [exit script] : "
      read choice < /dev/tty
      case ${choice} in
        c | C | continue ) mv -f .osbs-logs/Dockerfile.diff.new .osbs-logs/Dockerfile.diff ; git add .osbs-logs/Dockerfile.diff ; rhpkg commit -p -m "${COMMIT_MESSAGE} ${version_version} ${release_version} ${rhel_version}" > /dev/null ;;
        i | I | ignore ) rm -f .osbs-logs/Dockerfile.diff.new ;;
        q | Q | quit ) break;;
        * ) echo "${choice} not and option.  Assuming ignore" ; rm -f .osbs-logs/Dockerfile.diff.new ;;
        #* ) echo "${choice} not and option.  Assuming continue" ;  mv -f .osbs-logs/Dockerfile.diff.new .osbs-logs/Dockerfile.diff ; git add .osbs-logs/Dockerfile.diff ; rhpkg commit -p -m "Updating dockerfile diff" ;;
      esac
    fi
  fi
  popd >/dev/null

}

show_dockerfile_diffs() {
  pushd "${workingdir}/${container}" >/dev/null
  if ! [ -d .osbs-logs ] ; then
    mkdir .osbs-logs
  fi
  if ! [ -f .osbs-logs/Dockerfile.last ] ; then
    touch .osbs-logs/Dockerfile.last
  fi
  echo "  ---- Checking Dockerfile changes ----"
  newdiff=`diff -u Dockerfile .osbs-logs/Dockerfile.last`
  if [ "${newdiff}" == "" ] ; then
    echo "    None "
  else
    echo "${newdiff}"
    echo " "
    if [ "${FORCE}" == "TRUE" ] ; then
      echo "  Force Option Selected - Assuming Continue"
      /bin/cp -f Dockerfile .osbs-logs/Dockerfile.last
      git add .osbs-logs/Dockerfile.last
      rhpkg commit -p -m "${COMMIT_MESSAGE} ${version_version} ${release_version} ${rhel_version}" > /dev/null
    elif [ "${IGNORE}" == "TRUE" ] ; then
      echo "  Ignore Option Selected - Not committing"
    else
      echo "(c)ontinue [replace old diff], (i)gnore [leave old diff], (q)uit [exit script] : "
      read choice < /dev/tty
      case ${choice} in
        c | C | continue )
          /bin/cp -f Dockerfile .osbs-logs/Dockerfile.last
          git add .osbs-logs/Dockerfile.last
          rhpkg commit -p -m "${COMMIT_MESSAGE} ${version_version} ${release_version} ${rhel_version}" > /dev/null
          ;;
        i | I | ignore )
          ;;
        q | Q | quit )
          break
          ;;
        * )
          echo "${choice} not and option.  Assuming ignore"
          ;;
      esac
    fi
  fi
  popd >/dev/null

}

build_container() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  build_image
  popd >/dev/null
}

git_compare() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  setup_git_repo
  show_git_diffs
  popd >/dev/null
}

docker_update() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  update_dockerfile
  show_dockerfile_diffs
  popd >/dev/null
}

test_function() {
  echo "container: ${container} dependency: ${dependency} branch: ${branch}"
}

if [ "$#" -lt 1 ] ; then
  usage
fi

# Get our arguments
while [[ "$#" -ge 1 ]]
do
key="$1"
case $key in
    git_compare | docker_update | build_container | test)
      export action="${key}"
      ;;
    --group)
      add_group_to_list "$2"
      shift
      ;;
    --package)
      add_to_list "$2"
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
    --repo)
      BUILD_REPO="$2"
      shift
      ;;
    --message)
      COMMIT_MESSAGE="$2"
      shift
      ;;
    -v|--verbose)
      export VERBOSE="TRUE"
      ;;
    -f|--force)
      export FORCE="TRUE"
      ;;
    -i|--ignore)
      export IGNORE="TRUE"
      ;;
    --scratch)
      export SCRATCH_OPTION=" --scratch "
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
  export repo=$(echo "$spec" | awk '{print $3}')
  export path=$(echo "$spec" | awk '{print $4}')
  case "$action" in
    build_container )
      build_container
      ;;
    git_compare )
      git_compare
      ;;
    docker_update )
      docker_update
      ;;
    test )
      test_function
      ;;
    * )
      usage
      exit 2
      ;;
  esac
done

wait_for_all_builds
