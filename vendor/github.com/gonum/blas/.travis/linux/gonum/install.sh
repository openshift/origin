set -ex

# run the OS common installation script
source ${TRAVIS_BUILD_DIR}/.travis/$TRAVIS_OS_NAME/install.sh

# change to native directory so we don't test code that depends on an external
# blas library
cd native

# travis compiles commands in script and then executes in bash.  By adding
# set -e we are changing the travis build script's behavior, and the set
# -e lives on past the commands we are providing it.  Some of the travis
# commands are supposed to exit with non zero status, but then continue
# executing.  set -x makes the travis log files extremely verbose and
# difficult to understand.
# 
# see travis-ci/travis-ci#5120
set +ex
