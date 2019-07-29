#!/bin/bash

set -e

if [[ "$SPC" != "true" ]] || [[ -z "$GO_VERSION" ]]
then
    echo "This script is intended to be executed in an SPC,"
    echo "by run_ci_tests.sh. Using it otherwise may result"
    echo "in unplesent side-effects."
    exit 1
fi

export UPDATE_CMD="true"

# Additional packages needed ontop of the base (generic) image
case "$DISTRO" in
    *ubuntu*)
        export UPDATE_CMD="apt-get update"
        export INSTALL_CMD="apt-get -qq install bats btrfs-tools libdevmapper-dev ostree libostree-dev"
        ;;
    *fedora*)
        export INSTALL_CMD="dnf -y install bats btrfs-progs btrfs-progs-devel
                            e2fsprogs xfsprogs device-mapper-devel ostree ostree-devel"
        ;;
    *centos*)
        export INSTALL_CMD="yum install -y bats btrfs-progs btrfs-progs-devel
                            e2fsprogs xfsprogs device-mapper-devel ostree ostree-devel"
        ;;
    *)
        echo "Unknown/unsupported \$DISTRO=$DISTRO"
        exit 2
        ;;
esac

echo
echo "Executing: $INSTALL_CMD"
# Don't spam...unless it breaks
TMPFILE=$(mktemp)
$UPDATE_CMD &> $TMPFILE || ( cat $TMPFILE && exit 3 )
$INSTALL_CMD &> $TMPFILE || ( cat $TMPFILE && exit 3 )
rm -f "$TMPFILE"
echo "done"

if [[ ! -d "$HOME/.gimme" ]]
then
    echo
    echo "Setting up for go version \"$GO_VERSION\""
    # Ref: https://github.com/travis-ci/gimme/blob/master/README.md
    mkdir -p "$HOME/bin"
    curl -sL -o $HOME/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme
    chmod +x $HOME/bin/gimme
    # Set env. vars here and for any future bash sessions
    X=$(echo 'export GOPATH="$HOME/go"' | tee -a $HOME/.bashrc) && eval "$X"
    X=$(echo 'export PATH="${PATH}:$HOME/bin:${GOPATH//://bin:}/bin"' | tee -a $HOME/.bashrc) && eval "$X"
    # Install go & set env vars
    X="$($HOME/bin/gimme $GO_VERSION | tee -a $HOME/.bashrc)" && eval "$X"
    unset X
fi
source "$HOME/.bashrc"  # Just in case anything was missed

echo
echo "Build Environment:"
go env
echo "PATH=$PATH"
echo "PWD=$PWD"

echo
echo "Building/Running tests"
make install.tools
make local-binary docs local-cross local-validate
make local-test-unit local-test-integration
