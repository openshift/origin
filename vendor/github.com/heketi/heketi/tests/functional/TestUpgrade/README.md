
# Upgrade Test

This is a set of tests that aims to verify that current versions of Heketi
can load databases from previous versions.

## Caveats

The test directory contains a (compressed and encoded) copy of an older
version of the database in order to avoid needing to have or build binaries
for older versions.

These tests use the mock executor and thus only test db behavior.

## Running without virtualenv

The virtualenv tool (along with its local pip) is used to resolve
dependencies needed by the test. If you wish to run the script but avoid
using virtualenv (using your own pre-configured virtualenv or some
other configuration) set the following environment variable like so:

```sh
export HEKETI_TEST_UPGRADE_VENV=no
```

Before running the test script.

## Requirements

* Python
* virtualenv (with pip)
* base64
* xz

