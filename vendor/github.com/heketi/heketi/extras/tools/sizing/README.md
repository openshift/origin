
# Experimental Heketi Device Sizing Tools

## "Fitting Room"

The `fitting_room.py` tool uses the heketi api and a heketi server configured
with the mock executor to test how well Heketi will be able to place a given
amount of storage at the volume level with given device sizes.  Currently, it
only tests a one-device per four-nodes scenario. It will save the db json
to a file as well as output to a "quasi-yaml" output log.

Examples:
```
 $ export PYTHONPATH="$PYTHONPATH:$PATH_TO_HEKETI_REPO/client/api/python"
 $ python3 fitting_room.py --runs 100 --seed=0 --device-size 750 --min 1 --max 100  -o results.s750.n1.x100.t1000,2

 $ python3 fitting_room.py --runs 10 --seed=2019 --device-size 850 --min 10 --max 250  -o results.out
```

Results can be converted from the native "qausi-yaml" output format into CSV
using the `results2csv.py`.
