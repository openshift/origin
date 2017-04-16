#!/bin/bash

# This scripts tests performance on main openshift features
# 
#
# this first version focus on aws instances.
# 
# TODO
#   - add "Test Anything Protocol" outputs
#   - redirect standard & err outputs for vagrant install openshift3

set -o errexit
set -o nounset
set -o pipefail

# parameters
############

# instance name : -i --instance
instance_name=${INSTANCE_NAME:-test_perf00}

# provider name : -p --provider
provider_name=${PROVIDER_NAME:-aws}

# os name : -os
instance_os=${OS_NAME:-fedora}

# Constants
###########

os_root=$(dirname "${BASH_SOURCE}")/..
out_dir=${os_root}/_output
out_pkg_dir=${os_root}/Godeps/_workspace/pkg
go_out="${os_root}/_output/local/go/bin"

# set path so OpenShift is available
export PATH="${go_out}:${PATH}"

# some env cleaning preparation
if [ -z ${http_proxy+sfx} ];
then
    echo "http_proxy is unset"
else
    echo "http_proxy is set to '$http_proxy'"
    backup_http_proxy=$http_proxy
    echo "http_proxy is backed up"
    unset http_proxy
fi

if [ -z ${https_proxy+sfx} ];
then
    echo "https_proxy is unset"
else
    echo "https_proxy is set to '$https_proxy'"
    backup_https_proxy=$https_proxy
    echo "https_proxy is backed up"
    unset http_proxy
fi

# default workflow
# the default workflow is to have all the steps executed
# in this order. Finer options can change this.
########################################################

# build -bc -build-client
do_build_client=true

# start instance -si  -start-instances 
do_start_instances=true

# run tests -rt -run-tests
do_run_tests=true

# stop instances -pi -stop-instances
#do_stop_instances=true

# terminate instances -ti -terminate-instances
do_terminate_instances=true

# proxy to use
proxy_to_instances=

# Public DNS to use
public_dns=

# Commands
#while test -n "$1"; do
while [ "$#" -ne 0 ]; do 
    case "$1" in
        -i|--instance)            
            instance_name=$2
            shift 2
            ;;  
        -p|--provider)
            provider_name=$2
            shift 2
            ;;  
        -os|--operating-system)
            instance_os=$2
            shift 2
            ;;
        -px|--proxy)
            proxy_to_instances=$2
            shift 2
            ;;
        --public-dns)
            public_dns=$2
            shift 2
            ;;

        
        ## ===================================
        -bc|--build-client)
            do_build_client=true
            do_start_instances=false
            do_run_tests=false
            do_stop_instances=false
            do_terminate_instance=false
            shift 1
            ;;
        -si|--start-instances)
            do_build_client=true
            do_start_instances=true
            do_run_tests=false
            do_stop_instances=false
            do_terminate_instance=false
            shift 1
            ;;
        -rt|--run-tests)
            do_build_client=true
            do_start_instances=true
            do_run_tests=true
            do_stop_instances=false
            do_terminate_instance=false
            shift 1
            ;;
        -pi|--stop-instances)
            do_build_client=true
            do_start_instances=true
            do_run_tests=true
            do_stop_instances=true
            do_terminate_instance=false
            shift 1
            ;;
        -ti|--terminate-instances|-a|--all)
            do_build_client=true
            do_start_instances=true
            do_run_tests=true
            do_stop_instances=true
            do_terminate_instance=true
            shift 1
            ;; 

        ## ===================================
        
        -osi|--only-start-instances)
            do_build_client=false
            do_start_instances=true
            do_run_tests=false
            do_stop_instances=false
            do_terminate_instance=false

            shift 1
            ;;
        -ort|--only-run-tests)
            do_build_client=false
            do_start_instances=false
            do_run_tests=true
            do_stop_instances=false
            do_terminate_instance=false
            shift 1
            ;;
        -opi|--only-stop-instances)
            do_build_client=false
            do_start_instances=false
            do_run_tests=false
            do_stop_instances=true
            do_terminate_instance=false

            shift 1
            ;;
        -oti|--only-terminate-instances)
            do_build_client=false
            do_start_instances=false
            do_run_tests=false
            do_stop_instances=false
            do_terminate_instance=true

            shift 1
            ;; 
        -h|--help)
            echo "Usage:                                                                            "
            echo "                                                                                  "
            echo "   test-perf.sh [flags]                                                           "
            echo "                                                                                  "
            echo "   Will execute the performance tests according the options chosen via flags      "   
            echo "   Without flag it will follow the workflow below:                                "   
            echo "     1) build openshift client on localhost                                       "   
            echo "     2) build and start the remote instance                                       "
            echo "     3) run the performance tests                                                 "
            echo "     4) terminate the instances                                                   " 
            echo "                                                                                  "
            echo "Available flags                                                                   "
            echo "  -i  , --instance test_perf00    : Instance name that will be used               " 
            echo "  -p  , --provider aws            : Name of the provider                          "
            echo "  -os , --operating-system fedora : Operating System Used                         "
            echo "  -px , --proxy <host:port>       : Proxy to use to access instances              "
            echo "        --public-dns              : Public DNS to use to access instance          "
            echo "                                                                                  "
            echo "  -bc , --build-client       : only build the client                              "
            echo "  -si , --start-instances    : follow the workflow until starting instances       "
            echo "  -rt , --run-tests          : follow the workflow until running the tests        "
#            echo "  -pi , --stop-instances     : follow the workflow until stopping the instances   "
            echo "  -ti , --terminate-instances: follow the workflow until terminating the instances"
            echo "                                                                                  "
            echo "  -a , --all                 : All the workflow is executed.                      "
            echo "                                                                                  "
            echo "  -osi, --only-start-instances    : only starts instances                         "
            echo "  -ort, --only-run-tests          : only runs the tests                           "
#            echo "  -opi, --only-stop-instances     : only stops the instances                      "
            echo "  -oti, --only-terminate-instances: only terminates the instances                 "
            echo "                                                                                  "

            shift 1
            exit 0
            ;;
        *)
            echo "Invalid option; see --help or -h."
            exit 1
    esac
done


echo "Using openshift root : ${os_root}"
echo "Using instance name  : ${instance_name}"
echo "Using provider name  : ${provider_name}"
echo "Using os             : ${instance_os}"
echo ""

###########################################
# Build openshift client on jenkins home. #
###########################################

if [[ ${do_build_client} == true ]]
then
    echo "build openshift client."
    # 1. Cleaning as in the Makefile
    rm -rf ${out_dir} ${out_pkg_dir}
    
    # 2. Building as in the Makefile
    ${os_root}/hack/build-release.sh
    ${os_root}/hack/build-images.sh || true
fi

#############################################
# Spawning instance[s] on with the provider #
#############################################

local_certificates_path=${os_root}/openshift.local.certificates/admin
remote_certificates_path=/openshift.local.certificates/admin

if [[ ${do_start_instances} == true ]]
then
    echo "starting instances...."
    
    # Cleaning
    mkdir -p ${local_certificates_path}
    rm -fr ${os_root}/.vagrant-openshift.json ${local_certificates_path}

    # Generate the description of the instance
    pushd ${os_root}
    vagrant origin-init --stage inst --os ${instance_os} ${instance_name}
    popd
    
    # rename the instance
    #vagrant modify-instance -r ${instance_name}
    # expecting a line like this one :
    # ==> openshiftdev: Host: ec2-54-144-156-107.compute-1.amazonaws.com
    #
    
    instance_dns_from_start=$(vagrant up --provider=${provider_name} --provision | grep "Host: "| awk -F" " '{print $4}')
    
    echo "Running install-openshift3"
    vagrant install-openshift3 
    
    # Start openshit
    vagrant ssh --command "sudo systemctl start openshift.service"
    echo "instances started."

fi

###################################################################
# Actually do the perf tests
###################################################################

if [[ ${do_run_tests} == true  ]]
then
    echo "Running Tests"
    # Get back certificates 
    # see https://gist.github.com/geedew/11289350
    # scp certificate from vagrant host to local
    sshconfig=$(vagrant ssh-config)
    sshoptions=$(echo "${sshconfig}" | grep -v '^Host ' | awk -v ORS=' ' 'NF{print "-o " $1 "=" $2}')
    hostname=$(echo ${sshconfig} | grep -i 'HostName '| cut -f4 -d" " )
    
    echo "Using ssh options : ${sshoptions}"
    echo "Using hostname    : ${hostname}"
    echo "Remote certificate path : ${remote_certificates_path}"
    echo "Local certificate path  : ${local_certificates_path}"

    #echo scp -r ${sshoptions} ${hostname}:${remote_certificates_path} ${local_certificates_path}
    scp -r ${sshoptions} ${hostname}:${remote_certificates_path} ${local_certificates_path}  || true
    echo "Certificates copied in : ${local_certificates_path}"


    export KUBECONFIG=${local_certificates_path}/.kubeconfig

    if [ ! -z ${proxy_to_instances+sfx} ];
    then
        export http_proxy=${proxy_to_instances}
        export https_proxy=${proxy_to_instances}
        echo "Using proxy (http/https) : $http_proxy"
    fi
    
    if [[ -z $public_dns  ]]
    then
        if [[ -n $instance_dns_from_start  ]]
        then
            echo "Using instance dns name from vagrant up output"
            hostname=${instance_dns_from_start}
        else
            if [[ -n $hostname  ]]
            then
                echo "Using Host info from vagrant ssh config"
            else
                echo "No hostname configured : please use --public-dns to specify one"
                exit 1
            fi
        fi
    else
        echo "Using instance dns name given as parameter"
        hostname=${public_dns}
    fi
    echo " => ${hostname}"
    
    echo ${os_root}/_output/local/go/bin/openshift --server="https://${hostname}:8443" cli get services
    ${os_root}/_output/local/go/bin/openshift --server="https://${hostname}:8443" cli get services
    
    echo "Tests finished."
fi

# release, clean or set back previously changed variable
# we need to clean up proxy information because vagrant plugin
# does not use proxy information expected for openshif client
if [ ! -z ${backup_https_proxy+sfx} ];
then
    echo "https_proxy is set back to $backup_https_proxy"
    https_proxy=$backup_https_proxy
fi

if [ ! -z ${backup_http_proxy+sfx} ];
then
    echo "http_proxy is set back to $backup_http_proxy"
    http_proxy=$backup_http_proxy
fi


## Terminate the instance
#if [[ ${do_stop_instances} == true ]]
#then
#    echo "stopping instances..."
#    vagrant modify-instance -s -r ${instance_name}_terminate 
#    echo "instances stopped"
#    
#fi

# Terminate the instance
if [[ ${do_terminate_instance} == true ]]
then
    echo "terminating instances..."
    unset http_proxy
    unset https_proxy
    vagrant modify-instance -s -r ${instance_name}_terminate 
    echo "instances terminated."
fi

