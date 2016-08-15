#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Bootstrap Bindata ====="

TMP_GENERATED_BOOTSTRAP_DIR="_output/verify-bootstrap-bindata"

echo "Generating bootstrap bindata..."
if ! output=`OUTPUT_ROOT=${TMP_GENERATED_BOOTSTRAP_DIR} ${OS_ROOT}/hack/update-generated-bootstrap-bindata.sh 2>&1`
then
	echo "FAILURE: Generation of fresh bindata failed:"
	echo "$output"
  exit 1
fi

echo "Diffing current bootstrap bindata against freshly generated bindata"
ret=0
diff -Naup "${OS_ROOT}/pkg/bootstrap/bindata.go" "${TMP_GENERATED_BOOTSTRAP_DIR}/pkg/bootstrap/bindata.go" || ret=$?
rm -rf "${TMP_GENERATED_BOOTSTRAP_DIR}"
if [[ $ret -eq 0 ]]
then
  echo "SUCCESS: Generated bootstrap bindata up to date."
else
  echo "FAILURE: Generated bootstrap bindata out of date. Please run hack/update-generated-bootstrap-bindata.sh"
  exit 1
fi
