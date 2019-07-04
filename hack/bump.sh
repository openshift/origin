#!/bin/bash -e

export GO111MODULE=on

if [ $# = 0 ]; then
    echo "Updates dependencies in go.mod."
    echo
    echo "  hack/bump.sh all                 Updates all branches to latest heads."
    echo "  hack/bump.sh k8s.io/gengo ...    Updates k8s.io/gengo to the latest head of its branch."
    echo ""
    exit 0
fi

if [[ " ${args[@]} " =~ " all " ]] && [ $# != 1 ]; then
    echo "Either pass 'all' as only argument or only packages, but not both."
    echo ""
    exit 1
fi

function yaml2json() {
    python -c 'import sys, yaml, json; json.dump(yaml.load(sys.stdin), sys.stdout)'
}

# for the transition we have a glide.lock. This will go away, then
# we rely on completeness of go.mod.
if [ -f glide.lock ]; then
    # get hardcoded versions from lock first
    grep -v "updated:" glide.lock | yaml2json | jq -r '.imports | map("\(.name) \(.version) \(.repo)")|.[]' | while read p v r; do
        if [ -z "$r" ] || [ "$r" = null ]; then
            r="$p"
        else
            r=$(echo $r | sed 's#https://##;s#.git$##')
        fi
        if ! [[ $p == gopkg.in/*.v* ]]; then
            echo "Locking (from glide.lock) $p@v0.0.0-20180919145318-${v:0:10}"
            go mod edit -require "$p@v0.0.0-20180919145318-${v:0:10}"
            go mod edit -replace "$p=$r@v0.0.0-20180919145318-${v:0:10}"
        fi
    done
fi

args=( "$@" )

function shouldBeUpdated() {
    [[ "${args[0]}" = "all" ]] || [[ " ${args[@]} " =~ " $1 " ]]
}

# update go.mod with entries of glide.yaml
grep -v "updated:" glide.yaml | yaml2json | jq -r '.import | map("\(.package) \(.version) \(.repo)")|.[]' | while read p v r; do
    unset major modver
    if [ -z "$r" ] || [ "$r" = null ]; then
        remote="https://$p.git"
        r="$p"
    else
        remote="${r%.git}.git"
        r=$(echo $r | sed 's#https://##;s#.git$##')
    fi
    if [[ "$v" == v*.*.* ]] && ! [[ $p == *.v* ]]; then
        dir=$(go mod download -json "$p@$v" | jq -r '.Dir' 2>/dev/null)
        if [ -f "${dir}/go.mod" ]; then
            modver="$v"
            major=$(echo "$v" | cut -f1 -d.)
            echo "Found go.mod in $p@$modver"
            echo "Semver $p@$v => $modver"
        fi
        # else fall trough to non-sha case below
    fi
    if echo "$v" | grep -q -x '[a-z0-9]{40}'; then
        modver="v0.0.0-20190702153934-${v:0:10}"
        echo "Sha $p@$v => $modver"
    elif [ -z "$modver" ]; then
        if ! shouldBeUpdated "$p"; then
            continue
        fi

        # a branch
        sha=$(git ls-remote "${remote}" "$v" | awk '{print $1}')
        if [ -z "$sha" ]; then
            echo "Cannot get sha of $p@$v"
            exit 1
        fi
        modver="v0.0.0-20190702153934-${sha:0:10}"
        echo "Ref $p@$v => $modver"
    fi

    go mod edit -require "$p@${major:-${modver}}"
    go mod edit -replace "$p=$r@$modver"
done

echo ""
echo "Now run hack/update-vendor.sh"