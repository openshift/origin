#!/bin/bash -e

### Use govulncheck to check for known vulnerabilities in the project.
### Fail if vulnerabilities are found in module dependencies.
### Warn (but do not fail) on stdlib vulnerabilities.
### TODO: Include useful information (ID, URL) about the vulnerability.

go install golang.org/x/vuln/cmd/govulncheck@latest

report=`mktemp`
trap "rm $report" EXIT

govulncheck -json ./... > $report

modvulns=$(jq -r '.Vulns[].Modules[] | select(.Path != "stdlib") | [.Path, .FoundVersion, .FixedVersion]' < $report)
libvulns=$(jq -r '.Vulns[].Modules[] | select(.Path == "stdlib") | [.Path, .FoundVersion, .FixedVersion]' < $report)

echo "$modvulns"
echo "$libvulns"

# Exit nonzero iff there are any vulnerabilities in module dependencies
test -z "$modvulns"
