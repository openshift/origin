# TLS Ownership registry

This registry stores expected metadata for TLS artifacts in `openshift-*` namespaces. This metadata 
is used as expected result in e2e test which validates cluster secrets and configmaps.

## How to register a new certificate

* "all tls artifacts must be registered" test dumps JSON entries which are expected to be present
* Update `tls/ownership/tls-ownership.json` with suggested JSON entry, fill in necessary metadata in `certificateAuthorityBundleInfo` or `certKeyInfo`.
* If applicable, remove entry from `tls/violations/ownership/ownership-violations.json`
* Run `make update` to validate ownership settings
