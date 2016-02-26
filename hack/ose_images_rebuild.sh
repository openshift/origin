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
DIST_GIT_BRANCH="rhaos-3.2-rhel-7"
#DIST_GIT_BRANCH="rhaos-3.1-rhel-7"
#DIST_GIT_BRANCH="rhaos-3.2-rhel-7-candidate"
SCRATCH_OPTION=""
BUILD_REPO="http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-building.repo"
COMMIT_MESSAGE="Update dockerfile"
#DIST_GIT_BRANCH="rhaos-3.1-rhel-7-candidate"
OSBS_REGISTRY=brew-pulp-docker01.web.prod.ext.phx2.redhat.com:8888
#OSBS_REGISTRY=rcm-img-docker01.build.eng.bos.redhat.com:5001
PUSH_REGISTRY=registry.qe.openshift.com

packagelist=""
declare -A packagekey
# format:
# dist-git_name	image_dependency git_repo git_path
packagekey['aos-f5-router-docker']="aos-f5-router-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/router/f5 openshift3/ose-f5-router aep3_beta/aep-f5-router"
packagekey['image-inspector-docker']="image-inspector-docker None None None openshift3/image-inspector aep3_beta/image-inspector"
packagekey['logging-auth-proxy-docker']="logging-auth-proxy-docker None None None openshift3/logging-auth-proxy aep3_beta/logging-auth-proxy"
packagekey['logging-deployment-docker']="logging-deployment-docker None None None openshift3/logging-deployment aep3_beta/logging-deployment"
packagekey['logging-elasticsearch-docker']="logging-elasticsearch-docker None None None openshift3/logging-elasticsearch aep3_beta/logging-elasticsearch"
packagekey['logging-fluentd-docker']="logging-fluentd-docker None None None openshift3/logging-fluentd aep3_beta/logging-fluentd"
packagekey['logging-kibana-docker']="logging-kibana-docker None None None openshift3/logging-kibana aep3_beta/logging-kibana"
packagekey['metrics-cassandra-docker']="metrics-cassandra-docker None https://github.com/openshift/origin-metrics.git origin-metrics/cassandra openshift3/metrics-cassandra aep3_beta/metrics-cassandra"
packagekey['metrics-deployer-docker']="metrics-deployer-docker None https://github.com/openshift/origin-metrics.git origin-metrics/deployer openshift3/metrics-deployer aep3_beta/metrics-deployer"
packagekey['metrics-hawkular-metrics-docker']="metrics-hawkular-metrics-docker None https://github.com/openshift/origin-metrics.git origin-metrics/hawkular-metrics openshift3/metrics-hawkular-metrics aep3_beta/metrics-hawkular-metrics"
packagekey['metrics-heapster-docker']="metrics-heapster-docker None https://github.com/openshift/origin-metrics.git origin-metrics/heapster openshift3/metrics-heapster aep3_beta/metrics-heapster"
packagekey['openshift-enterprise-base-docker']="openshift-enterprise-base-docker None ${BASE_GIT_REPO} ose/images/base none none"
packagekey['openshift-enterprise-deployer-docker']="openshift-enterprise-deployer-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/deployer openshift3/ose-deployer aep3_beta/aep-deployer"
packagekey['openshift-enterprise-docker']="openshift-enterprise-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/ose openshift3/ose aep3_beta/aep"
packagekey['openshift-enterprise-docker-builder-docker']="openshift-enterprise-docker-builder-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/builder/docker/docker-builder openshift3/ose-docker-builder none"
packagekey['openshift-enterprise-dockerregistry-docker']="openshift-enterprise-dockerregistry-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/dockerregistry openshift3/ose-docker-registry aep3_beta/aep-docker-registry"
packagekey['openshift-enterprise-haproxy-router-base-docker']="openshift-enterprise-haproxy-router-base-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/router/haproxy-base none none"
packagekey['openshift-enterprise-haproxy-router-docker']="openshift-enterprise-haproxy-router-docker openshift-enterprise-haproxy-router-base-docker ${BASE_GIT_REPO} ose/images/router/haproxy openshift3/ose-haproxy-router aep3_beta/aep-haproxy-router"
packagekey['openshift-enterprise-keepalived-ipfailover-docker']="openshift-enterprise-keepalived-ipfailover-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/ipfailover/keepalived openshift3/ose-keepalived-ipfailover aep3_beta/aep-keepalived-ipfailover"
packagekey['openshift-enterprise-node-docker']="openshift-enterprise-node-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/node openshift3/node aep3_beta/node"
packagekey['openshift-enterprise-openvswitch-docker']="openshift-enterprise-openvswitch-docker None ${BASE_GIT_REPO} ose/images/openvswitch openshift3/openvswitch none"
packagekey['openshift-enterprise-pod-docker']="openshift-enterprise-pod-docker None ${BASE_GIT_REPO} ose/images/pod openshift3/ose-pod aep3_beta/aep-pod"
packagekey['openshift-enterprise-recycler-docker']="openshift-enterprise-recycler-docker openshift-enterprise-base-docker ${BASE_GIT_REPO} ose/images/recycler openshift3/ose-recycler aep3_beta/aep-recycler"
packagekey['openshift-enterprise-sti-builder-docker']="openshift-enterprise-sti-builder-docker openshift-enterprise-docker ${BASE_GIT_REPO} ose/images/builder/docker/sti-builder openshift3/ose-sti-builder none"
packagekey['openshift-jenkins-docker']="openshift-jenkins-docker None https://github.com/openshift/mongodb mongodb/2.4 openshift3/jenkins-1-rhel7 none"
packagekey['openshift-mongodb-docker']="openshift-mongodb-docker None https://github.com/openshift/mongodb mongodb/2.4 openshift3/mongodb-24-rhel7 none"
packagekey['openshift-mysql-docker']="openshift-mysql-docker None https://github.com/openshift/mysql mysql/5.5 openshift3/mysql-55-rhel7 none"
packagekey['openshift-postgresql-docker']="openshift-postgresql-docker None https://github.com/openshift/postgresql postgresql/9.2 openshift3/postgresql-92-rhel7 none"
packagekey['openshift-sti-base-docker']="openshift-sti-base-docker None https://github.com/openshift/sti-base sti-base none none"
packagekey['openshift-sti-nodejs-docker']="openshift-sti-nodejs-docker openshift-sti-base-docker https://github.com/openshift/sti-nodejs sti-nodejs/0.10 openshift3/nodejs-010-rhel7 none"
packagekey['openshift-sti-perl-docker']="openshift-sti-perl-docker openshift-sti-base-docker https://github.com/openshift/sti-perl sti-perl/5.16 openshift3/perl-516-rhel7 none"
packagekey['openshift-sti-php-docker']="openshift-sti-php-docker openshift-sti-base-docker https://github.com/openshift/sti-php sti-php/5.5 openshift3/php-55-rhel7 none"
packagekey['openshift-sti-python-docker']="openshift-sti-python-docker openshift-sti-base-docker https://github.com/openshift/sti-python sti-python/3.3 openshift3/python-33-rhel7 none"
packagekey['openshift-sti-ruby-docker']="openshift-sti-ruby-docker openshift-sti-base-docker https://github.com/openshift/sti-ruby sti-ruby/2.0 openshift3/ruby-20-rhel7 none"

usage() {
  echo "Usage `basename $0` [action] <options>" >&2
  echo >&2
  echo "Actions:" >&2
  echo "  build_container :: Clone dist-git, build containers" >&2
  echo "  docker_update   :: Clone dist-git, update version, release, or rhel" >&2
  echo "  bump_and_build  :: docker_update, build containers" >&2
  echo "  docker_backfill :: Copy dist-git Dockerfile to git Dockerfile.product" >&2
  echo "  git_compare     :: Clone dist-git and git, compare files and Dockerfile" >&2
  echo "  push_images     :: Push images to qe-registry" >&2
  echo "  make_yaml       :: Print out yaml from Dockerfile for release" >&2
  echo "  list            :: Display full list of packages / images" >&2
  echo "  test            :: Display what packages would be worked on" >&2
  echo >&2
  echo "Options:" >&2
  echo "  -h, --help          :: Show this options menu" >&2
  echo "  -v, --verbose       :: Be verbose" >&2
  echo "  -f, --force         :: Force: always do dist-git commits " >&2
  echo "  -i, --ignore        :: Ignore: do not do dist-git commits " >&2
  echo "  -d, --deps          :: Dependents: Also do the dependents" >&2
  echo "  -n, --notlatest     :: Do not tag or push as latest" >&2
  echo "  --scratch           :: Do a scratch build" >&2
  echo "  --group [group]     :: Which group list to use (base sti metrics logging misc all)" >&2
  echo "  --package [package] :: Which package to use e.g. openshift-enterprise-pod-docker" >&2
  echo "  --version [version] :: Change Dockerfile version e.g. 3.1.1.2" >&2
  echo "  --release [version] :: Change Dockerfile release e.g. 3" >&2
  echo "  --bump_release      :: Change Dockerfile release by 1 e.g. 3->4" >&2
  echo "  --rhel [version]    :: Change Dockerfile RHEL version e.g. rhel7.2:7.2-35 or rhel7:latest" >&2
  echo "  --branch [version]  :: Use a certain dist-git branch  default[${DIST_GIT_BRANCH}]" >&2
  echo "  --repo [Repo URL]   :: Use a certain yum repo  default[${BUILD_REPO}]" >&2
  echo >&2
  echo "Note: --group and --package can be used multiple times" >&2
  popd &>/dev/null
  exit 1
}

add_to_list() {
  if ! [[ ${packagelist} =~ "::${1}::" ]] ; then
    NEWLINE=$'\n'
    export list+="${packagekey[${1}]}${NEWLINE}"
    export packagelist+=" ::${1}::"
    if [ "${VERBOSE}" == "TRUE" ] ; then
      echo "----------"
      echo ${packagelist}
      echo ${list}
    fi
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
    sti)
      add_to_list openshift-sti-base-docker
      add_to_list openshift-sti-nodejs-docker
      add_to_list openshift-sti-perl-docker
      add_to_list openshift-sti-php-docker
      add_to_list openshift-sti-python-docker
      add_to_list openshift-sti-ruby-docker
      ;;
    misc)
      add_to_list openshift-jenkins-docker
      add_to_list openshift-mongodb-docker
      add_to_list openshift-mysql-docker
      add_to_list openshift-postgresql-docker
      add_to_list image-inspector-docker
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
    rhpkg container-build ${SCRATCH_OPTION} --repo ${BUILD_REPO} >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-signed-errata.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-errata.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/sdodson/aos-unsigned.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    echo -n "  Waiting for createContainer taskid ."
    taskid=`grep createContainer ${workingdir}/logs/${container}.buildlog | awk '{print $1}' | sort -u`
    while [ "${taskid}" == "" ]
    do
      echo -n "."
      sleep 5
      taskid=`grep createContainer ${workingdir}/logs/${container}.buildlog | awk '{print $1}' | sort -u`
      if grep -q -e "Unknown build target:" -e "buildContainer (noarch) failed" -e "server startup error" ${workingdir}/logs/${container}.buildlog ; then
        echo " error"
        echo "=== ${container} IMAGE BUILD FAILED ==="
        mv ${workingdir}/logs/${container}.buildlog ${workingdir}/logs/done/
        echo "::${container}::" >> ${workingdir}/logs/finished
        echo "::${container}::" >> ${workingdir}/logs/buildfailed
        taskid="FAILED"
      fi
    done
    echo " "
    if ! [ "${taskid}" == "FAILED" ] ; then
      brew watch-logs ${taskid} >> ${workingdir}/logs/${container}.watchlog 2>&1 &
    fi
}

start_build_image() {
  pushd "${workingdir}/${container}" >/dev/null
  if [ "${FORCE}" == "TRUE" ] || [[ ${parent} =~ "::${container}::" ]] ; then
    build_image
  else
    check_build_dependencies
    failedcheck=`grep ::${dependency}:: ${workingdir}/logs/buildfailed`
    if [ "${failedcheck}" == "" ] ; then
      build_image
    else
      echo "  dependency error: ${dependency} failed to build"
      echo "=== ${container} IMAGE BUILD FAILED ==="
      echo "::${container}::" >> ${workingdir}/logs/finished
      echo "::${container}::" >> ${workingdir}/logs/buildfailed
    fi
  fi
  popd >/dev/null
}

update_dockerfile() {
  pushd "${workingdir}/${container}" >/dev/null
  find . -name ".osbs*" -prune -o -name "Dockerfile*" -type f -print | while read line
  do
    if [ "${update_version}" == "TRUE" ] ; then
      sed -i -e "s/Version=\".*\"/Version=\"${version_version}\"/" ${line}
      sed -i -e "s/FROM \(.*\):v.*/FROM \1:${version_version}/" ${line}
    fi
    if [ "${update_release}" == "TRUE" ] ; then
      sed -i -e "s/Release=\".*\"/Release=\"${release_version}\"/" ${line}
    fi
    if [ "${bump_release}" == "TRUE" ] ; then
      old_release_version=$(grep Release= ${line} | cut -d'=' -f2 | cut -d'"' -f2 )
      let new_release_version=$old_release_version+1
      sed -i -e "s/Release=\".*\"/Release=\"${new_release_version}\"/" ${line}
    fi
    if [ "${update_rhel}" == "TRUE" ] ; then
      sed -i -e "s/FROM rhel7.*/FROM ${rhel_version}/" ${line}
    fi
  done
  popd >/dev/null
}

show_git_diffs() {
  pushd "${workingdir}/${container}" >/dev/null
  echo "  ---- Checking files changed, added or removed ----"
  extra_check=$(diff --brief -r ${workingdir}/${container} ${workingdir}/${path} | grep -v -e Dockerfile -e git -e osbs )
  if ! [ "${extra_check}" == "" ] ; then
    echo "${extra_check}"
  fi
  differ_check=$(echo "${extra_check}" | grep " differ")
  new_file=$(echo "${extra_check}" | grep "Only in ${workingdir}/${path}")
  old_file=$(echo "${extra_check}" | grep "Only in ${workingdir}/${container}")
  if ! [ "${differ_check}" == "" ] ; then
    echo "  ---- Non-Dockerfile changes ----"
    echo "${differ_check}" | while read differ_line
    do
      myold_file=$(echo "${differ_line}" | awk '{print $2}')
      mynew_file=$(echo "${differ_line}" | awk '{print $4}')
      diff -u ${myold_file} ${mynew_file}
      cp -vf ${mynew_file} ${myold_file}
      git add ${myold_file}
    done
  fi
  if ! [ "${old_file}" == "" ] ; then
    echo "  ---- Removed Non-Dockerfiles ----"
    echo "${old_file}" | while read old_file_line
    do
      myold_file=$(echo "${old_file_line}" | awk '{print $3}')
      git rm ${myold_file}
    done
  fi
  if ! [ "${new_file}" == "" ] ; then
    echo "  ---- New Non-Dockerfiles ----"
    echo " New files must be added by hand - sorry about that"
    echo "${new_file}"
    working_path="${workingdir}/${path}"
    echo "${new_file}" | while read new_file_line
    do
      mynew_file=$(echo "${new_file_line}" | awk '{print $3}')
      mynew_file_trim="${mynew_file#$working_path}"
      cp -v ${mynew_file} ${workingdir}/${container}/${mynew_file_trim}
      git add ${workingdir}/${container}/${mynew_file_trim}
    done
  fi
  echo "  ---- Checking Dockerfile changes ----"
  diff --label Dockerfile --label ${path}/Dockerfile -u Dockerfile ${workingdir}/${path}/Dockerfile >> .osbs-logs/Dockerfile.diff.new
  newdiff=`diff .osbs-logs/Dockerfile.diff .osbs-logs/Dockerfile.diff.new`
  if [ "${newdiff}" == "" ] ; then
    rm -f .osbs-logs/Dockerfile.diff.new
  fi
  if ! [ "${newdiff}" == "" ] || ! [ "${extra_check}" == "" ] ; then
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

backfill_dockerfile() {
  pushd "${workingdir}/${container}" >/dev/null
  echo "  ---- Dockerfile.product changes ----"
  diff --label Dockerfile --label ${path}/Dockerfile.product -u Dockerfile ${workingdir}/${path}/Dockerfile.product
  if [ "${FORCE}" == "TRUE" ] ; then
    echo "  Force Option Selected - Assuming Continue"
    cp -f Dockerfile ${workingdir}/${path}/Dockerfile.product
    pushd ${workingdir}/${path} >/dev/null
    git add Dockerfile.product
    popd >/dev/null
  else
    echo "(c)ontinue [replace old Dockerfile.product], (i)gnore [leave old Dockerfile.product] : "
    read choice < /dev/tty
    case ${choice} in
      c | C | continue )
        cp -f Dockerfile ${workingdir}/${path}/Dockerfile.product
        pushd ${workingdir}/${path} >/dev/null
        git add Dockerfile.product
        popd >/dev/null
        ;;
      i | I | ignore )
        echo "  Ignoring" ;;
      * )
        echo "${choice} not and option.  Assuming ignore" ; echo "  Ignoring" ;;
    esac
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
  newdiff=`diff -u .osbs-logs/Dockerfile.last Dockerfile`
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

show_yaml() {
  if ! [ "${brew_name}" == "none" ] ; then
    pushd "${workingdir}/${container}" >/dev/null
    package_name=`grep Name= Dockerfile | cut -d'"' -f2`
    package_version=`grep Version= Dockerfile | cut -d'"' -f2`
    package_release=`grep Release= Dockerfile | cut -d'"' -f2`
    echo "---"
    echo "repository: ${brew_name}"
    echo "tags: ${package_version},${package_version}-${package_release},latest"
    echo "build: ${container}-${package_version}-${package_release}"
    echo "repository_tag: ${brew_name}:${package_version}-${package_release}"
    if ! [ "${alt_name}" == "none" ] ; then
      echo "---"
      echo "repository: ${alt_name}"
      echo "tags: ${package_version},${package_version}-${package_release},latest"
      echo "build: ${container}-${package_version}-${package_release}"
      echo "repository_tag: ${alt_name}:${package_version}-${package_release}"
    fi
    popd >/dev/null
  fi
}

function push_image {
   docker push $1
   if [ $? -ne 0 ]; then
     echo "OH NO!!! There was a problem pushing the image, you may not be logged in or there was some other error.
To login, visit https://api.qe.openshift.com/oauth/token/request then
  docker login -e USERID@redhat.com -u USERID@redhat.com -p TOKEN https://registry.qe.redhat.com
"
     exit 1
   fi
}

start_push_image() {
  pushd "${workingdir}/${container}" >/dev/null
  package_name=`grep Name= Dockerfile | cut -d'"' -f2`
  if ! [ "${update_version}" == "TRUE" ] ; then
    version_version=`grep Version= Dockerfile | cut -d'"' -f2`
  fi
  if ! [ "${update_release}" == "TRUE" ] ; then
    release_version=`grep Release= Dockerfile | cut -d'"' -f2`
  fi
  echo "  ${container} ${package_name}:${version_version}"
  echo
  docker pull ${OSBS_REGISTRY}/${package_name}:${version_version}
  docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/${package_name}:${version_version}
  push_image ${PUSH_REGISTRY}/${package_name}:${version_version}
  if ! [ "${NOTLATEST}" == "TRUE" ] ; then
    docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/${package_name}:latest
    push_image ${PUSH_REGISTRY}/${package_name}:latest
  fi
  if ! [ "${alt_name}" == "none" ] ; then
    trimmed_alt_name=$(echo "${alt_name}" | cut -d'/' -f2)
    if [ "${VERBOSE}" == "TRUE" ] ; then
      echo "----------"
      echo "docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/openshift3/${trimmed_alt_name}:${version_version}"
      echo "push_image ${PUSH_REGISTRY}/openshift3/${trimmed_alt_name}:${version_version}"
      echo "----------"
    fi
    docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/openshift3/${trimmed_alt_name}:${version_version}
    push_image ${PUSH_REGISTRY}/openshift3/${trimmed_alt_name}:${version_version}
    if ! [ "${NOTLATEST}" == "TRUE" ] ; then
      docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/openshift3/${trimmed_alt_name}:latest
      push_image ${PUSH_REGISTRY}/openshift3/${trimmed_alt_name}:latest
    fi
  fi
  popd >/dev/null
}

check_dependents() {
  if ! [ "${dependent_list_new}" == "" ] ; then
    dependent_list_working="${dependent_list_new}"
    dependent_list_new=""
    for line in "${dependent_list_working}"
    do
      if [ "${VERBOSE}" == "TRUE" ] ; then
        echo "Checking dependents for: ${line}"
      fi

      for index in ${!packagekey[@]}; do
        if [[ ${dependent_list} =~ "::${index}::" ]] ; then
          if [ "${VERBOSE}" == "TRUE" ] ; then
            echo "  Already have on list: ${index}"
          fi
        else
          checkdep=$(echo "${packagekey[${index}]}" | awk '{print $2}')
          if [ "${VERBOSE}" == "TRUE" ] ; then
            echo "  Not on list - checking: ${index}"
            echo "    Dependency is: ${checkdep}"
          fi
          if [[ ${dependent_list} =~ "::${checkdep}::" ]] ; then
            export dependent_list+="::${index}:: "
            export dependent_list_new+="::${index}:: "
            add_to_list ${index}
            if [ "${VERBOSE}" == "TRUE" ] ; then
              echo "      Added to build list: ${index}"
            fi
          fi
        fi
      done
    done
    check_dependents
  fi
}

build_container() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  start_build_image
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

docker_backfill() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  setup_git_repo
  backfill_dockerfile
  popd >/dev/null
}

build_yaml() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  show_yaml
  popd >/dev/null
}

push_images() {
  pushd "${workingdir}" >/dev/null
  setup_dist_git
  start_push_image
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
    git_compare | docker_update | build_container | make_yaml | docker_backfill | push_images | test)
      export action="${key}"
      ;;
    bump_and_build)
      export bump_release="TRUE"
      export action="${key}"
      ;;
    list)
      export action="${key}"
      add_group_to_list base
      add_group_to_list sti
      add_group_to_list misc
      add_group_to_list logging
      add_group_to_list metrics
      ;;
    --group)
      group_input="$2"
      if [ "${group_input}" == "all" ] ; then
        add_group_to_list base
        add_group_to_list sti
        add_group_to_list misc
        add_group_to_list logging
        add_group_to_list metrics
      else
        add_group_to_list "${group_input}"
      fi
      shift
      ;;
    --package)
      export parent+=" ::$2::"
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
    --bump_release)
      export bump_release="TRUE"
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
    -d|--dep|--deps|--dependents)
      export DEPENDENTS="TRUE"
      ;;
    -n|--notlatest)
      export NOTLATEST="TRUE"
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

# Setup directory
if ! [ "${action}" == "test" ] && ! [ "${action}" == "list" ] ; then
  workingdir=$(mktemp -d /var/tmp/rebuild-images-XXXXXX)
  pushd "${workingdir}" &>/dev/null
  mkdir -p logs/done
  echo "::None::" >> logs/finished
  touch logs/buildfailed
  echo "Using working directory: ${workingdir}"
fi

# Setup dependents
if [ "${DEPENDENTS}" == "TRUE" ] ; then
  if [ "${VERBOSE}" == "TRUE" ] ; then
    echo "Dependents Parent: ${parent}"
  fi
  for item_name in ${packagelist}
  do
    container_name=$(echo "$item_name" | cut -d':' -f3)
    if [ "${VERBOSE}" == "TRUE" ] ; then
      echo "Item Name: ${item_name}"
      echo "Container: ${container_name}"
    fi
    if ! [ "${container_name}" == "" ] ; then
      export dependent_list+="::${container_name}:: "
      export dependent_list_new+="::${container_name}:: "
    fi
  done
  check_dependents
else
  export parent=""
fi

echo "${list}" | while read spec
do
  [ -z "$spec" ] && continue
  export branch="${DIST_GIT_BRANCH}"
  export container=$(echo "$spec" | awk '{print $1}')
  export dependency=$(echo "$spec" | awk '{print $2}')
  export repo=$(echo "$spec" | awk '{print $3}')
  export path=$(echo "$spec" | awk '{print $4}')
  export brew_name=$(echo "$spec" | awk '{print $5}')
  export alt_name=$(echo "$spec" | awk '{print $6}')
  case "$action" in
    build_container )
      echo "=== ${container} ==="
      build_container
      ;;
    git_compare )
      echo "=== ${container} ==="
      git_compare
      ;;
    docker_update )
      echo "=== ${container} ==="
      docker_update
      ;;
    docker_backfill )
      echo "=== ${container} ==="
      docker_backfill
      ;;
    bump_and_build )
      echo "=== ${container} ==="
      docker_update
      build_container
      ;;
    make_yaml )
      build_yaml
      ;;
    push_images )
      echo "=== ${container} ==="
      push_images
      ;;
    test | list )
      test_function
      ;;
    * )
      usage
      exit 2
      ;;
  esac
done

case "$action" in
  build_container | bump_and_build )
    wait_for_all_builds
    ;;
  docker_backfill )
    pushd ${workingdir}/ose >/dev/null
    if [ "${FORCE}" == "TRUE" ] ; then
      echo "  Force Option Selected - Assuming Continue"
      git commit -m " Backporting dist-git Dockerfile to Dockerfile.product"
      git push
    else
      git status
      echo "(c)ontinue [replace old Dockerfile.product], (i)gnore [leave old Dockerfile.product] : "
      read choice < /dev/tty
      case ${choice} in
        c | C | continue )
          git commit -m " Backporting dist-git Dockerfile to Dockerfile.product"
          git push
          ;;
        * )
          echo "  Ignoring" ;;
      esac
    fi
    popd >/dev/null
  ;;
esac
