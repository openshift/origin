#!/bin/bash -e

cat <<EOF > /tmp/build_script.sh
#!/bin/bash -ex
cd \${GOPATH}/src/github.com/openshift/origin
OS_VERSION_FILE="" ./hack/build-go.sh && chmod -R go+rw {Godeps/_workspace/pkg,_output}
EOF

chmod +x /tmp/build_script.sh
exec /tmp/build_script.sh
