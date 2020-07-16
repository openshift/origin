Hacking on OpenShift Testing
============================

OpenShift's E2E test suite is maintained in this repo.

The hyperkube binaries were previously maintained in this repo, but are now
maintained in the https://github.com/openshift/kubernetes repo.

### End-to-End (e2e) and Extended Tests

End to end tests (e2e) which should verify a long set of flows in the product
as a user would see them.  Two e2e tests should not overlap more than 10% of
function and are not intended to test error conditions in detail. The project
examples should be driven by e2e tests. e2e tests can also test external
components working together.

All e2e tests are compiled into the `openshift-tests` binary.
To build the test binary, run `make build-extended-test`.

To run a specific test, or an entire suite of tests, read
[test/extended/README](https://github.com/openshift/origin/blob/master/test/extended/README.md)
for more information.

## Updating external examples

`hack/update-external-example.sh` will pull down example files from external
repositories and deposit them under the `examples` directory.
Run this script if you need to refresh an example file, or add a new one.  See
the script and `examples/quickstarts/README.md` for more details.
