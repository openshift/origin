#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/admin/certs-validation"

CERT_DIR="${BASETMPDIR}/certs"
mkdir -p -- "${CERT_DIR}"

pushd "${CERT_DIR}" >/dev/null

# oadm ca create-signer-cert should generate certificate for 5 years by default
os::cmd::expect_success_and_not_text \
    "oadm ca create-signer-cert --cert='${CERT_DIR}/ca.crt' \
                                --key='${CERT_DIR}/ca.key' \
                                --serial='${CERT_DIR}/ca.serial.txt' \
                                --overwrite=true" \
    'WARNING: .* is greater than 5 years'

expected_year="$(TZ=GMT date -d "+$((365*5)) days" +'%Y')"

os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/ca.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"

# oadm ca create-signer-cert should generate certificate with specified number of days and show warning
os::cmd::expect_success_and_text \
    "oadm ca create-signer-cert --cert='${CERT_DIR}/ca.crt' \
                                --key='${CERT_DIR}/ca.key' \
                                --serial='${CERT_DIR}/ca.serial.txt' \
                                --overwrite=true \
                                --expire-days=$((365*6))" \
    'WARNING: .* is greater than 5 years'

expected_year="$(TZ=GMT date -d "+$((365*6)) days" +'%Y')"

os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/ca.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"


# oadm create-node-config should generate certificates for 2 years by default
# NOTE: tests order is important here because this test uses CA certificate that was generated before

# we have to remove these files otherwise oadm create-node-config won't generate new ones
rm -f -- ${CERT_DIR}/master-client.crt ${CERT_DIR}/server.crt

os::cmd::expect_success_and_not_text \
    "oadm create-node-config \
            --node-dir='${CERT_DIR}' \
            --node=example \
            --hostnames=example.org \
            --certificate-authority='${CERT_DIR}/ca.crt' \
            --node-client-certificate-authority='${CERT_DIR}/ca.crt' \
            --signer-cert='${CERT_DIR}/ca.crt' \
            --signer-key='${CERT_DIR}/ca.key' \
            --signer-serial='${CERT_DIR}/ca.serial.txt'" \
    'WARNING: .* is greater than 2 years'

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"
for CERT_FILE in master-client.crt server.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

# oadm create-node-config should generate certificates with specified number of days and show warning
# NOTE: tests order is important here because this test uses CA certificate that was generated before

# we have to remove these files otherwise oadm create-node-config won't generate new ones
rm -f -- ${CERT_DIR}/master-client.crt ${CERT_DIR}/server.crt

os::cmd::expect_success_and_text \
    "oadm create-node-config \
            --node-dir='${CERT_DIR}' \
            --node=example \
            --hostnames=example.org \
            --certificate-authority='${CERT_DIR}/ca.crt' \
            --node-client-certificate-authority='${CERT_DIR}/ca.crt' \
            --signer-cert='${CERT_DIR}/ca.crt' \
            --signer-key='${CERT_DIR}/ca.key' \
            --signer-serial='${CERT_DIR}/ca.serial.txt' \
            --expire-days=$((365*3))" \
    'WARNING: .* is greater than 2 years'

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"

for CERT_FILE in master-client.crt server.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done


# oadm create-api-client-config should generate certificates for 2 years by default
# NOTE: tests order is important here because this test uses CA certificate that was generated before

os::cmd::expect_success_and_not_text \
    "oadm create-api-client-config \
            --client-dir='${CERT_DIR}' \
            --user=test-user \
            --certificate-authority='${CERT_DIR}/ca.crt' \
            --signer-cert='${CERT_DIR}/ca.crt' \
            --signer-key='${CERT_DIR}/ca.key' \
            --signer-serial='${CERT_DIR}/ca.serial.txt'" \
    'WARNING: .* is greater than 2 years'

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"
os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/test-user.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"

# oadm create-api-client-config should generate certificates with specified number of days and show warning
# NOTE: tests order is important here because this test uses CA certificate that was generated before

os::cmd::expect_success_and_text \
    "oadm create-api-client-config \
            --client-dir='${CERT_DIR}' \
            --user=test-user \
            --certificate-authority='${CERT_DIR}/ca.crt' \
            --signer-cert='${CERT_DIR}/ca.crt' \
            --signer-key='${CERT_DIR}/ca.key' \
            --signer-serial='${CERT_DIR}/ca.serial.txt' \
            --expire-days=$((365*3))" \
    'WARNING: .* is greater than 2 years'

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"
os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/test-user.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"


# oadm ca create-server-cert should generate certificate for 2 years by default
# NOTE: tests order is important here because this test uses CA certificate that was generated before
os::cmd::expect_success_and_not_text \
    "oadm ca create-server-cert --signer-cert='${CERT_DIR}/ca.crt' \
                                --signer-key='${CERT_DIR}/ca.key' \
                                --signer-serial='${CERT_DIR}/ca.serial.txt' \
                                --overwrite=true \
                                --hostnames=example.org \
                                --cert='${CERT_DIR}/example.org.crt' \
                                --key='${CERT_DIR}/example.org.key'" \
    'WARNING: .* is greater than 2 years'

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"

os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/example.org.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"

# oadm ca create-server-cert should generate certificate with specified number of days and show warning
# NOTE: tests order is important here because this test uses CA certificate that was generated before
os::cmd::expect_success_and_text \
    "oadm ca create-server-cert --signer-cert='${CERT_DIR}/ca.crt' \
                                --signer-key='${CERT_DIR}/ca.key' \
                                --signer-serial='${CERT_DIR}/ca.serial.txt' \
                                --overwrite=true --hostnames=example.org \
                                --cert='${CERT_DIR}/example.org.crt' \
                                --key='${CERT_DIR}/example.org.key' \
                                --expire-days=$((365*3))" \
    'WARNING: .* is greater than 2 years'

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"

os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/example.org.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"

# oadm ca create-master-certs should generate certificates for 2 years and CA for 5 years by default
os::cmd::expect_success_and_not_text \
    "oadm ca create-master-certs --cert-dir='${CERT_DIR}' \
                                 --hostnames=example.org \
                                 --overwrite=true" \
    'WARNING: .* is greater than'

expected_ca_year="$(TZ=GMT date -d "+$((365*5)) days" +'%Y')"
for CERT_FILE in ca.crt ca-bundle.crt service-signer.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_ca_year}"
done

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"
for CERT_FILE in admin.crt {master,etcd}.server.crt master.{etcd,kubelet,proxy}-client.crt openshift-master.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

# oadm ca create-master-certs should generate certificates with specified number of days and show warnings
os::cmd::expect_success_and_text \
    "oadm ca create-master-certs --cert-dir='${CERT_DIR}' \
                                 --hostnames=example.org \
                                 --overwrite=true \
                                 --expire-days=$((365*3)) \
                                 --signer-expire-days=$((365*6))" \
    'WARNING: .* is greater than'

expected_ca_year="$(TZ=GMT date -d "+$((365*6)) days" +'%Y')"
for CERT_FILE in ca.crt ca-bundle.crt service-signer.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_ca_year}"
done

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"
for CERT_FILE in admin.crt {master,etcd}.server.crt master.{etcd,kubelet,proxy}-client.crt openshift-master.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

# Preparation for "openshift start node" tests
# NOTE: tests order is important here because this test uses client and CA certificates that were generated before

# Pre-create directory with certificates because "openshift start node" doesn't have options to specify
# alternative path to the certificates
mkdir -p -- "${CERT_DIR}/start-node/openshift.local.config/master"
cp "${CERT_DIR}/ca-bundle.crt" \
    "${CERT_DIR}/ca.crt" \
    "${CERT_DIR}/ca.key" \
    "${CERT_DIR}/ca.serial.txt" \
    "${CERT_DIR}/start-node/openshift.local.config/master"

# Pre-create kubeconfig that is required by "openshift start node"
oadm create-kubeconfig \
    --client-certificate="${CERT_DIR}/master-client.crt" \
    --client-key="${CERT_DIR}/master-client.key" \
    --certificate-authority="${CERT_DIR}/ca.crt" \
    --kubeconfig="${CERT_DIR}/start-node/cert-test.kubeconfig"

# openshift start node should generate certificates for 2 years by default
pushd start-node >/dev/null

# we have to remove these files otherwise openshift start node won't generate new ones
rm -rf openshift.local.config/node/

os::cmd::expect_failure_and_not_text \
    "timeout 30 openshift start node \
        --kubeconfig='${CERT_DIR}/start-node/cert-test.kubeconfig' \
        --volume-dir='${CERT_DIR}/volumes'" \
    'WARNING: .* is greater than'

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"
for CERT_FILE in master-client.crt server.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-node/openshift.local.config/node/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

popd >/dev/null


# openshift start node should generate certificates with specified number of days and show warning
# NOTE: tests order is important here because this test uses client and CA certificates that were generated before
pushd start-node >/dev/null

# we have to remove these files otherwise openshift start node won't generate new ones
rm -rf openshift.local.config/node/

os::cmd::expect_failure_and_text \
    "timeout 30 openshift start node \
        --kubeconfig='${CERT_DIR}/start-node/cert-test.kubeconfig' \
        --volume-dir='${CERT_DIR}/volumes' \
        --expire-days=$((365*3))" \
    'WARNING: .* is greater than'

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"
for CERT_FILE in master-client.crt server.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-node/openshift.local.config/node/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

popd >/dev/null


# openshift start master should generate certificates for 2 years and CA for 5 years by default

# we have to remove these files otherwise openshift start master won't generate new ones
rm -rf start-master
mkdir -p start-master

os::cmd::expect_success_and_not_text \
    "timeout 30 openshift start master --write-config='${CERT_DIR}/start-master'" \
    'WARNING: .* is greater than'

expected_ca_year="$(TZ=GMT date -d "+$((365*5)) days" +'%Y')"
for CERT_FILE in ca.crt ca-bundle.crt service-signer.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-master/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_ca_year}"
done

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"
for CERT_FILE in admin.crt {master,etcd}.server.crt master.{etcd,kubelet,proxy}-client.crt openshift-master.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-master/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

# openshift start master should generate certificates with specified number of days and show warnings

# we have to remove these files otherwise openshift start master won't generate new ones
rm -rf start-master
mkdir -p start-master

os::cmd::expect_success_and_text \
    "timeout 30 openshift start master --write-config='${CERT_DIR}/start-master' \
                            --expire-days=$((365*3)) \
                            --signer-expire-days=$((365*6))" \
    'WARNING: .* is greater than'

expected_ca_year="$(TZ=GMT date -d "+$((365*6)) days" +'%Y')"
for CERT_FILE in ca.crt ca-bundle.crt service-signer.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-master/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_ca_year}"
done

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"
for CERT_FILE in admin.crt {master,etcd}.server.crt master.{etcd,kubelet,proxy}-client.crt openshift-master.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-master/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done


# openshift start should generate certificates for 2 years and CA for 5 years by default

# we have to remove these files otherwise openshift start won't generate new ones
rm -rf start-all
mkdir -p start-all

pushd start-all >/dev/null
os::cmd::expect_success_and_not_text \
    "timeout 30 openshift start --write-config='${CERT_DIR}/start-all' \
                     --hostname=example.org" \
    'WARNING: .* is greater than'

expected_ca_year="$(TZ=GMT date -d "+$((365*5)) days" +'%Y')"
for CERT_FILE in ca.crt ca-bundle.crt service-signer.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-all/master/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_ca_year}"
done

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"
for CERT_FILE in admin.crt {master,etcd}.server.crt master.{etcd,kubelet,proxy}-client.crt openshift-master.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-all/master/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

for CERT_FILE in master-client.crt server.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-all/node-example.org/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

popd >/dev/null

# openshift start should generate certificates with specified number of days and show warnings

# we have to remove these files otherwise openshift start won't generate new ones
rm -rf start-all
mkdir -p start-all

pushd start-all >/dev/null
os::cmd::expect_success_and_text \
    "timeout 30 openshift start --write-config='${CERT_DIR}/start-all' \
                     --hostname=example.org \
                     --expire-days=$((365*3)) \
                     --signer-expire-days=$((365*6))" \
    'WARNING: .* is greater than'

expected_ca_year="$(TZ=GMT date -d "+$((365*6)) days" +'%Y')"
for CERT_FILE in ca.crt ca-bundle.crt service-signer.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-all/master/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_ca_year}"
done

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"
for CERT_FILE in admin.crt {master,etcd}.server.crt master.{etcd,kubelet,proxy}-client.crt openshift-master.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-all/master/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

for CERT_FILE in master-client.crt server.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/start-all/node-example.org/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

popd >/dev/null

os::test::junit::declare_suite_end

popd >/dev/null

# remove generated files only if tests passed
rm -rf -- "${CERT_DIR}"
