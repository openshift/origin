#!/bin/bash
# Runs all the test scripts with as much parallization as possible.


OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"
cd ${OS_ROOT}

TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${BASETMPDIR:-${TMPDIR}/openshift-test-all}"
mkdir -p ${BASETMPDIR}


function cleanup()
{
	out=$?
	echo
	if [ $out -ne 0 ]; then
		echo "[FAIL] !!!!! Test Failed !!!!"
	else
		echo "[INFO] Test Succeeded"
	fi
	echo
	
	kill_all_processes

	echo "[INFO] Exiting"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

# with --all, we're going to eventually run test-assets.sh.	To do that, we'll need install-assets.sh
# go ahead and start that in the background nowINSTALL_ASSETS_LOG=${BASETMPDIR}/install-assets.log
INSTALL_ASSETS_LOG=${BASETMPDIR}/install-assets.log
TMPDIR=${BASETMPDIR} LOG_DIR=${BASETMPDIR} hack/install-assets.sh &> ${INSTALL_ASSETS_LOG} &
INSTALL_ASSETS_PID=$!
echo "Started hack/install-assets.sh (${INSTALL_ASSETS_PID}) at $(date)"

# unit tests are always run and can run in parallel with everything
TEST_GO_LOG=${BASETMPDIR}/test-go.log
TEST_KUBE=1 KUBE_COVER=" -cover -covermode=atomic" KUBE_RACE=" -race" hack/test-go.sh &> ${TEST_GO_LOG} &
TEST_GO_PID=$!
echo "Started hack/test-go.sh (${TEST_GO_PID}) at $(date)"

# integration can run in parallel with test-cmd, but not e2e because of node concerns
TEST_INT_LOG=${BASETMPDIR}/test-integration-docker.log
hack/test-integration-docker.sh &> ${TEST_INT_LOG} &
TEST_INT_PID=$!
echo "Started hack/test-integration-docker.sh (${TEST_INT_PID}) at $(date)"

# integration can run in parallel with test-cmd, but not e2e because of node concerns
TEST_CMD_LOG=${BASETMPDIR}/test-cmd.log
hack/test-cmd.sh &> ${TEST_CMD_LOG} &
TEST_CMD_PID=$!
echo "Started hack/test-cmd.sh (${TEST_CMD_PID}) at $(date)"



wait ${INSTALL_ASSETS_PID}
INSTALL_ASSETS_RET=$?
if [ "${INSTALL_ASSETS_RET}" -eq "0" ]; then
	echo "hack/install-assets.sh (${INSTALL_ASSETS_PID}) SUCCEEDED at or before $(date)"
else
	echo "hack/install-assets.sh (${INSTALL_ASSETS_PID}) FAILED at or before $(date)"
	echo "`cat ${INSTALL_ASSETS_LOG}`"
	exit ${INSTALL_ASSETS_RET}
fi

# after install-assets is done, run test-assets.sh
TEST_ASSETS_LOG=${BASETMPDIR}/test-assets.log
hack/test-assets.sh &> ${TEST_ASSETS_LOG} &
TEST_ASSETS_PID=$!
echo "Started hack/test-assets.sh (${TEST_ASSETS_PID}) at $(date)"


wait ${TEST_INT_PID}
TEST_INT_RET=$?
if [ "${TEST_INT_RET}" -eq "0" ]; then
	echo "hack/test-integration-docker.sh (${TEST_INT_PID}) SUCCEEDED at or before $(date)"
else
	echo "hack/test-integration-docker.sh (${TEST_INT_PID}) FAILED at or before $(date)"
	echo "`cat ${TEST_INT_LOG}`"
	exit ${INSTALL_ASSETS_RET}
fi

wait ${TEST_ASSETS_PID}
TEST_ASSETS_RET=$?
if [ "${TEST_ASSETS_RET}" -eq "0" ]; then
	echo "hack/test-assets.sh (${TEST_ASSETS_PID}) SUCCEEDED at or before $(date)"
else
	FAILED=3
	echo "hack/test-assets.sh (${TEST_ASSETS_PID}) FAILED at or before $(date)"
	echo "`cat ${TEST_ASSETS_LOG}`"
	exit ${INSTALL_ASSETS_RET}
fi

# after integration and test-assets are done, run e2e
TEST_E2E_LOG=${BASETMPDIR}/test-end-to-end-docker.log
hack/test-end-to-end-docker.sh &> ${TEST_E2E_LOG} &
TEST_E2E_PID=$!
echo "Started hack/test-end-to-end-docker.sh (${TEST_INT_PID}) at $(date) at or before $(date)"


wait ${TEST_GO_PID}
TEST_GO_RET=$?
if [ "${TEST_GO_RET}" -eq "0" ]; then
	echo "hack/test-go.sh (${TEST_GO_PID}) SUCCEEDED at or before $(date)"
else
	echo "hack/test-go.sh (${TEST_GO_PID}) FAILED at or before $(date)"
	echo "`cat ${TEST_GO_LOG}`"
	exit ${INSTALL_ASSETS_RET}
fi

wait ${TEST_CMD_PID}
TEST_CMD_RET=$?
if [ "${TEST_CMD_RET}" -eq "0" ]; then
	echo "hack/test-cmd.sh (${TEST_CMD_PID}) SUCCEEDED at or before $(date)"
else
	echo "hack/test-cmd.sh (${TEST_CMD_PID}) FAILED at or before $(date)"
	echo "`cat ${TEST_CMD_LOG}`"
	exit ${INSTALL_ASSETS_RET}
fi

wait ${TEST_E2E_PID}
TEST_E2E_RET=$?
if [ "${TEST_E2E_RET}" -eq "0" ]; then
	echo "hack/test-end-to-end-docker.sh (${TEST_E2E_PID}) SUCCEEDED at or before $(date)"
else
	echo "hack/test-end-to-end-docker.sh (${TEST_E2E_PID}) FAILED at or before $(date)"
	echo "`cat ${TEST_E2E_LOG}`"
	exit ${TEST_E2E_RET}
fi


# env[:test_exit_code] = 0
# install_assets_ret =0
# unit_test_ret = 0
# verify_ret = 0
# test_integration_ret = 0
# test_cmd_ret = 0
# test_assets_ret = 0
# test_e2e_ret = 0
# test_extended_ret = 0

# threads = []


# with --all, we're going to eventually run test-assets.sh.	To do that, we'll need install-assets.sh
# go ahead and start that in the background now
# if @options[:all]
# install_assets_thread = Thread.new { 
# 	cmds = ['hack/install-assets.sh']
# 	install_assets_ret = run_tests(env, cmds, false)
# }
# threads << install_assets_thread
# end

# # unit tests are always run and can run in parallel with everything
# threads << Thread.new { 
# cmds = ['TEST_KUBE=1 KUBE_COVER=" -cover -covermode=atomic" KUBE_RACE=" -race" hack/test-go.sh']
# unit_test_ret = run_tests(env, cmds, false)
# }

# # verifies are always run and can run in parallel with everything
# threads << Thread.new { 
# cmds = ['make verify']
# verify_ret = run_tests(env, cmds, false)
# }

# if @options[:all]
# # integration can run in parallel with test-cmd, but not e2e because of node concerns
# test_integration_thread = Thread.new { 
# 	cmds = ['KUBE_RACE=" " hack/test-integration-docker.sh']
# 	test_integration_ret = run_tests(env, cmds, false)
# }
# threads << test_integration_thread

# # wait until only two jobs are running and then start test-cmd.sh.	it doesn't have any conflicts
# # but we don't want to overtax things
# test_cmd_thread = Thread.new { 
# 	cmds = ['hack/test-cmd.sh']
# 	test_cmd_ret = run_tests(env, cmds, false)
# }
# threads << test_cmd_thread

# # after install-assets is done, run test-assets.sh
# install_assets_thread.join
# cmds = ['hack/test-assets.sh']
# test_assets_ret = run_tests(env, cmds, false)

# # after integration and test-assets are done, run e2e
# test_integration_thread.join
# cmds = ['hack/test-end-to-end-docker.sh']
# test_e2e_ret = run_tests(env, cmds, true)

# # after test-e2e, run the extended
# if @options[:extended_test_packages].length > 0
# 	# for extended tests we need a ginkgo binary
# 	do_execute(env[:machine], "go get github.com/onsi/ginkgo/ginkgo", {:timeout => 60*60*2, :fail_on_error => true, :verbose => false})
# 	cmds = @options[:extended_test_packages].split(",").map{ |p| 'test/extended/'+Shellwords.escape(p)+'.sh'}
# 	test_extended_ret = run_tests(env, cmds, true)
# end

# end

# threads.each { |thr| thr.join }
# env[:test_exit_code] += install_assets_ret
# env[:test_exit_code] += unit_test_ret
# env[:test_exit_code] += verify_ret
# env[:test_exit_code] += test_integration_ret
# env[:test_exit_code] += test_cmd_ret
# env[:test_exit_code] += test_assets_ret
# env[:test_exit_code] += test_e2e_ret
# env[:test_exit_code] += test_extended_ret
