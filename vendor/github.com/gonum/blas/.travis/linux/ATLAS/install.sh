set -ex

# fetch and install ATLAS libs
sudo apt-get update -qq && sudo apt-get install -qq libatlas-base-dev

# fetch and install gonum/blas against ATLAS
export CGO_LDFLAGS="-L/usr/lib -lblas"
go get github.com/gonum/blas

# run the OS common installation script
source ${TRAVIS_BUILD_DIR}/.travis/$TRAVIS_OS_NAME/install.sh

# travis compiles commands in script and then executes in bash.  By adding
# set -e we are changing the travis build script's behavior, and the set
# -e lives on past the commands we are providing it.  Some of the travis
# commands are supposed to exit with non zero status, but then continue
# executing.  set -x makes the travis log files extremely verbose and
# difficult to understand.
# 
# see travis-ci/travis-ci#5120
set +ex
