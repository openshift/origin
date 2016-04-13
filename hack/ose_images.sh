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
## COMMON VARIABLES ##
source ose.conf

## LOCAL VARIABLES ##
DIST_GIT_BRANCH="rhaos-3.2-rhel-7"
#DIST_GIT_BRANCH="rhaos-3.1-rhel-7"
#DIST_GIT_BRANCH="rhaos-3.2-rhel-7-candidate"
SCRATCH_OPTION=""
BUILD_REPO="http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-building.repo"
COMMIT_MESSAGE=""
#DIST_GIT_BRANCH="rhaos-3.1-rhel-7-candidate"
OSBS_REGISTRY=brew-pulp-docker01.web.prod.ext.phx2.redhat.com:8888
#OSBS_REGISTRY=rcm-img-docker01.build.eng.bos.redhat.com:5001
PUSH_REGISTRY=registry.qe.openshift.com

usage() {
  echo "Usage `basename $0` [action] <options>" >&2
  echo >&2
  echo "Actions:" >&2
  echo "  build_container :: Clone dist-git, build containers" >&2
  echo "  docker_update   :: Clone dist-git, update version, release, or rhel" >&2
  echo "  docker_backfill :: Copy dist-git Dockerfile to git Dockerfile.product" >&2
  echo "  git_compare     :: Clone dist-git and git, compare files and Dockerfile" >&2
  echo "  update_compare  :: Clone dist-git and git, update dockerfile, compare all" >&2
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
  echo "  --message [message] :: Git commit message" >&2
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
    export packagelist+=" ::${1}::"
    if [ "${VERBOSE}" == "TRUE" ] ; then
      echo "----------"
      echo ${packagelist}
    fi
  fi
}

add_group_to_list() {
  case ${1} in
    base)
      add_to_list openshift-enterprise-base-docker
      add_to_list openshift-enterprise-openvswitch-docker
      add_to_list openshift-enterprise-pod-docker
      add_to_list aos3-installation-docker
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
  git clone -q ${git_repo} 2>/dev/null
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
  depcheck=`grep ::${build_dependency}:: ${workingdir}/logs/finished`
  while [ "${depcheck}" == "" ]
  do
    now=`date`
    echo "Waiting for ${build_dependency} to be built - ${now}"
    sleep 120
    check_builds
    depcheck=`grep ::${build_dependency}:: ${workingdir}/logs/finished`
  done
}

build_image() {
    rhpkg container-build ${SCRATCH_OPTION} --repo ${BUILD_REPO} >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-building.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-latest.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-errata-building.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-unsigned-errata-latest.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-signed-building.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    #rhpkg container-build --repo http://file.rdu.redhat.com/tdawson/repo/aos-signed-latest.repo >> ${workingdir}/logs/${container}.buildlog 2>&1 &
    echo -n "  Waiting for createContainer taskid ."
    sleep 10
    taskid=`grep createContainer ${workingdir}/logs/${container}.buildlog | awk '{print $1}' | sort -u`
    while [ "${taskid}" == "" ]
    do
      echo -n "."
      sleep 10
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
    failedcheck=`grep ::${build_dependency}:: ${workingdir}/logs/buildfailed`
    if [ "${failedcheck}" == "" ] ; then
      build_image
    else
      echo "  dependency error: ${build_dependency} failed to build"
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
  if ! [ "${git_style}" == "dockerfile_only" ] ; then
    echo "  ---- Checking files changed, added or removed ----"
    extra_check=$(diff --brief -r ${workingdir}/${container} ${workingdir}/${git_path} | grep -v -e Dockerfile -e git -e osbs )
    if ! [ "${extra_check}" == "" ] ; then
      echo "${extra_check}"
    fi
    differ_check=$(echo "${extra_check}" | grep " differ")
    new_file=$(echo "${extra_check}" | grep "Only in ${workingdir}/${git_path}")
    old_file=$(echo "${extra_check}" | grep "Only in ${workingdir}/${container}")
    if ! [ "${differ_check}" == "" ] ; then
      echo "  ---- Non-Docker file changes ----"
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
      echo "  ---- Removed Non-Docker files ----"
      echo "${old_file}" | while read old_file_line
      do
        myold_file=$(echo "${old_file_line}" | awk '{print $4}')
        # echo "Removing: ${myold_file}"
        git rm ${myold_file}
      done
    fi
    if ! [ "${new_file}" == "" ] ; then
      echo "  ---- New Non-Docker files ----"
      #echo " New files must be added by hand - sorry about that"
      echo "${new_file}"
      working_path="${workingdir}/${git_path}"
      echo "${new_file}" | while read new_file_line
      do
        mynew_file=$(echo "${new_file_line}" | awk '{print $4}')
        mynew_file_trim="${mynew_file#$working_path}"
        cp -rv ${workingdir}/${git_path}/${mynew_file} ${workingdir}/${container}/${mynew_file}
        git add ${workingdir}/${container}/${mynew_file}
      done
    fi
  fi
  echo "  ---- Checking Dockerfile changes ----"
  diff --label Dockerfile --label ${git_path}/Dockerfile -u Dockerfile ${workingdir}/${git_path}/${git_dockerfile} >> .osbs-logs/Dockerfile.diff.new
  if ! [ -f .osbs-logs/Dockerfile.diff ] ; then
    touch .osbs-logs/Dockerfile.diff
  fi
  newdiff=`diff .osbs-logs/Dockerfile.diff .osbs-logs/Dockerfile.diff.new`
  if [ "${newdiff}" == "" ] ; then
    rm -f .osbs-logs/Dockerfile.diff.new
  fi
  if ! [ "${newdiff}" == "" ] || ! [ "${extra_check}" == "" ] ; then
    echo "${newdiff}"
    echo " "
    echo "Changes occured "
    if [ "${FORCE}" == "TRUE" ] ; then
      echo "  Force Option Selected - Assuming Continue"
      mv -f .osbs-logs/Dockerfile.diff.new .osbs-logs/Dockerfile.diff ; git add .osbs-logs/Dockerfile.diff ; rhpkg commit -p -m "${COMMIT_MESSAGE} ${version_version} ${release_version} ${rhel_version}" > /dev/null
    else
      echo "  To view/modify changes, go to: ${workingdir}/${container}"
      echo "(c)ontinue [rhpkg commit], (i)gnore, (q)uit [exit script] : "
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
  diff --label Dockerfile --label ${git_path}/Dockerfile.product -u Dockerfile ${workingdir}/${git_path}/Dockerfile.product
  if [ "${FORCE}" == "TRUE" ] ; then
    echo "  Force Option Selected - Assuming Continue"
    cp -f Dockerfile ${workingdir}/${git_path}/Dockerfile.product
    pushd ${workingdir}/${git_path} >/dev/null
    git add Dockerfile.product
    popd >/dev/null
  else
    echo "(c)ontinue [replace old Dockerfile.product], (i)gnore [leave old Dockerfile.product] : "
    read choice < /dev/tty
    case ${choice} in
      c | C | continue )
        cp -f Dockerfile ${workingdir}/${git_path}/Dockerfile.product
        pushd ${workingdir}/${git_path} >/dev/null
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
  pushd "${workingdir}/${container}" >/dev/null
  package_version=`grep Version= Dockerfile | cut -d'"' -f2`
  package_release=`grep Release= Dockerfile | cut -d'"' -f2`
  for image_name in ${docker_name_list}
  do
    echo "---"
    echo "repository: ${image_name}"
    echo "tags: ${package_version},${package_version}-${package_release},latest"
    echo "build: ${container}-${package_version}-${package_release}"
    echo "repository_tag: ${image_name}:${package_version}-${package_release}"
  done
  popd >/dev/null
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
  echo "====================================================" >>  ${workingdir}/logs/push.image.log
  echo "  ${container} ${package_name}:${version_version}" | tee -a ${workingdir}/logs/push.image.log
  echo "    START: $(date +"%Y-%m-%d %H:%M:%S")" | tee -a ${workingdir}/logs/push.image.log
  echo | tee -a ${workingdir}/logs/push.image.log
  docker pull ${OSBS_REGISTRY}/${package_name}:${version_version} | tee -a ${workingdir}/logs/push.image.log
  echo | tee -a ${workingdir}/logs/push.image.log
  docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/${package_name}:${version_version} | tee -a ${workingdir}/logs/push.image.log
  echo | tee -a ${workingdir}/logs/push.image.log
  push_image ${PUSH_REGISTRY}/${package_name}:${version_version} | tee -a ${workingdir}/logs/push.image.log
  echo | tee -a ${workingdir}/logs/push.image.log
  if ! [ "${NOTLATEST}" == "TRUE" ] ; then
    docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/${package_name}:latest | tee -a ${workingdir}/logs/push.image.log
    echo | tee -a ${workingdir}/logs/push.image.log
    push_image ${PUSH_REGISTRY}/${package_name}:latest | tee -a ${workingdir}/logs/push.image.log
    echo | tee -a ${workingdir}/logs/push.image.log
  fi
  if ! [ "${alt_name}" == "" ] ; then
    trimmed_alt_name=$(echo "${alt_name}" | cut -d'/' -f2)
    if [ "${VERBOSE}" == "TRUE" ] ; then
      echo "----------"
      echo "docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/aep3/${trimmed_alt_name}:${version_version}"
      echo "push_image ${PUSH_REGISTRY}/aep3/${trimmed_alt_name}:${version_version}"
      echo "----------"
    fi
    docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/aep3/${trimmed_alt_name}:${version_version} | tee -a ${workingdir}/logs/push.image.log
    echo | tee -a ${workingdir}/logs/push.image.log
    push_image ${PUSH_REGISTRY}/aep3/${trimmed_alt_name}:${version_version} | tee -a ${workingdir}/logs/push.image.log
    echo | tee -a ${workingdir}/logs/push.image.log
    if ! [ "${NOTLATEST}" == "TRUE" ] ; then
      docker tag -f ${OSBS_REGISTRY}/${package_name}:${version_version} ${PUSH_REGISTRY}/aep3/${trimmed_alt_name}:latest | tee -a ${workingdir}/logs/push.image.log
      echo | tee -a ${workingdir}/logs/push.image.log
      push_image ${PUSH_REGISTRY}/aep3/${trimmed_alt_name}:latest | tee -a ${workingdir}/logs/push.image.log
      echo | tee -a ${workingdir}/logs/push.image.log
    fi
  fi
  echo "    END: $(date +"%Y-%m-%d %H:%M:%S")" | tee -a ${workingdir}/logs/push.image.log
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

      for index in ${!dict_image_from[@]}; do
        if [[ ${dependent_list} =~ "::${index}::" ]] ; then
          if [ "${VERBOSE}" == "TRUE" ] ; then
            echo "  Already have on list: ${index}"
          fi
        else
          checkdep=$(echo "${dict_image_from[${index}]}" | awk '{print $2}')
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
  echo -e "container: ${container}\tdocker names: ${dict_image_name[${container}]}"
  if [ "${VERBOSE}" == "TRUE" ] ; then
    echo -e "dependency: ${build_dependency}\tbranch: ${branch}"
  fi
}

if [ "$#" -lt 1 ] ; then
  usage
fi

# Get our arguments
while [[ "$#" -ge 1 ]]
do
key="$1"
case $key in
    git_compare | docker_update | build_container | make_yaml | docker_backfill | push_images | update_compare | test)
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
      export really_bump_release="TRUE"
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
      export REALLYFORCE="TRUE"
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
    container_name=$(echo "${item_name}" | cut -d':' -f3)
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

for unique_package in ${packagelist}
do
  [ -z "${unique_package}" ] && continue
  export branch="${DIST_GIT_BRANCH}"
  export container=$(echo "${unique_package}" | cut -d':' -f3)
  export build_dependency=$(echo "${dict_image_from[${container}]}" | awk '{print $2}')
  case "$action" in
    build_container )
      echo "=== ${container} ==="
      build_container
      ;;
    git_compare )
      export git_repo=$(echo "${dict_git_compare[${container}]}" | awk '{print $1}')
      export git_path=$(echo "${dict_git_compare[${container}]}" | awk '{print $2}')
      export git_dockerfile=$(echo "${dict_git_compare[${container}]}" | awk '{print $3}')
      export git_style=$(echo "${dict_git_compare[${container}]}" | awk '{print $4}')
      if [ "${COMMIT_MESSAGE}" == "" ] ; then
        COMMIT_MESSAGE="Sync origin git to ose dist-git "
      fi
      echo "=== ${container} ==="
      if ! [ "${git_repo}" == "None" ] ; then
        git_compare
      else
        echo " No git repo to compare to."
        echo " Skipping"
      fi
      ;;
    docker_update )
      if [ "${COMMIT_MESSAGE}" == "" ] ; then
        COMMIT_MESSAGE="Updating Dockerfile version and release"
      fi
      echo "=== ${container} ==="
      docker_update
      ;;
    docker_backfill )
      if [ "${COMMIT_MESSAGE}" == "" ] ; then
        COMMIT_MESSAGE="Backporting dis-git Dockerfile changes to ose Dockerfile.product"
      fi
      echo "=== ${container} ==="
      docker_backfill
      ;;
    update_compare )
      if [ "${REALLYFORCE}" == "TRUE" ] ; then
        export FORCE="TRUE"
      fi
      if [ "${really_bump_release}" == "TRUE" ] ; then
        export bump_release="TRUE"
      fi
      export git_repo=$(echo "${dict_git_compare[${container}]}" | awk '{print $1}')
      export git_path=$(echo "${dict_git_compare[${container}]}" | awk '{print $2}')
      export git_dockerfile=$(echo "${dict_git_compare[${container}]}" | awk '{print $3}')
      export git_style=$(echo "${dict_git_compare[${container}]}" | awk '{print $4}')
      echo "=== ${container} ==="
      if [ "${COMMIT_MESSAGE}" == "" ] ; then
        COMMIT_MESSAGE="Updating Dockerfile version and release"
      fi
      export FORCE="TRUE"
      docker_update
      if ! [ "${git_repo}" == "None" ] ; then
        if [ "${COMMIT_MESSAGE}" == "" ] ; then
          COMMIT_MESSAGE="Sync origin git to ose dist-git "
        fi
        if [ "${REALLYFORCE}" == "TRUE" ] ; then
          export FORCE="TRUE"
        else
          export FORCE="FALSE"
        fi
        git_compare
        if [ "${COMMIT_MESSAGE}" == "" ] ; then
          COMMIT_MESSAGE="Reupdate Dockerfile after compare"
        fi
        export FORCE="TRUE"
        export bump_release="FALSE"
        docker_update
      else
        echo " No git repo to compare to."
        echo " Skipping"
      fi
      ;;
    make_yaml )
      docker_name_list="${dict_image_name[${container}]}"
      if ! [ "${docker_name_list}" == "" ] ; then
        build_yaml
      fi
      ;;
    push_images )
      echo "=== ${container} ==="
      export brew_name=$(echo "${dict_image_name[${container}]}" | awk '{print $1}')
      export alt_name=$(echo "${dict_image_name[${container}]}" | awk '{print $2}')
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
