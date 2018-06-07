#!/bin/bash

# Copy secrets into a location they can be output during image run

mkdir -p "${HOME}/testsecret"
if [[ -f /tmp/secret1 ]]; then
    # Copy three secrets defined in testsecret fixture to directory
    cp /tmp/secret? "${HOME}/testsecret"
else
    echo "Unable to locate testsecret fixture files"
    exit 1
fi

mkdir -p "${HOME}/testsecret2"
if [[ -f secret1  ]]; then
    # Copy three secrets defined in testsecret2 fixture to directory
    cp secret? "${HOME}/testsecret2"
else
    echo "Unable to locate testsecret2 fixture files"
    exit 2
fi 

mkdir -p "${HOME}/testconfig"
if [[ -f /tmp/configmap/foo ]]; then
    # Copy three configMap entries defined in configmap1 fixture to directory
    cp /tmp/configmap/* "${HOME}/testconfig"
else
    echo "Unable to locate test-configmap fixture files"
    exit 3
fi

mkdir -p "${HOME}/testconfig2"
if [[ -f configmap2/foo  ]]; then
    # Copy three configMap entries defined in configmap2 fixture to directory
    cp configmap2/* "${HOME}/testconfig2"
else
    echo "Unable to locate test-configmap-2 fixture files"
    exit 4
fi
