#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
HEKETI_DIR="$(cd "$SCRIPT_DIR" && cd ../../.. && pwd)"
HEKETI_SERVER="./heketi-server"

cd "$SCRIPT_DIR" || exit 1

(cd "$HEKETI_DIR" && make server) || exit 1

cp "$HEKETI_DIR/heketi" "$HEKETI_SERVER"

rm -f heketi.key heketi.crt
openssl req \
    -newkey rsa:2048 \
    -x509 \
    -nodes \
    -keyout heketi.key \
    -new \
    -out heketi.crt \
    -subj /CN=localhost \
    -extensions alt_names \
    -config ssl.conf \
    -days 3650

for fn in "heketi.key" "heketi.crt"; do
    if [[ ! -f "$fn" ]]; then
        echo "ERROR: openssl failed to create ${fn} file" >&2
        exit 1
    fi
done


if ! command -v virtualenv &>/dev/null; then
    echo "WARNING: virtualenv not installed... skipping test" >&2
    exit 0
fi

failures=()

rm -rf .env
export PYTHONPATH="$PYTHONPATH:$HEKETI_DIR/client/api/python"
virtualenv .env
. .env/bin/activate
pip install -r "$HEKETI_DIR/client/api/python/requirements.txt"

echo '----> Running test_tls.py'
python test_tls.py -v "$@"
if [[ $? -ne 0 ]]; then
    failures+=(test_tls.py)
fi

echo '----> Running client_tls_test'
go test ./client_tls_test -v -tags functional
if [[ $? -ne 0 ]]; then
    failures+=(client_tls_test)
fi

if [[ "${#failures[@]}" -gt 0 ]]; then
    echo "--- FAILED:" "${failures[@]}"
    exit 1
fi
exit 0
