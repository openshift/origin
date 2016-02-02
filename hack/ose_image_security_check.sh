#!/bin/bash
#
# This script checks if our images need a security update
#
# User must be able to run docker images
#
# Required packages:
#   docker

rhel_version="rhel7.2"
jboss7_version="jboss-base-7/jdk8:1.1"
jboss6_version="jboss-eap-6/eap-openshift"

declare -A packagekey
# format:
# key:dist-git_name	array:"docker_name docker_base_base"
packagekey['aos-f5-router-docker']="openshift3/ose-f5-router rhel"
packagekey['image-inspector-docker']="openshift3/image-inspector rhel"
packagekey['logging-auth-proxy-docker']="openshift3/logging-auth-proxy rhel"
packagekey['logging-deployment-docker']="openshift3/logging-deployment rhel"
packagekey['logging-elasticsearch-docker']="openshift3/logging-elasticsearch rhel"
packagekey['logging-fluentd-docker']="openshift3/logging-fluentd rhel"
packagekey['logging-kibana-docker']="openshift3/logging-kibana rhel"
packagekey['metrics-cassandra-docker']="openshift3/metrics-cassandra jboss7"
packagekey['metrics-deployer-docker']="openshift3/metrics-deployer rhel"
packagekey['metrics-hawkular-metrics-docker']="openshift3/metrics-hawkular-metrics jboss6"
packagekey['metrics-heapster-docker']="openshift3/metrics-heapster rhel"
packagekey['openshift-enterprise-base-docker']="openshift3/ose-base rhel"
packagekey['openshift-enterprise-deployer-docker']="openshift3/ose-deployer rhel"
packagekey['openshift-enterprise-docker']="openshift3/ose rhel"
packagekey['openshift-enterprise-docker-builder-docker']="openshift3/ose-docker-builder rhel"
packagekey['openshift-enterprise-dockerregistry-docker']="openshift3/ose-docker-registry rhel"
packagekey['openshift-enterprise-haproxy-router-base-docker']="openshift3/ose-haproxy-router-base rhel"
packagekey['openshift-enterprise-haproxy-router-docker']="openshift3/ose-haproxy-router rhel"
packagekey['openshift-enterprise-keepalived-ipfailover-docker']="openshift3/ose-keepalived-ipfailover rhel"
packagekey['openshift-enterprise-node-docker']="openshift3/node rhel"
packagekey['openshift-enterprise-openvswitch-docker']="openshift3/openvswitch rhel"
packagekey['openshift-enterprise-pod-docker']="openshift3/ose-pod rhel"
packagekey['openshift-enterprise-recycler-docker']="openshift3/ose-recycler rhel"
packagekey['openshift-enterprise-sti-builder-docker']="openshift3/ose-sti-builder rhel"
packagekey['openshift-jenkins-docker']="openshift3/jenkins-1-rhel7 rhel"
packagekey['openshift-mongodb-docker']="openshift3/mongodb-24-rhel7 rhel"
packagekey['openshift-mysql-docker']="openshift3/mysql-55-rhel7 rhel"
packagekey['openshift-postgresql-docker']="openshift3/postgresql-92-rhel7 rhel"
packagekey['openshift-sti-base-docker']="openshift3/sti-base rhel"
packagekey['openshift-sti-nodejs-docker']="openshift3/nodejs-010-rhel7 rhel"
packagekey['openshift-sti-perl-docker']="openshift3/perl-516-rhel7 rhel"
packagekey['openshift-sti-php-docker']="openshift3/php-55-rhel7 rhel"
packagekey['openshift-sti-python-docker']="openshift3/python-33-rhel7 rhel"
packagekey['openshift-sti-ruby-docker']="openshift3/ruby-20-rhel7 rhel"

usage() {
  echo "Usage ${0} [action] <options>" >&2
  echo >&2
  echo "Actions:" >&2
  echo "  check_security  :: Check for security updates in a docker image" >&2
  echo "  list            :: Display full list of packages / images" >&2
  echo "  test            :: Display what packages / images would be worked on" >&2
  echo >&2
  echo "Options:" >&2
  echo "  -h, --help          :: Show this options menu" >&2
  echo "  -v, --verbose       :: Be verbose" >&2
  echo "  --group [group]     :: Which group list to use (base sti metrics logging misc all)" >&2
  echo "  --package [package] :: Which package to use e.g. openshift-enterprise-pod-docker" >&2
  echo "  --version [version] :: Change image version e.g. 3.1.1 (Not Implemented Yet)" >&2
  echo "  --release [version] :: Change image release e.g. 3 (Not Implemented Yet)" >&2
  echo "  --rhel [version]    :: Change RHEL version e.g. rhel7.2:7.2-35 or rhel7.2 (Not Implemented Yet)" >&2
  echo >&2
  echo "Required: You must be able to run docker.  root is usually used" >&2
  echo "Note: --group and --package can be used multiple times" >&2
  echo >&2
  popd &>/dev/null
  exit 1
}

add_to_list() {
  NEWLINE=$'\n'
  export list+="${1} ${packagekey[${1}]}${NEWLINE}"
  if [ "${VERBOSE}" == "TRUE" ] ; then
    echo "----------"
    echo ${1} ${list}
  fi
}

add_group_to_list() {
  case ${1} in
    base)
      add_to_list openshift-enterprise-openvswitch-docker
      add_to_list openshift-enterprise-pod-docker
      add_to_list openshift-enterprise-docker
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


check_base() {
  pushd "${workingdir}" >/dev/null
  case ${base_type} in
    rhel )
      docker run ${rhel_version} yum --disablerepo=* --enablerepo=rhel-7-server-rpms update --security -d 1 -e 0 --assumeno | grep rhel-7-server | grep -v updateinfo | awk '{print $1 " " $3}' | sort -u > ${base_type}.security.rpms
      docker run ${rhel_version} yum --disablerepo=* --enablerepo=rhel-7-server-rpms updateinfo security | grep RHSA | sort -u > ${base_type}.security.info
      ;;
    jboss7 )
      # We have no access to the image, just touch needed files
      #docker run ${jboss7_version} yum --disablerepo=* --enablerepo=rhel-7-server-rpms update --security -d 1 -e 0 --assumeno | grep rhel-7-server | grep -v updateinfo | awk '{print $1 " " $3}' | sort -u > ${base_type}.security.rpms
      #docker run ${jboss7_version} yum --disablerepo=* --enablerepo=rhel-7-server-rpms updateinfo security | grep RHSA | sort -u > ${base_type}.security.info
      touch ${base_type}.security.rpms
      touch ${base_type}.security.info
      ;;
    jboss6 )
      # The image has no yum repos to check against, just touch needed files
      #docker run ${jboss6_version} yum --disablerepo=* --enablerepo=rhel-7-server-rpms update --security -d 1 -e 0 --assumeno | grep rhel-7-server | grep -v updateinfo | awk '{print $1 " " $3}' | sort -u > ${base_type}.security.rpms
      #docker run ${jboss6_version} yum --disablerepo=* --enablerepo=rhel-7-server-rpms updateinfo security | grep RHSA | sort -u > ${base_type}.security.info
      touch ${base_type}.security.rpms
      touch ${base_type}.security.info
      ;;
  esac

  popd >/dev/null
}


check_container() {
  pushd "${workingdir}" >/dev/null
  if ! [ -f ${base_type}.security.rpms ] ; then
    echo "--Checking ${base_type} image first ..."
    check_base
  fi
  echo "--Checking ${container} ..."
  docker run -u root --entrypoint="/bin/yum"  ${container} --disablerepo=* --enablerepo=rhel-7-server-rpms update --security -d 1 -e 0 --assumeno | grep rhel-7-server | grep -v updateinfo | awk '{print $1 " " $3}' | sort -u > ${brew_name}.security.rpms
  comm -13 ${base_type}.security.rpms ${brew_name}.security.rpms > ${brew_name}.security.rpms.not.in.base
  if [ -s ${brew_name}.security.rpms.not.in.base ] ; then
    docker run -u root --entrypoint="/bin/yum"  ${container} --disablerepo=* --enablerepo=rhel-7-server-rpms updateinfo security | grep RHSA | sort -u > ${brew_name}.security.info
    comm -13 ${base_type}.security.info ${brew_name}.security.info > ${brew_name}.security.info.not.in.base
    echo " +++++++++++++++++++++++"
    echo " ++++ Update Needed ++++"
    cat ${brew_name}.security.rpms.not.in.base
    cat ${brew_name}.security.info.not.in.base
  else
    echo " ++++++++++++++++++++++++++++++++++++"
    echo " ++++ No Security Updates Needed ++++"
  fi
  popd >/dev/null
}

test_function() {
  echo -e "brew_name: ${brew_name} \tcontainer: ${container} \tbase: ${base_type}"
}

if [ "$#" -lt 1 ] ; then
  usage
fi

# Get our arguments
while [[ "$#" -ge 1 ]]
do
key="$1"
case $key in
    check_security | test)
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
      add_group_to_list "$2"
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
    -v|--verbose)
      export VERBOSE="TRUE"
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
if ! [ "$action" == "test" ] && ! [ "$action" == "list" ] ; then
workingdir=$(mktemp -d /var/tmp/check-image-security-XXXXXX)
echo "Using working directory: ${workingdir}"
fi

echo "${list}" | while read spec
do
  [ -z "$spec" ] && continue
  export brew_name=$(echo "$spec" | awk '{print $1}')
  export container=$(echo "$spec" | awk '{print $2}')
  export base_type=$(echo "$spec" | awk '{print $3}')
  case "$action" in
    check_security )
      echo "===================================="
      echo "=== ${container} ==="
      check_container
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
