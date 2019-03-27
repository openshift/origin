#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
HEKETI_DIR="$(cd "$SCRIPT_DIR" && cd ../../.. && pwd)"
HEKETI_SERVER="./heketi-server"

cd "$SCRIPT_DIR" || exit 1

(cd "$HEKETI_DIR" && make) || exit 1

cp "$HEKETI_DIR/heketi" "$HEKETI_SERVER"

export PYTHONPATH="$PYTHONPATH:$HEKETI_DIR/client/api/python"

if [[ "$HEKETI_TEST_UPGRADE_VENV" != "no" ]]; then
    if ! command -v virtualenv &>/dev/null; then
        echo "WARNING: virtualenv not installed... skipping test" >&2
        exit 0
    fi

    rm -rf .env
    virtualenv .env
    . .env/bin/activate
    pip install -r "$HEKETI_DIR/client/api/python/requirements.txt"
fi
echo '----> Running test_upgrade.py'
exec python test_upgrade.py -v "$@"
