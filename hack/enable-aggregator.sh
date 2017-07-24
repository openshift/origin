#!/bin/bash


oadm ca create-signer-cert --cert=openshift.local.config/master/front-proxy-ca.crt --key=openshift.local.config/master/front-proxy-ca.key
oadm create-api-client-config --certificate-authority=openshift.local.config/master/front-proxy-ca.crt --signer-cert=openshift.local.config/master/front-proxy-ca.crt --signer-key=openshift.local.config/master/front-proxy-ca.key --user aggregator-front-proxy --client-dir=openshift.local.config/master

mv openshift.local.config/master/front-proxy-ca.crt .
mv openshift.local.config/master/aggregator-front-proxy.crt .
mv openshift.local.config/master/aggregator-front-proxy.key .

rm -rf openshift.local.config/master

openshift start --write-config=openshift.local.config

mv front-proxy-ca.crt openshift.local.config/master/
mv aggregator-front-proxy.crt openshift.local.config/master/
mv aggregator-front-proxy.key openshift.local.config/master/

sed -i 's|certFile: ""|certFile: aggregator-front-proxy.crt|g' openshift.local.config/master/master-config.yaml
sed -i 's|keyFile: ""|keyFile: aggregator-front-proxy.key|g' openshift.local.config/master/master-config.yaml
sed -i 's|  requestHeader: null|  requestHeader: \n    clientCA: front-proxy-ca.crt\n    clientCommonNames:  \n    - aggregator-front-proxy \n    usernameHeaders: \n    - X-Remote-User \n    groupHeaders: \n    - X-Remote-Group \n    extraHeaderPrefixes: \n    - X-Remote-Extra-|g' openshift.local.config/master/master-config.yaml

echo 'now run sudo $(which openshift) start --master-config openshift.local.config/master/master-config.yaml --node-config openshift.local.config/$(find  openshift.local.config/ -maxdepth 1 -name "node*" | xargs basename)/node-config.yaml'