#!/bin/bash

# This script ensures RPM's build properly and can
# be installed as expected

set -o nounset
set -o pipefail


# Values that can be overriden
RPM_TEST_PRODUCT=${RPM_TEST_PRODUCT:-"origin"}           # origin or atomic-enterprise
RPM_TEST_OUTPUT_DIR=${RPM_TEST_OUTPUT_DIR:-"/tmp/tito/"} # Output for all build artifacts
RPM_TEST_SKIP_LINT=${RPM_TEST_SKIP_LINT:-""}             # Set to anything to disable rpmlint test

# Values that should be left alone
REQUIRED_PACKAGES="rpmlint rpm-build tito"               # Required packages to build and test
RPM_DIR=$RPM_TEST_OUTPUT_DIR/`arch`                      # Convenience. Path to the RPM output directory
SERVICE_PREFIX="origin"                                  # Used as both RPM name and service script prefix
if [ $RPM_TEST_PRODUCT == "atomic-enterprise" ]; then
    SERVICE_PREFIX="atomic-openshift"
fi

# ===
# Testing helper functions

# Show info line
function info()
{
    printf "\033[1;37mINFO\033[0m: $1\n"
}

# Show a test pass line
function pass()
{
    printf "\033[0;32mPASS\033[0m: $1\n"
}

# Show an error line
function error()
{
    printf "\033[0;33mError\033[0m: $1\n"
}

# Show a test fail line
function fail()
{
    printf "\033[0;31mFAIL\033[0m: $1\n"
}

# Show a failure and exit if the expected return code isn't returned
function fail_out()
{
    if [ $1 -ne 0 ]; then
        fail "$2"
        exit 1
    fi
}

# Show an error and exit if the expected return code isn't returned
function error_out()
{
    if [ $1 -ne 0 ]; then
        error "$2"
        exit 1
    fi
}
# ====


# Root check
if [ `id -u` -eq 0 ]; then
    error_out 1 'Do not run tests as root.'
fi


# Verifies the environment can produce rpms
function check_environment()
{
    info "Checking environment for suitability"
    # Check for required packages
    for required_rpm in $REQUIRED_PACKAGES; do
        rpm -q $required_rpm > /dev/null
        error_out $? "$required_rpm is missing. $REQUIRED_PACKAGES must all be installed."
    done
    pass "Environment looks good. Tests can run."
}

# Cleans out the generated RPM directory
function clean_output_dir_of_rpms()
{
    rm -rf $RPM_TEST_OUTPUT_DIR/`arch`
    info "Cleaned output dir of rpms."
}

# Builds the RPMs for a product
function build_rpm()
{
    mkdir -p $RPM_TEST_OUTPUT_DIR
    clean_output_dir_of_rpms
    info "Starting tito build."
    dist=""
    if [ $RPM_TEST_PRODUCT == "atomic-enterprise" ]; then
        dist="--dist=.el7aos"
    fi
    tito build --rpm --test --offline $dist -o "$RPM_TEST_OUTPUT_DIR"
    if [ $? -ne 0 ]; then
        fail "tito failed to build rpms"
        exit 1
    fi

    pass "Build RPMS"
}

# Uses rpmlint to check for errors
function lint_rpms()
{
    rpmlint -V
    rpmlint -i $RPM_DIR/*rpm
    if [ $? -eq 64 ]; then
        fail "rpmlint reported errors. (Warnings ignored...)"
        exit 1
    fi

    pass "Lint RPMS"
}

# Ensures the proper services are in the generated RPMs
function verify_expected_services()
{
    rpm -qpl $RPM_DIR/$SERVICE_PREFIX-master*rpm | grep $SERVICE_PREFIX-master.service > /dev/null
    fail_out $? "$SERVICE_PREFIX-master.service not in the $SERVICE_PREFIX-master rpm"

    rpm -qpl $RPM_DIR/$SERVICE_PREFIX-node*rpm | grep $SERVICE_PREFIX-node.service > /dev/null
    fail_out $? "$SERVICE_PREFIX-node.service not in the $SERVICE_PREFIX-node rpm"

    pass "Verify Expected Services"
}

# Verifies that installation can happen
function test_install()
{
    info "Verifying install cases..."
    info "Testing install of all rpms"
    rpm -ivh --test $RPM_DIR/*rpm
    fail_out $? "Unable to install all packages together"

    rpm_version=`rpm -qp --qf "%{VERSION}" $RPM_DIR/$SERVICE_PREFIX-master*.rpm`

    info "Testing install of main and master"
    rpm -ivh --test $RPM_DIR/$SERVICE_PREFIX-$rpm_version*.rpm $RPM_DIR/$SERVICE_PREFIX-master*.rpm
    fail_out $? "Unable to install main and master"

    info "Testing install of main, node and tuned"
    rpm -ivh --test $RPM_DIR/$SERVICE_PREFIX-$rpm_version*.rpm $RPM_DIR/$SERVICE_PREFIX-node*.rpm $RPM_DIR/tuned-profiles-$SERVICE_PREFIX-node*rpm
    fail_out $? "Unable to install main, node and tuned"
    pass "Test Install"
}



# Run the build tests
check_environment
build_rpm
if [ -e $RPM_TEST_SKIP_LINT ]; then
    lint_rpms
fi
test_install
verify_expected_services
exit 0
