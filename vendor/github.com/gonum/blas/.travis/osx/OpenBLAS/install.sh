set -ex

# fetch and install OpenBLAS using homebrew
brew install homebrew/science/openblas

# fetch and install gonum/blas against OpenBLAS
export CGO_LDFLAGS="-L/usr/local/opt/openblas/lib -lopenblas"
go get github.com/gonum/blas
pushd cgo
go install -v -x
popd

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
