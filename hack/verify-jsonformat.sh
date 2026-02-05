#!/usr/bin/env bash

# Usage:
#
# To only verify that there are no errors in JSON files, run:
# $ ./hack/verify-jsonformat.sh
#
# Additionally, to verify also the formatting of JSON file, run:
# $ ./hack/verify-jsonformat.sh --format
#
# In case there is a formatting error, you can use this command to fix it
# automatically:
# $ ./hack/verify-jsonformat.sh --format --fix
#
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

json_files=$(find {examples,docs,images,test} -name "*.json")
tmp_dir=$(mktemp -d)
found=0
excluded_files=(
  "test/extended/testdata/cmd/test/cmd/testdata/new-app/bc-from-imagestreamimage.json"
  "test/extended/testdata/cmd/test/cmd/testdata/new-app/invalid.json"
  # OPM index.json files are JSON arrays, not objects - the jsonformat tool only handles objects
  "test/extended/util/compat_otp/testdata/opm/render/validate/catalog-error/operator-2/index.json"
)

set +e

format="${1:-""}"
fix="${2:-""}"

for f in $json_files; do
  if [[ " ${excluded_files[@]} " =~ " ${f} " ]]; then
    continue
  fi
  tmp_file="${tmp_dir}$(basename $f)"
  go run ./hack/jsonformat/main.go ${f} > ${tmp_file}
  result=$?
  if [ "${format}" == "--format" ]; then
    if ! diff --brief ${f} ${tmp_file} > /dev/null; then
      if [ "$result" == "0" -a "${fix}" == "--fix" ]; then
        echo "Fixing JSON formatting for '${f}'"
        cp ${tmp_file} ${f}
      else
        echo "JSON formatting problem: '${f}'"
        found=1
      fi
      continue
    fi
  fi
  [ "$result" != "0" ] && found=1
done

rm -rf ${tmp_dir}

if [ "$found" == "1" ]; then
  echo -e "\nThere are problems with some JSON files, to verify them you can run:"
  echo -e "$ go run ./hack/jsonformat.go <filename>\n"
  exit
fi
