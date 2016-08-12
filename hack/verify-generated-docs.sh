#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Docs ====="

GENERATED_DOCS_ROOT_REL="docs/generated"
GENERATED_DOCS_ROOT="${OS_ROOT}/${GENERATED_DOCS_ROOT_REL}"
TMP_GENERATED_DOCS_ROOT_REL="_output/verify-generated-docs"
TMP_GENERATED_DOCS_ROOT="${OS_ROOT}/${TMP_GENERATED_DOCS_ROOT_REL}/${GENERATED_DOCS_ROOT_REL}"

GENERATED_DOCS_MAN_ROOT_REL="docs/man/man1"
GENERATED_DOCS_MAN_ROOT="${OS_ROOT}/${GENERATED_DOCS_MAN_ROOT_REL}"
TMP_GENERATED_DOCS_MAN_ROOT="${OS_ROOT}/${TMP_GENERATED_DOCS_ROOT_REL}/${GENERATED_DOCS_MAN_ROOT_REL}"

echo "Generating fresh docs..."
if ! output=`${OS_ROOT}/hack/update-generated-docs.sh ${TMP_GENERATED_DOCS_ROOT_REL} 2>&1`
then
	echo "FAILURE: Generation of fresh docs failed:"
	echo "$output"
  exit 1
fi

echo "Diffing current docs against freshly generated docs"
ret=0
diff -Naupr "${GENERATED_DOCS_ROOT}" "${TMP_GENERATED_DOCS_ROOT}" || ret=$?
rm -rf "${TMP_GENERATED_DOCS_ROOT}"

echo "Diffing current man pages against freshly generated man pages"
retman=0
diff -Naupr "${GENERATED_DOCS_MAN_ROOT}" "${TMP_GENERATED_DOCS_MAN_ROOT}" || retman=$?
rm -rf "${TMP_GENERATED_DOCS_MAN_ROOT}"

if [[ $ret -eq 0 ]] && [[ $retman -eq 0 ]]
then
  echo "SUCCESS: Generated docs and man pages up to date."
elif [[ $ret -eq 0 ]] && [[ $retman -ne 0 ]]
then
  echo "FAILURE: Generated docs up to date, but generated man pages out of date. Please run hack/update-generated-docs.sh"
  exit 1
elif [[ $ret -ne 0 ]] && [[ $retman -eq 0 ]]
then
  echo "FAILURE: Generated man pages up to date, but generated docs out of date. Please run hack/update-generated-docs.sh"
  exit 1
else
  echo "FAILURE: Generated docs and man pages out of date. Please run hack/update-generated-docs.sh"
  exit 1
fi
