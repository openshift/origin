#!/bin/sh
set -euo pipefail
export repo=$1
export from=$2
export to=$3

release="_output/local/releases/CHANGELOG.md"

t="patch"
if [[ "${to}" == *".0-"* || "${to}" == *".0" ]]; then
  t="feature"
  v="$( echo "${to}" | cut -f1 -d'-' )"
  v="${v/%.0/}"
  v="${v/#v/}"
fi

# NAME FORK PACKAGE PATH
function component() {
  if [[ ! -f tools/godepversion/godepversion.go ]]; then
    return
  fi

  if go run tools/godepversion/godepversion.go Godeps/Godeps.json $3 &>/dev/null; then
    git show $to:Godeps/Godeps.json > /tmp/godeps.new
    new="$(go run tools/godepversion/godepversion.go /tmp/godeps.new $3)"
    git show $from:Godeps/Godeps.json > /tmp/godeps.old
    old=$(go run tools/godepversion/godepversion.go /tmp/godeps.old $3)
    if [[ "${new}" != "${old}" ]]; then
      version=$(go run tools/godepversion/godepversion.go /tmp/godeps.new $3 comment)
      echo "- Updated to $1 [$version + patches](https://github.com/$2/commits/$new)"
    else
      echo "- Updates to $1"
    fi
    git log --grep=UPSTREAM --no-merges --pretty='tformat:%H' $from..$to -- vendor/$4 | \
      xargs -L 1 /bin/sh -c 'echo "  - $( git show -s --pretty=tformat:%s $1 | cut -f 2- -d " " ) [\\$( git log $to ^$1 --merges --ancestry-path --pretty="tformat:%s" | tail -1 | cut -f 4 -d " " )](https://github.com/$repo/pull/$( git log $to ^$1 --merges --ancestry-path --pretty="tformat:%s" | tail -1 | cut -f 4 -d " " | cut -c 2- ))"' '' | sort -n
  fi
}

cat << EOF
${to}

This is a ${t} release of OpenShift Origin.

## Backwards Compatibility

- REASON [\#PR](https://github.com/$repo/pull/PR)


## Changes

EOF

if [[ "${t}" == "feature" ]]; then
  echo "[Roadmap for the v${v} release](https://ci.openshift.redhat.com/releases_overview.html#${v})"
fi

cat << EOF

[$to](https://github.com/$repo/tree/$to) ($( date +"%Y-%m-%d" )) [Full Changelog](https://github.com/$repo/compare/$from...$to)


## API

- AREA
  - REASON [\#PR](https://github.com/$repo/pull/PR)


### Component updates

EOF

component Kubernetes openshift/kubernetes k8s.io/kubernetes/pkg/api k8s.io/kubernetes
component "Docker distribution" openshift/distribution github.com/docker/distribution github.com/docker/distribution

cat << EOF


### Features

#### FEATURE DESCRIPTION

PARAGRAPH

* DESCRIPTION [\#PR](https://github.com/$repo/pull/PR)


#### Other Features

MOVE FROM BUGS


### Bugs

$( git log --merges --pretty='tformat:%p %s' --reverse "$from..$to" | cut -f 2,6 -d ' ' | xargs -L 1 /bin/sh -c 'echo "- $(git show -s --pretty=tformat:%s $1) [\\$2](https://github.com/$repo/pull/${2:1})"' '' | grep -vE "UPSTREAM|bump" )


## Release SHA256 Checksums

\`\`\`
$( grep -v '\-image-' _output/local/releases/CHECKSUM )
\`\`\`

EOF
