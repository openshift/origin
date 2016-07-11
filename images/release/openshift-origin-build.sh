#!/bin/bash

os_dir="${GOPATH}/src/github.com/openshift/origin"
build_script_path=`mktemp /tmp/build.XXX.sh`

cat <<EOF > ${build_script_path}
#!/bin/bash -e

function chown_output {
    if [ ! -z "$OWNER_GROUP" ]; then
        chown -R "$OWNER_GROUP" _output
    fi
}

cd ${os_dir}
OS_VERSION_FILE="" ./hack/build-go.sh && chown_output
EOF

echo "++ Checking for gofmt errors"
./hack/verify-gofmt.sh
if [ "$?" != "0" ]; then
  echo "Fix the gofmt errors above and then run this command again."
  exit 1
fi

if [ "$1" == "--check" ]; then
  echo "++ Checking for golint errors"
  pushd ${os_dir} >/dev/null
  ./hack/verify-golint.sh -m
  popd >/dev/null
fi

exec sh ${build_script_path}
