#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates,secrets --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/secrets"
# This test validates secret interaction
os::cmd::expect_failure_and_text 'oc secrets new foo --type=blah makefile=Makefile' 'error: unknown secret type "blah"'
os::cmd::expect_success 'oc secrets new foo --type=blah makefile=Makefile --confirm'
os::cmd::expect_success_and_text 'oc get secrets/foo -o jsonpath={.type}' 'blah'

os::cmd::expect_success 'oc secrets new-dockercfg dockerconfigjson --docker-username=sample-user --docker-password=sample-password --docker-email=fake@example.org'
# can't use a go template here because the output needs to be base64 decoded.  base64 isn't installed by default in all distros
os::cmd::expect_success "oc get secrets/dockerconfigjson -o jsonpath='{ .data.\.dockerconfigjson }' | base64 -d > ${HOME}/dockerconfigjson"
os::cmd::expect_success 'oc secrets new from-file .dockerconfigjson=${HOME}/dockerconfigjson'
# check to make sure the type was correctly auto-detected
os::cmd::expect_success_and_text 'oc get secret/from-file --template="{{ .type }}"' 'kubernetes.io/dockerconfigjson'
# make sure the -o works correctly
os::cmd::expect_success_and_text 'oc secrets new-dockercfg dockerconfigjson --docker-username=sample-user --docker-password=sample-password --docker-email=fake@example.org -o yaml' 'kubernetes.io/dockerconfigjson'
os::cmd::expect_success_and_text 'oc secrets new from-file .dockerconfigjson=${HOME}/dockerconfigjson -o yaml' 'kubernetes.io/dockerconfigjson'
# check to make sure malformed names fail as expected
os::cmd::expect_failure_and_text 'oc secrets new bad-name .docker=cfg=${HOME}/dockerconfigjson' "error: Key names or file paths cannot contain '='."

workingdir="$( mktemp -d )"
os::cmd::try_until_success "oc get secret/dockerconfigjson"
os::cmd::expect_success_and_text "oc extract secret/dockerconfigjson --to '${workingdir}'" '.dockerconfigjson'
os::cmd::expect_success_and_text "oc extract secret/dockerconfigjson --to=-" 'sample-user'
os::cmd::expect_success_and_text "oc extract secret/dockerconfigjson --to=-" 'sample-password'
os::cmd::expect_success_and_text "cat '${workingdir}/.dockerconfigjson'" 'sample-user'
os::cmd::expect_failure_and_text "oc extract secret/dockerconfigjson --to '${workingdir}'" 'error: .dockerconfigjson: file exists, pass --confirm to overwrite'
os::cmd::expect_failure_and_text "oc extract secret/dockerconfigjson secret/dockerconfigjson --to '${workingdir}'" 'error: .dockerconfigjson: file exists, pass --confirm to overwrite'
os::cmd::expect_success_and_text "oc extract secret/dockerconfigjson secret/dockerconfigjson --to '${workingdir}' --confirm" '.dockerconfigjson'
os::cmd::expect_success_and_text "oc extract secret/dockerconfigjson --to '${workingdir}' --confirm" '.dockerconfigjson'
os::cmd::expect_success "oc extract secret/dockerconfigjson --to '${workingdir}' --confirm | xargs rm"
os::cmd::expect_failure_and_text "oc extract secret/dockerconfigjson --to missing-dir" "stat missing-dir: no such file or directory"

# attach secrets to service account
# single secret with prefix
os::cmd::expect_success 'oc secrets add deployer dockerconfigjson'
# don't add the same secret twice
os::cmd::expect_success 'oc secrets add serviceaccounts/deployer dockerconfigjson secrets/from-file'
# make sure we can add as as pull secret
os::cmd::expect_success 'oc secrets add deployer dockerconfigjson from-file --for=pull'
# make sure we can add as as pull secret and mount secret at once
os::cmd::expect_success 'oc secrets add serviceaccounts/deployer secrets/dockerconfigjson secrets/from-file --for=pull,mount'

GIT_CONFIG_PATH="${ARTIFACT_DIR}/.gitconfig"
touch "${GIT_CONFIG_PATH}"
git config --file "${GIT_CONFIG_PATH}" user.name sample-user
git config --file "${GIT_CONFIG_PATH}" user.token password

function create_valid_file() {
	echo test_data > "${ARTIFACT_DIR}/${1}"
	echo "${ARTIFACT_DIR}/${1}"
}

CA_CERT_PATH=$(create_valid_file ca.pem)
PRIVATE_KEY_PATH=$(create_valid_file id_rsa)

os::cmd::expect_success "oc secrets new-basicauth basicauth --username=sample-user --password=sample-password --gitconfig='${GIT_CONFIG_PATH}' --ca-cert='${CA_CERT_PATH}'"
# check to make sure two mutual exclusive flags return error as expected
os::cmd::expect_failure_and_text 'oc secrets new-basicauth bad-file --password=sample-password --prompt' 'error: must provide either --prompt or --password flag'
# check to make sure incorrect .gitconfig path fail as expected
os::cmd::expect_failure_and_text 'oc secrets new-basicauth bad-file --username=user --gitconfig=/bad/path' 'error: open /bad/path: no such file or directory'

os::cmd::expect_success "oc secrets new-sshauth sshauth --ssh-privatekey='${PRIVATE_KEY_PATH}' --ca-cert='${CA_CERT_PATH}'"
# check to make sure incorrect SSH private-key path fail as expected
os::cmd::expect_failure_and_text 'oc secrets new-sshauth bad-file --ssh-privatekey=/bad/path' 'error: open /bad/path: no such file or directory'

# attach secrets to service account (deprecated)
# single secret with prefix
os::cmd::expect_success 'oc secrets add deployer basicauth'
# don't add the same secret twice
os::cmd::expect_success 'oc secrets add deployer basicauth sshauth'
# make sure we can add as as pull secret
os::cmd::expect_success 'oc secrets add deployer basicauth sshauth --for=pull'
# make sure we can add as as pull secret and mount secret at once
os::cmd::expect_success 'oc secrets add deployer basicauth sshauth --for=pull,mount'

# attach secrets to service account
# test that those secrets can be unlinked
# after they have been deleted.
os::cmd::expect_success 'oc create secret generic deleted-secret'
os::cmd::expect_success 'oc secrets link deployer deleted-secret'
# confirm our soon-to-be-deleted secret has been linked
os::cmd::expect_success_and_text "oc get serviceaccount deployer -o jsonpath='{.secrets[?(@.name==\"deleted-secret\")]}'" 'deleted\-secret'
# delete "deleted-secret" and attempt to unlink from service account
os::cmd::expect_success 'oc delete secret deleted-secret'
os::cmd::expect_failure_and_text 'oc secrets unlink deployer secrets/deleted-secret' 'Unlinked deleted secrets'
# ensure already-deleted secret has been unlinked
os::cmd::expect_success_and_not_text "oc get serviceaccount deployer -o jsonpath='{.secrets[?(@.name==\"deleted-secret\")]}'" 'deleted\-secret'

# attach secrets to service account
# single secret with prefix
os::cmd::expect_success 'oc secrets link deployer basicauth'
# don't add the same secret twice
os::cmd::expect_success 'oc secrets link deployer basicauth sshauth'
# make sure we can add as as pull secret
os::cmd::expect_success 'oc secrets link deployer basicauth sshauth --for=pull'
# make sure we can add as as pull secret and mount secret at once
os::cmd::expect_success 'oc secrets link deployer basicauth sshauth --for=pull,mount'

# Confirm that all the linked secrets are present
os::cmd::expect_success 'oc get serviceaccounts/deployer -o yaml |grep -q basicauth'
os::cmd::expect_success 'oc get serviceaccounts/deployer -o yaml |grep -q sshauth'

# Remove secrets from service account
os::cmd::expect_success 'oc secrets unlink deployer basicauth'
# Confirm that the secret was removed
os::cmd::expect_failure 'oc get serviceaccounts/deployer -o yaml |grep -q basicauth'

# Re-link that secret
os::cmd::expect_success 'oc secrets link deployer basicauth'

# Removing a non-existent secret should warn but succeed and change nothing
os::cmd::expect_failure_and_text 'oc secrets unlink deployer foobar' 'secret "foobar" not found'

# Make sure that removing an existent and non-existent secret succeeds but warns about the non-existent one
os::cmd::expect_failure_and_text 'oc secrets unlink deployer foobar basicauth' 'secret "foobar" not found'
# Make sure that the existing secret is removed
os::cmd::expect_failure 'oc get serviceaccounts/deployer -o yaml |grep -q basicauth'

# Make sure that removing a real but unlinked secret succeeds
# https://github.com/openshift/origin/pull/9234#discussion_r70832486
os::cmd::expect_failure_and_text 'oc secrets unlink deployer basicauth', 'No valid secrets found or secrets not linked to service account'

# Make sure that it succeeds if *any* of the secrets are linked
# https://github.com/openshift/origin/pull/9234#discussion_r70832486
os::cmd::expect_success 'oc secrets unlink deployer basicauth sshauth'

# Confirm that the linked one was removed
os::cmd::expect_failure 'oc get serviceaccounts/deployer -o yaml |grep -q sshauth'

# command alias
os::cmd::expect_success 'oc secret --help'
os::cmd::expect_success 'oc secret new --help'
os::cmd::expect_success 'oc secret new-dockercfg --help'
os::cmd::expect_success 'oc secret new-basicauth --help'
os::cmd::expect_success 'oc secret new-sshauth --help'
os::cmd::expect_success 'oc secret add --help'
os::cmd::expect_success 'oc secret link --help'
os::cmd::expect_success 'oc secret unlink --help'

echo "secrets: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/serviceaccounts-create-kubeconfig"
os::cmd::expect_success "oc serviceaccounts create-kubeconfig default > '${BASETMPDIR}/generated_default.kubeconfig'"
os::cmd::expect_success_and_text "KUBECONFIG='${BASETMPDIR}/generated_default.kubeconfig' oc whoami" "system:serviceaccount:$(oc project -q):default"
echo "serviceaccounts: ok"
os::test::junit::declare_suite_end
