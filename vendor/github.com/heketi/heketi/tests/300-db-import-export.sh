#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

cd "${SCRIPT_DIR}/functional/TestDbExportImport" || exit 1


require_heketi_binaries() {
	if [ ! -x heketi-server ] || [ ! -x heketi-cli ] ; then
		(cd ../../../ && make)
		cp ../../../heketi heketi-server
		cp ../../../client/cli/go/heketi-cli heketi-cli
	fi
}

start_server() {
	rm -f heketi.db &> /dev/null
	./heketi-server --config="./heketi.json" --disable-auth &> heketi.log &
	server_pid=$!
	sleep 2
}

restart_server() {
	./heketi-server --config="./heketi.json" --disable-auth &>> heketi.log &
	server_pid=$!
	sleep 2
}


kill_server() {
        if [[ -n $server_pid ]]
        then
                kill "${server_pid}"
                server_pid=""
        fi
}

show_err() {
        if [[ $? -ne 0 ]]
        then
                echo -e "\\nFAIL: error on line $1"
        fi
}

cleanup() {
        kill_server
	rm -f heketi.db* &> /dev/null
	rm -f db.json.* &> /dev/null
	rm -f heketi.log &> /dev/null
	rm -f heketi-server &> /dev/null
	rm -f heketi-cli &> /dev/null
	rm -f topologyinfo.* &> /dev/null
}

reset_gen_id() {
python <<EOF
path="$1"

import json

with open(path) as fh:
    j = json.load(fh)

if j['dbattributeentries'].get('DB_GENERATION_ID'):
    j['dbattributeentries']['DB_GENERATION_ID']['Value'] = 'x-fake-id'

with open(path, 'w') as fh:
    json.dump(j, fh)
EOF
}

require_heketi_binaries
start_server
trap 'cleanup $LINENO' EXIT
trap 'show_err $LINENO' ERR

# populate db
./heketi-cli --server "http://127.0.0.1:8080" topology load --json topology.json &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" volume create --size 100 --block=true &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" volume create --size 100 --snapshot-factor=1.25 &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" blockvolume create --size 1 &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" volume create --size 2 --durability=disperse  --disperse-data=2 --redundancy=1 &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" volume create --size 2 --durability=none  --gluster-volume-options="performance.rda-cache-limit 10MB" &> /dev/null
./heketi-cli --server "http://127.0.0.1:8080" topology info > topologyinfo.original

# tool should not open db in use
if ./heketi-server db export --jsonfile db.json.failcase --dbfile heketi.db &> /dev/null
then
        echo "FAILED: tool could open the db file when in use"
        exit 1
fi

# stop server and free db
kill_server

# test one cycle of export and import
./heketi-server db export --jsonfile db.json.original --dbfile heketi.db &> /dev/null

# verify that the dump contains a db gen id
python <<EOF
path="db.json.original"

import json

with open(path) as fh:
    j = json.load(fh)

assert 'dbattributeentries' in j
assert 'DB_GENERATION_ID' in j['dbattributeentries']
assert 'Key' in j['dbattributeentries']['DB_GENERATION_ID']
assert j['dbattributeentries']['DB_GENERATION_ID']['Key'] == 'DB_GENERATION_ID'
assert 'Value' in j['dbattributeentries']['DB_GENERATION_ID']
assert len(j['dbattributeentries']['DB_GENERATION_ID']['Value']) == 32
EOF

./heketi-server db import --jsonfile db.json.original --dbfile heketi.db.new &> /dev/null
./heketi-server db export --jsonfile db.json.new --dbfile heketi.db.new

# reset the db generation id to a dummy value, otherwise all dumps are unique
reset_gen_id db.json.original
reset_gen_id db.json.new
diff db.json.original db.json.new &> /dev/null

# existing json file should not be overwritten
if ./heketi-server db export --jsonfile db.json.original --dbfile heketi.db &> /dev/null
then
        echo "FAILED: overwrote the json file"
        exit 1
fi

# existing db file should not be overwritten
if ./heketi-server db import --jsonfile db.json.original --dbfile heketi.db.new &> /dev/null
then
        echo "FAILED: overwrote the db file"
        exit 1
fi

restart_server
./heketi-cli --server "http://127.0.0.1:8080" topology info > topologyinfo.new
diff topologyinfo.original topologyinfo.new &> /dev/null

# Generate "special" db contents
go test -timeout=1h -tags dbexamples -v

# can we dump dbs with pending operations
./heketi-server db export --jsonfile db.json.TestLeakPendingVolumeCreate \
    --dbfile heketi.db.TestLeakPendingVolumeCreate &> /dev/null
python <<EOF
import json
j = json.load(open("db.json.TestLeakPendingVolumeCreate"))

# check basic key presence
assert "pendingoperations" in j
assert "nonsense" not in j

# check expected numbers of items
assert len(j["pendingoperations"]) == 1
assert len(j["volumeentries"]) == 1
assert "DB_HAS_PENDING_OPS_BUCKET" in j["dbattributeentries"]

# check that pending operations match pending ids
vol = list(j["volumeentries"].values())[0]
op = list(j["pendingoperations"].values())[0]
assert vol["Pending"]["Id"] == op["Id"]
for b in j["brickentries"].values():
    assert b["Pending"]["Id"] == op["Id"]
EOF
# round trip it
./heketi-server db import --jsonfile db.json.TestLeakPendingVolumeCreate \
    --dbfile heketi.db.TestLeakPendingVolumeCreate_2 &> /dev/null
./heketi-server db export --jsonfile db.json.TestLeakPendingVolumeCreate_2 \
     --dbfile heketi.db.TestLeakPendingVolumeCreate_2 &> /dev/null

reset_gen_id db.json.TestLeakPendingVolumeCreate
reset_gen_id db.json.TestLeakPendingVolumeCreate_2
diff db.json.TestLeakPendingVolumeCreate db.json.TestLeakPendingVolumeCreate_2 &> /dev/null
