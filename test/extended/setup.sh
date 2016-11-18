#!/bin/bash
#
# This abstracts starting up an extended server.

# If invoked with arguments, executes the test directly.
function os::test::extended::focus {
	if [[ $# -ne 0 ]]; then
		os::log::info "Running custom: $*"
		os::test::extended::test_list "$@"
		if [[ "${TEST_COUNT}" -eq 0 ]]; then
			os::log::error "No tests would be run"
			exit 1
		fi
		${EXTENDEDTEST} "$@"
		exit $?
	fi
}

# Launches an extended server for OpenShift
# TODO: this should be doing less, because clusters should be stood up outside
#		and then tests are executed.	Tests that depend on fine grained setup should
#		be done in other contexts.
function os::test::extended::setup () {
	# build binaries
	if [[ -z "$(os::build::find-binary ginkgo)" ]]; then
		hack/build-go.sh vendor/github.com/onsi/ginkgo/ginkgo
	fi
	if [[ -z "$(os::build::find-binary extended.test)" ]]; then
		hack/build-go.sh test/extended/extended.test
	fi
	if [[ -z "$(os::build::find-binary openshift)" ]]; then
		hack/build-go.sh
	fi

	os::util::environment::setup_time_vars
	os::util::environment::setup_all_server_vars "test-extended/core"

	# ensure proper relative directories are set
	GINKGO="$(os::build::find-binary ginkgo)"
	EXTENDEDTEST="$(os::build::find-binary extended.test)"
	export GINKGO
	export EXTENDEDTEST
	export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
	export KUBE_REPO_ROOT="${OS_ROOT}/vendor/k8s.io/kubernetes"

	# allow setup to be skipped
	if [[ -z "${TEST_ONLY+x}" ]]; then
		ensure_iptables_or_die

		function cleanup() {
			out=$?
			cleanup_openshift
			os::log::info "Exiting"
			return $out
		}

		trap "exit" INT TERM
		trap "cleanup" EXIT
		os::log::info "Starting server"

		os::util::environment::use_sudo
		os::util::environment::setup_images_vars

		local sudo=${USE_SUDO:+sudo}

		# If the current system has the XFS volume dir mount point we configure
		# in the test images, assume to use it which will allow the local storage
		# quota tests to pass.
		LOCAL_STORAGE_QUOTA=""
		if [[ -d "/mnt/openshift-xfs-vol-dir" ]] && ${sudo} lvs | grep -q "xfs"; then
			LOCAL_STORAGE_QUOTA="1"
			export VOLUME_DIR="/mnt/openshift-xfs-vol-dir"
		else
			os::log::warn "/mnt/openshift-xfs-vol-dir does not exist, local storage quota tests may fail."
		fi

		os::log::system::start

		if [[ -n "${SHOW_ALL:-}" ]]; then
			SKIP_NODE=1
		fi

		# when selinux is enforcing, the volume dir selinux label needs to be
		# svirt_sandbox_file_t
		#
		# TODO: fix the selinux policy to either allow openshift_var_lib_dir_t
		# or to default the volume dir to svirt_sandbox_file_t.
		if selinuxenabled; then
			${sudo} chcon -t svirt_sandbox_file_t ${VOLUME_DIR}
		fi
		CONFIG_VERSION=""
		if [[ -n "${API_SERVER_VERSION:-}" ]]; then
			CONFIG_VERSION="${API_SERVER_VERSION}"
		elif [[ -n "${CONTROLLER_VERSION:-}" ]]; then
			CONFIG_VERSION="${CONTROLLER_VERSION}"
		fi
		os::start::configure_server "${CONFIG_VERSION}"
		#turn on audit logging for extended tests ... mimic what is done in os::start::configure_server, but don't
		# put change there - only want this for extended tests
		os::log::info "Turn on audit logging"
		cp "${SERVER_CONFIG_DIR}/master/master-config.yaml" "${SERVER_CONFIG_DIR}/master/master-config.orig2.yaml"
		openshift ex config patch "${SERVER_CONFIG_DIR}/master/master-config.orig2.yaml" --patch="{\"auditConfig\": {\"enabled\": true}}"  > "${SERVER_CONFIG_DIR}/master/master-config.yaml"

		# If the XFS volume dir mount point exists enable local storage quota in node-config.yaml so these tests can pass:
		if [[ -n "${LOCAL_STORAGE_QUOTA}" ]]; then
			# The ec2 images usually have ~5Gi of space defined for the xfs vol for the registry; want to give /registry a good chunk of that
			# to store the images created when the extended tests run
			cp "${NODE_CONFIG_DIR}/node-config.yaml" "${NODE_CONFIG_DIR}/node-config.orig2.yaml"
			openshift ex config patch "${NODE_CONFIG_DIR}/node-config.orig2.yaml" --patch='{"volumeConfig":{"localQuota":{"perFSGroup":"4480Mi"}}}' > "${NODE_CONFIG_DIR}/node-config.yaml"
		fi
		os::log::info "Using VOLUME_DIR=${VOLUME_DIR}"

		# This is a bit hacky, but set the pod gc threshold appropriately for the garbage_collector test.
		cp "${SERVER_CONFIG_DIR}/master/master-config.yaml" "${SERVER_CONFIG_DIR}/master/master-config.orig3.yaml"
		openshift ex config patch "${SERVER_CONFIG_DIR}/master/master-config.orig3.yaml" --patch='{"kubernetesMasterConfig":{"controllerArguments":{"terminated-pod-gc-threshold":["100"]}}}' > "${SERVER_CONFIG_DIR}/master/master-config.yaml"

		os::start::server "${API_SERVER_VERSION:-}" "${CONTROLLER_VERSION:-}" "${SKIP_NODE:-}"

		export KUBECONFIG="${ADMIN_KUBECONFIG}"

		os::start::registry
		if [[ -z "${SKIP_NODE:-}" ]]; then
			oc rollout status dc/docker-registry
		fi
		DROP_SYN_DURING_RESTART=1 CREATE_ROUTER_CERT=1 os::start::router

		os::log::info "Creating image streams"
		oc create -n openshift -f "${OS_ROOT}/examples/image-streams/image-streams-centos7.json" --config="${ADMIN_KUBECONFIG}"
	else
		# be sure to set VOLUME_DIR if you are running with TEST_ONLY
		os::log::info "Not starting server, VOLUME_DIR=${VOLUME_DIR:-}"
	fi
}

# Run extended tests or print out a list of tests that need to be run
# Input:
# - FOCUS - the extended test focus
# - SKIP - the tests to skip
# - SHOW_ALL - if set, then only print out tests to be run
# - Arguments - arguments to pass to ginkgo
function os::test::extended::run () {
        local listArgs=()
        local runArgs=()
        if [[ -n "${FOCUS:-}" ]]; then
          listArgs+=("--ginkgo.focus=${FOCUS}")
          runArgs+=("-focus=${FOCUS}")
        fi
        if [[ -n "${SKIP:-}" ]]; then
          listArgs+=("--ginkgo.skip=${SKIP}")
          runArgs+=("-skip=${SKIP}")
        fi

	if [[ -n "${SHOW_ALL:-}" ]]; then
		PRINT_TESTS=1
		os::test::extended::test_list "${listArgs[@]}"
		return
	fi

	os::test::extended::test_list "${listArgs[@]}"

	if [[ "${TEST_COUNT}" -eq 0 ]]; then
		os::log::warn "No tests were selected"
		return
	fi

	"${GINKGO}" -v "${runArgs[@]}" "${EXTENDEDTEST}" "$@"
}

# Create a list of extended tests to be run with the given arguments
# Input:
# - Arguments to pass to ginkgo
# - SKIP_ONLY - If set, only selects tests to be skipped
# - PRINT_TESTS - If set, print the list of tests
# Output:
# - TEST_COUNT - the number of tests selected by the arguments
function os::test::extended::test_list () {
	local full_test_list=()
	local selected_tests=()

	while IFS= read -r; do
		full_test_list+=( "${REPLY}" )
	done < <(TEST_OUTPUT_QUIET=true "${EXTENDEDTEST}" "$@" --ginkgo.dryRun --ginkgo.noColor )
	if [[ "{$REPLY}" ]]; then lines+=( "$REPLY" ); fi

	for test in "${full_test_list[@]}"; do
		if [[ -n "${SKIP_ONLY:-}" ]]; then
			if grep -q "35mskip" <<< "${test}"; then
				selected_tests+=( "${test}" )
			fi
		else
			if grep -q "1mok" <<< "${test}"; then
				selected_tests+=( "${test}" )
			fi
		fi
	done
	if [[ -n "${PRINT_TESTS:-}" ]]; then
		if [[ ${#selected_tests[@]} -eq 0 ]]; then
			os::log::warn "No tests were selected"
		else
			printf '%s\n' "${selected_tests[@]}" | sort
		fi
	fi
	export TEST_COUNT=${#selected_tests[@]}
}
readonly -f os::test::extended::test_list

# Not run by any suite
readonly EXCLUDED_TESTS=(
	"\[Skipped\]"
	"\[Disruptive\]"
	"\[Slow\]"
	"\[Flaky\]"
	"\[Compatibility\]"

	"\[Feature:Performance\]"

	# not enabled in Origin yet
	"\[Feature:GarbageCollector\]"

	# Depends on external components, may not need yet
	Monitoring              # Not installed, should be
	"Cluster level logging" # Not installed yet
	Kibana                  # Not installed
	Ubernetes               # Can't set zone labels today
	kube-ui                 # Not installed by default
	"^Kubernetes Dashboard"  # Not installed by default (also probbaly slow image pull)

	"\[Feature:Federation\]"   # Not enabled yet
	"\[Feature:Federation12\]"   # Not enabled yet
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
	"NFS"                      # no permissions https://github.com/openshift/origin/pull/6884
	"\[Feature:Example\]"      # may need to pre-pull images
	"ResourceQuota and capture the life of a secret" # https://github.com/openshift/origin/issue/9414
	"NodeProblemDetector"        # requires a non-master node to run on
	"unchanging, static URL paths for kubernetes api services" # the test needs to exclude URLs that are not part of conformance (/logs)

	# Needs triage to determine why it is failing
	"Addon update"          # TRIAGE
	SSH                     # TRIAGE
	"\[Feature:Upgrade\]"   # TRIAGE
	"SELinux relabeling"    # started failing
	"openshift mongodb replication creating from a template" # flaking on deployment
	"Update Demo should do a rolling update of a replication controller" # this is flaky and needs triaging

	# Test will never work
	"should proxy to cadvisor" # we don't expose cAdvisor port directly for security reasons

	# Need to relax security restrictions
	"validates that InterPod Affinity and AntiAffinity is respected if matching" # this *may* now be safe

	# Need multiple nodes
	"validates that InterPodAntiAffinity is respected if matching 2"

	# Inordinately slow tests
	"should create and stop a working application"
	"should always delete fast" # will be uncommented in etcd3

	# tested by networking.sh and requires the environment that script sets up
	"\[networking\] OVS"
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
	"DNS for ExternalName services"
	"DNS for pods for Hostname and Subdomain annotation"
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
	"\[Feature\:PodDisruptionbudget\]"
)
