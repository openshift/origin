#!/bin/bash
#
# This abstracts starting up an extended server.

# If invoked with arguments, executes the test directly.
function os::test::extended::focus {
  if [[ $# -ne 0 ]]; then
    echo "[INFO] Running custom: $*"
    tests=$(TEST_REPORT_DIR= TEST_OUTPUT_QUIET=true ${EXTENDEDTEST} --ginkgo.dryRun --ginkgo.noColor "$@" | col -b | grep -v "35mskip0m" | grep "1mok0m" | wc -l)
    if [[ "${tests}" -eq 0 ]]; then
      echo "[ERROR] No tests would be run"
      exit 1
    fi
    ${EXTENDEDTEST} "$@"
    exit $?
  fi
}

# Launches an extended server for OpenShift
# TODO: this should be doing less, because clusters should be stood up outside
#   and then tests are executed.  Tests that depend on fine grained setup should
#   be done in other contexts.
function os::test::extended::setup {
  # build binaries
  if [[ -z $(os::build::find-binary ginkgo) ]]; then
    hack/build-go.sh vendor/github.com/onsi/ginkgo/ginkgo
  fi
  if [[ -z $(os::build::find-binary extended.test) ]]; then
    hack/build-go.sh test/extended/extended.test
  fi
  if [[ -z $(os::build::find-binary openshift) ]]; then
    hack/build-go.sh
  fi

  os::util::environment::setup_time_vars

  # ensure proper relative directories are set
  export GINKGO="$(os::build::find-binary ginkgo)"
  export EXTENDEDTEST="$(os::build::find-binary extended.test)"
  export TMPDIR=${BASETMPDIR:-/tmp}
  export EXTENDED_TEST_PATH="$(pwd)/test/extended"
  export KUBE_REPO_ROOT="$(pwd)/vendor/k8s.io/kubernetes"

  # output tests instead of running
  if [[ -n "${SHOW_ALL:-}" ]]; then
    TEST_OUTPUT_QUIET=true ${EXTENDEDTEST} --ginkgo.dryRun --ginkgo.noColor | grep ok | grep -v skip | cut -c 20- | sort
    exit 0
  fi

  # allow setup to be skipped
  if [[ -z "${TEST_ONLY+x}" ]]; then
    ensure_iptables_or_die

    function cleanup()
    {
      out=$?
      cleanup_openshift
      echo "[INFO] Exiting"
      return $out
    }

    trap "exit" INT TERM
    trap "cleanup" EXIT
    echo "[INFO] Starting server"

    os::util::environment::setup_all_server_vars "test-extended/core"
    os::util::environment::use_sudo
    os::util::environment::setup_images_vars
    reset_tmp_dir

    # If the current system has the XFS volume dir mount point we configure
    # in the test images, assume to use it which will allow the local storage
    # quota tests to pass.
    if [ -d "/mnt/openshift-xfs-vol-dir" ]; then
      export VOLUME_DIR="/mnt/openshift-xfs-vol-dir"
    else
      echo "[WARN] /mnt/openshift-xfs-vol-dir does not exist, local storage quota tests may fail."
    fi

    os::log::start_system_logger

    # when selinux is enforcing, the volume dir selinux label needs to be
    # svirt_sandbox_file_t
    #
    # TODO: fix the selinux policy to either allow openshift_var_lib_dir_t
    # or to default the volume dir to svirt_sandbox_file_t.
    if selinuxenabled; then
          sudo chcon -t svirt_sandbox_file_t ${VOLUME_DIR}
    fi
    configure_os_server
    #turn on audit logging for extended tests ... mimic what is done in util.sh configure_os_server, but don't
    # put change there - only want this for extended tests
    echo "[INFO] Turn on audit logging"
    cp ${SERVER_CONFIG_DIR}/master/master-config.yaml ${SERVER_CONFIG_DIR}/master/master-config.orig2.yaml
    openshift ex config patch ${SERVER_CONFIG_DIR}/master/master-config.orig2.yaml --patch="{\"auditConfig\": {\"enabled\": true}}"  > ${SERVER_CONFIG_DIR}/master/master-config.yaml

    # Similar to above check, if the XFS volume dir mount point exists enable
    # local storage quota in node-config.yaml so these tests can pass:
    if [ -d "/mnt/openshift-xfs-vol-dir" ]; then
	# The ec2 images usually have ~5Gi of space defined for the xfs vol for the registry; want to give /registry a good chunk of that
	# to store the images created when the extended tests run
      sed -i 's/perFSGroup: null/perFSGroup: 4480Mi/' $NODE_CONFIG_DIR/node-config.yaml
    fi
    echo "[INFO] Using VOLUME_DIR=${VOLUME_DIR}"

    # This is a bit hacky, but set the pod gc threshold appropriately for the garbage_collector test.
    os::util::sed 's/\(controllerArguments:\ \)null/\1\n    terminated-pod-gc-threshold: ["100"]/' \
      ${MASTER_CONFIG_DIR}/master-config.yaml

    start_os_server

    export KUBECONFIG="${ADMIN_KUBECONFIG}"

    install_registry
    wait_for_registry
    DROP_SYN_DURING_RESTART=1 CREATE_ROUTER_CERT=1 install_router

    echo "[INFO] Creating image streams"
    oc create -n openshift -f examples/image-streams/image-streams-centos7.json --config="${ADMIN_KUBECONFIG}"
  else
    # be sure to set VOLUME_DIR if you are running with TEST_ONLY
    echo "[INFO] Not starting server, VOLUME_DIR=${VOLUME_DIR:-}"
  fi
}

# Not run by any suite
readonly EXCLUDED_TESTS=(
  "\[Skipped\]"
  "\[Disruptive\]"
  "\[Slow\]"
  "\[Flaky\]"

  "\[Feature:Performance\]"

  # Depends on external components, may not need yet
  Monitoring              # Not installed, should be
  "Cluster level logging" # Not installed yet
  Kibana                  # Not installed
  Ubernetes               # Can't set zone labels today
  kube-ui                 # Not installed by default
  "^Kubernetes Dashboard"  # Not installed by default (also probbaly slow image pull)

  "\[Feature:Federation\]"   # Not enabled yet
  "\[Feature:PodAffinity\]"  # Not enabled yet
  Ingress                    # Not enabled yet
  "Cinder"                   # requires an OpenStack cluster
  "should support r/w"       # hostPath: This test expects that host's tmp dir is WRITABLE by a container.  That isn't something we need to gaurantee for openshift.
  "should check that the kubernetes-dashboard instance is alive" # we don't create this
  "\[Feature:ManualPerformance\]" # requires /resetMetrics which we don't expose

  # See the CanSupport implementation in upstream to determine wether these work.
  "Ceph RBD"      # Works if ceph-common Binary installed (but we can't gaurantee this on all clusters).
  "GlusterFS" # May work if /sbin/mount.glusterfs to be installed for plugin to work (also possibly blocked by serial pulling)
  "should support r/w" # hostPath: This test expects that host's tmp dir is WRITABLE by a container.  That isn't something we need to guarantee for openshift.

  "should allow starting 95 pods per node" # needs cherry-pick of https://github.com/kubernetes/kubernetes/pull/23945

  # Need fixing
  "Horizontal pod autoscaling" # needs heapster
  "should provide Internet connection for containers" # Needs recursive DNS
  PersistentVolume           # https://github.com/openshift/origin/pull/6884 for recycler
  "mount an API token into pods" # We add 6 secrets, not 1
  "ServiceAccounts should ensure a single API token exists" # We create lots of secrets
  "Networking should function for intra-pod" # Needs two nodes, add equiv test for 1 node, then use networking suite
  "should test kube-proxy"     # needs 2 nodes
  "authentication: OpenLDAP"   # needs separate setup and bucketing for openldap bootstrapping
  "should support exec through an HTTP proxy" # doesn't work because it requires a) static binary b) linux c) kubectl, https://github.com/openshift/origin/issues/7097
  "NFS"                      # no permissions https://github.com/openshift/origin/pull/6884
  "\[Feature:Example\]"      # may need to pre-pull images
  "ResourceQuota and capture the life of a secret" # https://github.com/openshift/origin/issue/9414
  "NodeProblemDetector"        # requires a non-master node to run on

  # Needs triage to determine why it is failing
  "Addon update"          # TRIAGE
  SSH                     # TRIAGE
  "\[Feature:Upgrade\]"   # TRIAGE
  "SELinux relabeling"    # started failing
  "schedule jobs on pod slaves use of jenkins with kubernetes plugin by creating slave from existing builder and adding it to Jenkins master" # https://github.com/openshift/origin/issues/7619
  "openshift mongodb replication creating from a template" # flaking on deployment
  "Update Demo should do a rolling update of a replication controller" # this is flaky and needs triaging

  # Test will never work
  "should proxy to cadvisor" # we don't expose cAdvisor port directly for security reasons

  # Inordinately slow tests
  "should create and stop a working application"
  "should always delete fast" # will be uncommented in etcd3
)

readonly SERIAL_TESTS=(
  "\[Serial\]"
  "\[Feature:ManualPerformance\]" # requires isolation
  "Service endpoints latency" # requires low latency
  "\[Feature:HighDensityPerformance\]" # requires no other namespaces
)

readonly CONFORMANCE_TESTS=(
  "\[Conformance\]"

  "Services.*NodePort"
  "ResourceQuota should"
  "\[networking\] basic openshift networking"
  "\[networking\]\[router\]"
  "Ensure supplemental groups propagate to docker"
  "EmptyDir"
  "PetSet"
  "PrivilegedPod should test privileged pod"
  "Pods should support remote command execution"
  "Pods should support retrieving logs from the container"
  "Kubectl client Simple pod should support"
  "Job should run a job to completion when tasks succeed"
  "\[images\]\[mongodb\] openshift mongodb replication"
  "\[job\] openshift can execute jobs controller"
  "\[volumes\] Test local storage quota FSGroup"
  "test deployment should run a deployment to completion"
  "Variable Expansion"
  "init containers"
  "Clean up pods on node kubelet"
  "\[Feature\:SecurityContext\]"
  "should create a LimitRange with defaults"
  "Generated release_1_2 clientset"
)
