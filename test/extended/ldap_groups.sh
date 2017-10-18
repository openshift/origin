#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will run all tests that are imported into test/extended.
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
os::util::environment::setup_time_vars

os::build::setup_env

function cleanup() {
	return_code=$?
	os::test::junit::generate_report
	os::cleanup::all
	os::util::describe_return_code "${return_code}"
	exit "${return_code}"
}
trap "cleanup" EXIT

os::log::info "Starting server"

os::util::ensure::iptables_privileges_exist
os::util::environment::use_sudo
os::cleanup::tmpdir
os::util::environment::setup_all_server_vars

os::log::system::start

os::start::configure_server
os::start::server

export KUBECONFIG="${ADMIN_KUBECONFIG}"

os::start::registry
oc rollout status dc/docker-registry

oc login ${MASTER_ADDR} -u ldap -p password --certificate-authority=${MASTER_CONFIG_DIR}/ca.crt
oc new-project openldap

# create all the resources we need
oc create -f test/extended/testdata/ldap

is_event_template=(               \
"{{with \$tags := .status.tags}}" \
    "{{range \$tag := \$tags}}"   \
        "{{\$tag.tag}} "          \
    "{{end}}"                     \
"{{end}}"                         \
)
is_event_template=$(IFS=""; echo "${is_event_template[*]}") # re-formats template for use

os::test::junit::declare_suite_start "extended/ldap-groups/setup"
# wait until the last event that occurred on the imagestream was the successful pull of the latest image
os::cmd::try_until_text "oc get imagestream openldap --template='${is_event_template}'" 'latest' "$((60*TIME_SEC))"

# kick off a build and wait for it to finish
oc start-build openldap --follow

server_ready_template=(                                  \
"{{with \$items := .items}}"                             \
    "{{with \$item := index \$items 0}}"                 \
        "{{range \$map := \$item.status.conditions}}"    \
            "{{with \$state := index \$map \"type\"}}"   \
                "{{\$state}}"                            \
            "{{end}}"                                    \
            "{{with \$valid := index \$map \"status\"}}" \
                "{{\$valid}} "                           \
            "{{end}}"                                    \
        "{{end}}"                                        \
    "{{end}}"                                            \
"{{end}}"                                                \
)
server_ready_template=$(IFS=$""; echo "${server_ready_template[*]}") # re-formats template for use

# wait for LDAP server to be ready
os::cmd::try_until_text "oc get pods -l deploymentconfig=openldap-server --template='${server_ready_template}'" "ReadyTrue " "$((60*TIME_SEC))"

oc login -u system:admin -n openldap
os::test::junit::declare_suite_end

LDAP_SERVICE_IP=$(oc get --output-version=v1 --template="{{ .spec.clusterIP }}" service openldap-server)

function compare_and_cleanup() {
	validation_file=$1
	actual_file=actual-${validation_file}
	rm -f ${WORKINGDIR}/${actual_file}
	oc get groups --no-headers | awk '{print $1}' | sort | xargs -I{} oc export group {} -o yaml >> ${WORKINGDIR}/${actual_file}
	os::util::sed '/sync-time/d' ${WORKINGDIR}/${actual_file}
	diff ${validation_file} ${WORKINGDIR}/${actual_file}
	oc delete groups --all
	echo -e "\tSUCCESS"
}

oc login -u system:admin -n default

os::log::info "Running extended tests"

schema=('rfc2307' 'ad' 'augmented-ad')

for (( i=0; i<${#schema[@]}; i++ )); do
	current_schema=${schema[$i]}
	os::log::info "Testing schema: ${current_schema}"
	os::test::junit::declare_suite_start "extended/ldap-groups/${current_schema}"

	WORKINGDIR=${BASETMPDIR}/${current_schema}
	mkdir ${WORKINGDIR}

	# create a temp copy of the test files
	cp test/extended/authentication/ldap/${current_schema}/* ${WORKINGDIR}
	pushd ${WORKINGDIR} > /dev/null

	# load OpenShift and LDAP group UIDs, needed for literal whitelists
	# use awk instead of sed for compatibility (see os::util::sed)
	group1_ldapuid=$(awk 'NR == 1 {print $0}' ldapgroupuids.txt)
	group2_ldapuid=$(awk 'NR == 2 {print $0}' ldapgroupuids.txt)
	group3_ldapuid=$(awk 'NR == 3 {print $0}' ldapgroupuids.txt)

	group1_osuid=$(awk 'NR == 1 {print $0}' osgroupuids.txt)
	group2_osuid=$(awk 'NR == 2 {print $0}' osgroupuids.txt)
	group3_osuid=$(awk 'NR == 3 {print $0}' osgroupuids.txt)

	# update sync-configs and validation files with the LDAP server's IP
	config_files=sync-config*.yaml
	validation_files=valid*.yaml
	for config in ${config_files} ${validation_files}
	do
		os::util::sed "s/LDAP_SERVICE_IP/${LDAP_SERVICE_IP}/g" ${config}
	done

	echo -e "\tTEST: Sync all LDAP groups from LDAP server"
	oc adm groups sync --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_all_ldap_sync.yaml


	# WHITELISTS
	echo -e "\tTEST: Sync subset of LDAP groups from LDAP server using whitelist file"
	oc adm groups sync --whitelist=whitelist_ldap.txt --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_whitelist_sync.yaml

	echo -e "\tTEST: Sync subset of LDAP groups from LDAP server using literal whitelist"
	oc adm groups sync ${group1_ldapuid} --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_whitelist_sync.yaml

	echo -e "\tTEST: Sync subset of LDAP groups from LDAP server using union of literal whitelist and whitelist file"
	oc adm groups sync ${group2_ldapuid} --whitelist=whitelist_ldap.txt --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_whitelist_union_sync.yaml

	echo -e "\tTEST: Sync subset of OpenShift groups from LDAP server using whitelist file"
	oc adm groups sync ${group1_ldapuid} --sync-config=sync-config.yaml --confirm
	oc patch group ${group1_osuid} -p 'users: []'
	oc adm groups sync --type=openshift --whitelist=whitelist_openshift.txt --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_whitelist_sync.yaml

	echo -e "\tTEST: Sync subset of OpenShift groups from LDAP server using literal whitelist"
	# sync group from LDAP
	oc adm groups sync ${group1_ldapuid} --sync-config=sync-config.yaml --confirm
	oc patch group ${group1_osuid} -p 'users: []'
	oc adm groups sync --type=openshift ${group1_osuid} --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_whitelist_sync.yaml

	echo -e "\tTEST: Sync subset of OpenShift groups from LDAP server using union of literal whitelist and whitelist file"
	# sync groups from LDAP
	oc adm groups sync ${group1_ldapuid} ${group2_ldapuid} --sync-config=sync-config.yaml --confirm
	oc patch group ${group1_osuid} -p 'users: []'
	oc patch group ${group2_osuid} -p 'users: []'
	oc adm groups sync --type=openshift group/${group2_osuid} --whitelist=whitelist_openshift.txt --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_whitelist_union_sync.yaml


	# BLACKLISTS
	echo -e "\tTEST: Sync subset of LDAP groups from LDAP server using whitelist and blacklist file"
	# oc adm groups sync --whitelist=ldapgroupuids.txt --blacklist=blacklist_ldap.txt --blacklist-group="${group1_ldapuid}" --sync-config=sync-config.yaml --confirm
	oc adm groups sync --whitelist=ldapgroupuids.txt --blacklist=blacklist_ldap.txt --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_all_blacklist_sync.yaml

	echo -e "\tTEST: Sync subset of LDAP groups from LDAP server using blacklist"
	# oc adm groups sync --blacklist=blacklist_ldap.txt --blacklist-group=${group1_ldapuid} --sync-config=sync-config.yaml --confirm
	oc adm groups sync --blacklist=blacklist_ldap.txt --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_all_blacklist_sync.yaml

	echo -e "\tTEST: Sync subset of OpenShift groups from LDAP server using whitelist and blacklist file"
	oc adm groups sync --sync-config=sync-config.yaml --confirm
	oc get group -o name --no-headers | xargs -n 1 oc patch -p 'users: []'
	# oc adm groups sync --type=openshift --whitelist=osgroupuids.txt --blacklist=blacklist_openshift.txt --blacklist-group=${group1_osuid} --sync-config=sync-config.yaml --confirm
	oc adm groups sync --type=openshift --whitelist=osgroupuids.txt --blacklist=blacklist_openshift.txt --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_all_openshift_blacklist_sync.yaml


	# MAPPINGS
	echo -e "\tTEST: Sync all LDAP groups from LDAP server using a user-defined mapping"
	oc adm groups sync --sync-config=sync-config-user-defined.yaml --confirm
	compare_and_cleanup valid_all_ldap_sync_user_defined.yaml

	echo -e "\tTEST: Sync all LDAP groups from LDAP server using a partially user-defined mapping"
	oc adm groups sync --sync-config=sync-config-partially-user-defined.yaml --confirm
	compare_and_cleanup valid_all_ldap_sync_partially_user_defined.yaml

	echo -e "\tTEST: Sync based on OpenShift groups respecting OpenShift mappings"
	oc adm groups sync --sync-config=sync-config-user-defined.yaml --confirm
	oc get group -o name --no-headers | xargs -n 1 oc patch -p 'users: []'
	oc adm groups sync --type=openshift --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_all_ldap_sync_user_defined.yaml

	echo -e "\tTEST: Sync all LDAP groups from LDAP server using DN as attribute whenever possible"
    oc adm groups sync --sync-config=sync-config-dn-everywhere.yaml --confirm
	compare_and_cleanup valid_all_ldap_sync_dn_everywhere.yaml

	echo -e "\tTEST: Sync based on OpenShift groups respecting OpenShift mappings and whitelist file"
	os::cmd::expect_success_and_text 'oc adm groups sync --whitelist=ldapgroupuids.txt --sync-config=sync-config-user-defined.yaml --confirm' 'group/'
	os::cmd::expect_success_and_text 'oc get group -o jsonpath={.items[*].metadata.name}' 'firstgroup secondgroup thirdgroup'
	os::cmd::expect_success_and_text 'oc adm groups sync --type=openshift --whitelist=ldapgroupuids.txt --sync-config=sync-config-user-defined.yaml --confirm' 'group/'
	os::cmd::expect_success_and_text 'oc get group -o jsonpath={.items[*].metadata.name}' 'firstgroup secondgroup thirdgroup'
	os::cmd::expect_success_and_text 'oc delete groups --all' 'deleted'
	os::cmd::expect_success_and_text 'oc get group -o jsonpath={.items[*].metadata.name} | wc -l' '0'


	# PRUNING
	echo -e "\tTEST: Sync all LDAP groups from LDAP server, change LDAP UID, then prune OpenShift groups"
	oc adm groups sync --sync-config=sync-config.yaml --confirm
	oc patch group ${group2_osuid} -p "{\"metadata\":{\"annotations\":{\"openshift.io/ldap.uid\":\"cn=garbage,${group2_ldapuid}\"}}}"
	oc adm groups prune --sync-config=sync-config.yaml --confirm
	compare_and_cleanup valid_all_ldap_sync_prune.yaml

	echo -e "\tTEST: Sync all LDAP groups from LDAP server using whitelist file, then prune OpenShift groups using the same whitelist file"
	os::cmd::expect_success_and_text 'oc adm groups sync --whitelist=ldapgroupuids.txt --sync-config=sync-config-user-defined.yaml --confirm' 'group/'
	os::cmd::expect_success_and_text 'oc get group -o jsonpath={.items[*].metadata.name}' 'firstgroup secondgroup thirdgroup'
	os::cmd::expect_success_and_text 'oc adm groups prune --whitelist=ldapgroupuids.txt --sync-config=sync-config-user-defined.yaml --confirm | wc -l' '0'
	os::cmd::expect_success_and_text 'oc get group -o jsonpath={.items[*].metadata.name}' 'firstgroup secondgroup thirdgroup'
	os::cmd::expect_success_and_text 'oc patch group secondgroup -p "{\"metadata\":{\"annotations\":{\"openshift.io/ldap.uid\":\"cn=garbage\"}}}"' 'group "secondgroup" patched'
	os::cmd::expect_success_and_text 'oc adm groups prune --whitelist=ldapgroupuids.txt --sync-config=sync-config-user-defined.yaml --confirm' 'group/secondgroup'
	os::cmd::expect_success_and_text 'oc get group -o jsonpath={.items[*].metadata.name}' 'firstgroup thirdgroup'
	os::cmd::expect_success_and_text 'oc delete groups --all' 'deleted'
	os::cmd::expect_success_and_text 'oc get group -o jsonpath={.items[*].metadata.name} | wc -l' '0'


	# PAGING
	echo -e "\tTEST: Sync all LDAP groups from LDAP server using paged queries"
	oc adm groups sync --sync-config=sync-config-paging.yaml --confirm
	compare_and_cleanup valid_all_ldap_sync.yaml


	os::test::junit::declare_suite_end
    popd > /dev/null
done

# special test for RFC2307
pushd ${BASETMPDIR}/rfc2307 > /dev/null
echo -e "\tTEST: Sync groups from LDAP server, tolerating errors"
oc adm groups sync --sync-config=sync-config-tolerating.yaml --confirm 2>"${LOG_DIR}/tolerated-output.txt"
grep 'For group "cn=group1,ou=groups,ou=incomplete\-rfc2307,dc=example,dc=com", ignoring member "cn=INVALID,ou=people,ou=rfc2307,dc=example,dc=com"' "${LOG_DIR}/tolerated-output.txt"
grep 'For group "cn=group2,ou=groups,ou=incomplete\-rfc2307,dc=example,dc=com", ignoring member "cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com"' "${LOG_DIR}/tolerated-output.txt"
grep 'For group "cn=group3,ou=groups,ou=incomplete\-rfc2307,dc=example,dc=com", ignoring member "cn=INVALID,ou=people,ou=rfc2307,dc=example,dc=com"' "${LOG_DIR}/tolerated-output.txt"
grep 'For group "cn=group3,ou=groups,ou=incomplete\-rfc2307,dc=example,dc=com", ignoring member "cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com"' "${LOG_DIR}/tolerated-output.txt"
compare_and_cleanup valid_all_ldap_sync_tolerating.yaml
popd > /dev/null

# special test for augmented-ad
pushd ${BASETMPDIR}/augmented-ad > /dev/null
echo -e "\tTEST: Sync all LDAP groups from LDAP server, remove LDAP group metadata entry, then prune OpenShift groups"
oc adm groups sync --sync-config=sync-config.yaml --confirm
ldapdelete -x -h $LDAP_SERVICE_IP -p 389 -D cn=Manager,dc=example,dc=com -w admin "${group1_ldapuid}"
oc adm groups prune --sync-config=sync-config.yaml --confirm
compare_and_cleanup valid_all_ldap_sync_delete_prune.yaml
popd > /dev/null
