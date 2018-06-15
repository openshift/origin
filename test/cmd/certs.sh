#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/admin/certs-validation"

CERT_DIR="${BASETMPDIR}/certs"
mkdir -p -- "${CERT_DIR}"

pushd "${CERT_DIR}" >/dev/null

# oc adm ca create-signer-cert should generate certificate for 5 years by default
os::cmd::expect_success_and_not_text \
    "oc adm ca create-signer-cert --cert='${CERT_DIR}/ca.crt' \
                                --key='${CERT_DIR}/ca.key' \
                                --serial='${CERT_DIR}/ca.serial.txt' \
                                --overwrite=true" \
    'WARNING: .* is greater than 5 years'

os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '01'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

expected_year="$(TZ=GMT date -d "+$((365*5)) days" +'%Y')"

os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/ca.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"

# Make a cert with the CA to see the counter increment
# We can then check to see if it gets reset due to overwrite
os::cmd::expect_success \
    "oc adm create-api-client-config \
            --client-dir='${CERT_DIR}' \
            --user=some-user \
            --certificate-authority='${CERT_DIR}/ca.crt' \
            --signer-cert='${CERT_DIR}/ca.crt' \
            --signer-key='${CERT_DIR}/ca.key' \
            --signer-serial='${CERT_DIR}/ca.serial.txt'"

os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '02'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

# oc adm ca create-signer-cert should generate certificate with specified number of days and show warning
os::cmd::expect_success_and_text \
    "oc adm ca create-signer-cert --cert='${CERT_DIR}/ca.crt' \
                                --key='${CERT_DIR}/ca.key' \
                                --serial='${CERT_DIR}/ca.serial.txt' \
                                --overwrite=true \
                                --expire-days=$((365*6))" \
    'WARNING: .* is greater than 5 years'

os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '01'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

expected_year="$(TZ=GMT date -d "+$((365*6)) days" +'%Y')"

os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/ca.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"


# oc adm create-node-config should generate certificates for 2 years by default
# NOTE: tests order is important here because this test uses CA certificate that was generated before

# we have to remove these files otherwise oc adm create-node-config won't generate new ones
rm -f -- ${CERT_DIR}/master-client.crt ${CERT_DIR}/server.crt

os::cmd::expect_success_and_not_text \
    "oc adm create-node-config \
            --node-dir='${CERT_DIR}' \
            --node=example \
            --hostnames=example.org \
            --certificate-authority='${CERT_DIR}/ca.crt' \
            --node-client-certificate-authority='${CERT_DIR}/ca.crt' \
            --signer-cert='${CERT_DIR}/ca.crt' \
            --signer-key='${CERT_DIR}/ca.key' \
            --signer-serial='${CERT_DIR}/ca.serial.txt'" \
    'WARNING: .* is greater than 2 years'

os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '03'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"
for CERT_FILE in master-client.crt server.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done

# oc adm create-node-config should generate certificates with specified number of days and show warning
# NOTE: tests order is important here because this test uses CA certificate that was generated before

# we have to remove these files otherwise oc adm create-node-config won't generate new ones
rm -f -- ${CERT_DIR}/master-client.crt ${CERT_DIR}/server.crt

os::cmd::expect_success_and_text \
    "oc adm create-node-config \
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

os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '05'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"

for CERT_FILE in master-client.crt server.crt; do
    os::cmd::expect_success_and_text \
        "openssl x509 -in '${CERT_DIR}/${CERT_FILE}' -enddate -noout | awk '{print \$4}'" \
        "${expected_year}"
done


# oc adm create-api-client-config should generate certificates for 2 years by default
# NOTE: tests order is important here because this test uses CA certificate that was generated before

os::cmd::expect_success_and_not_text \
    "oc adm create-api-client-config \
            --client-dir='${CERT_DIR}' \
            --user=test-user \
            --certificate-authority='${CERT_DIR}/ca.crt' \
            --signer-cert='${CERT_DIR}/ca.crt' \
            --signer-key='${CERT_DIR}/ca.key' \
            --signer-serial='${CERT_DIR}/ca.serial.txt'" \
    'WARNING: .* is greater than 2 years'


os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '06'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"
os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/test-user.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"

# oc adm create-api-client-config should generate certificates with specified number of days and show warning
# NOTE: tests order is important here because this test uses CA certificate that was generated before

os::cmd::expect_success_and_text \
    "oc adm create-api-client-config \
            --client-dir='${CERT_DIR}' \
            --user=test-user \
            --certificate-authority='${CERT_DIR}/ca.crt' \
            --signer-cert='${CERT_DIR}/ca.crt' \
            --signer-key='${CERT_DIR}/ca.key' \
            --signer-serial='${CERT_DIR}/ca.serial.txt' \
            --expire-days=$((365*3))" \
    'WARNING: .* is greater than 2 years'

os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '07'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"
os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/test-user.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"


# oc adm ca create-server-cert should generate certificate for 2 years by default
# NOTE: tests order is important here because this test uses CA certificate that was generated before
os::cmd::expect_success_and_not_text \
    "oc adm ca create-server-cert --signer-cert='${CERT_DIR}/ca.crt' \
                                --signer-key='${CERT_DIR}/ca.key' \
                                --signer-serial='${CERT_DIR}/ca.serial.txt' \
                                --overwrite=true \
                                --hostnames=example.org \
                                --cert='${CERT_DIR}/example.org.crt' \
                                --key='${CERT_DIR}/example.org.key'" \
    'WARNING: .* is greater than 2 years'

os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '08'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

expected_year="$(TZ=GMT date -d "+$((365*2)) days" +'%Y')"

os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/example.org.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"

# oc adm ca create-server-cert should generate certificate with specified number of days and show warning
# NOTE: tests order is important here because this test uses CA certificate that was generated before
os::cmd::expect_success_and_text \
    "oc adm ca create-server-cert --signer-cert='${CERT_DIR}/ca.crt' \
                                --signer-key='${CERT_DIR}/ca.key' \
                                --signer-serial='${CERT_DIR}/ca.serial.txt' \
                                --overwrite=true --hostnames=example.org \
                                --cert='${CERT_DIR}/example.org.crt' \
                                --key='${CERT_DIR}/example.org.key' \
                                --expire-days=$((365*3))" \
    'WARNING: .* is greater than 2 years'

os::cmd::expect_success_and_text "cat '${CERT_DIR}/ca.serial.txt'" '09'
os::cmd::expect_success_and_text "tail -c 1 '${CERT_DIR}/ca.serial.txt' | wc -l" '1'  # check for newline at end

expected_year="$(TZ=GMT date -d "+$((365*3)) days" +'%Y')"

os::cmd::expect_success_and_text \
    "openssl x509 -in '${CERT_DIR}/example.org.crt' -enddate -noout | awk '{print \$4}'" \
    "${expected_year}"

# oc adm ca create-master-certs should generate certificates for 2 years and CA for 5 years by default
os::cmd::expect_success_and_not_text \
    "oc adm ca create-master-certs --cert-dir='${CERT_DIR}' \
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

# oc adm ca create-master-certs should generate certificates with specified number of days and show warnings
os::cmd::expect_success_and_text \
    "oc adm ca create-master-certs --cert-dir='${CERT_DIR}' \
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

os::test::junit::declare_suite_end

popd >/dev/null

# remove generated files only if tests passed
rm -rf -- "${CERT_DIR}"
