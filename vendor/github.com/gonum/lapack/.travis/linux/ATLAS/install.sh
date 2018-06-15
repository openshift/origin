set -ex

# fetch and install ATLAS
sudo apt-get update -qq
sudo apt-get install -qq libatlas-base-dev


# fetch and install gonum/blas and gonum/matrix
export CGO_LDFLAGS="-L/usr/lib -latlas -llapack_atlas"
go get github.com/gonum/blas
go get github.com/gonum/matrix/mat64

# install lapack against ATLAS
pushd cgo/lapacke
go install -v -x
popd

# travis compiles commands in script and then executes in bash.  By adding
# set -e we are changing the travis build script's behavior, and the set
# -e lives on past the commands we are providing it.  Some of the travis
# commands are supposed to exit with non zero status, but then continue
# executing.  set -x makes the travis log files extremely verbose and
# difficult to understand.
# 
# see travis-ci/travis-ci#5120
set +ex
