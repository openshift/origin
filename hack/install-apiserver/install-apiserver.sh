#!/bin/bash

# 1.  Installer creates namespace of its choice
# 2.  Installer creates `api` service with a fixed selector (app: api) with service serving cert annotation
# 3.  Installer creates `apiserver` service account
# 4.  Installer harvests serving cert
# 5.  Installer harvests SA token
# 6.  Installer creates SA kubeconfig
# 7.  Installer binds “known” roles to SA user
# 8.  Installer provides the following information to all templates.
# 	1. etcd location
# 	2. etcd ca location
# 	3. etcd write cert/key path
# 	4. serving cert/key path
# 	5. SA kubeconfig path
# 	6. namespace name
# 9.  Installer processes and creates the pre-create template - this is the template that provides things like clusterroles, clusterrolebindings, random weird stuff.
# 10. Installer processes the static pod template - this is a template that ONLY creates a static pod.
# 11. Installer takes the pod and adds it to the node pod manifest folder
# 12. Installer waits until all the pod’s containers are ready.
# 13. Installer creates the APIService object and registers it

targetNamespace=kube-wardle
apiserverConfigDir=/home/deads/workspaces/openshift3/src/github.com/openshift/origin/openshift.local.config/kube-wardle
masterConfigDir=openshift.local.config/master
mkdir -p ${apiserverConfigDir} || true
nodeManifestDir=openshift.local.config/node-deads-dev-01/static-pods


# 1.  Installer creates namespace of its choice
oc create namespace ${targetNamespace}

# 2.  Installer creates `api` service with a fixed selector (app: api) with service serving cert annotation
oc -n ${targetNamespace} create service clusterip api --tcp=443:443
oc -n ${targetNamespace} annotate svc/api service.alpha.openshift.io/serving-cert-secret-name=api-serving-cert
until oc -n ${targetNamespace} get secrets/api-serving-cert; do
	echo "waiting for oc -n ${targetNamespace} get secrets/api-serving-cert"
	sleep 1
done

# 3.  Installer creates `apiserver` service account
oc  -n ${targetNamespace} create sa apiserver
until oc -n ${targetNamespace} sa get-token apiserver; do
	echo "waiting for oc -n ${targetNamespace} get secrets/api-serving-cert"
	sleep 1
done

# 4.  Installer harvests serving cert
oc -n ${targetNamespace} extract secret/api-serving-cert --to=${apiserverConfigDir}
mv ${apiserverConfigDir}/tls.crt ${apiserverConfigDir}/serving.crt
mv ${apiserverConfigDir}/tls.key ${apiserverConfigDir}/serving.key

# 5.  Installer harvests SA token
saToken=$(oc -n ${targetNamespace} sa get-token apiserver)

# 6.  Installer creates SA kubeconfig
# TODO do this a LOT better
# start with admin.kubeconfig
cp ${masterConfigDir}/admin.kubeconfig ${apiserverConfigDir}/kubeconfig
# remove all users
oc --config=${apiserverConfigDir}/kubeconfig config unset users
# set the service account token
configContext=$(oc --config=${apiserverConfigDir}/kubeconfig config current-context)
oc --config=${apiserverConfigDir}/kubeconfig config set-credentials serviceaccount --token=${saToken}
oc --config=${apiserverConfigDir}/kubeconfig config set-context ${configContext} --user=serviceaccount

# 7.  Installer binds “known” roles to SA user
# TODO remove this bit once we bootstrap these roles
oc create -f hack/install-apiserver/prestart-resources.yaml || true

oadm policy add-cluster-role-to-user system:auth-delegator -n ${targetNamespace} -z apiserver
oc create policybinding kube-system -n kube-system
oadm policy add-role-to-user extension-apiserver-authentication-reader -n kube-system --role-namespace=kube-system system:serviceaccount:${targetNamespace}:apiserver


# 8.  Installer provides the following information to all templates.
cp ${masterConfigDir}/ca.crt ${apiserverConfigDir}/etcd-ca.crt
cp ${masterConfigDir}/ca-bundle.crt ${apiserverConfigDir}/client-ca.crt
cp ${masterConfigDir}/master.etcd-client.crt ${apiserverConfigDir}/etcd-write.crt
cp ${masterConfigDir}/master.etcd-client.key ${apiserverConfigDir}/etcd-write.key
templateArgs="ETCD_URL=https://10.13.137.230:4001"
templateArgs="${templateArgs} CLIENT_CA=/etcd/apiserver-config/client-ca.crt"
templateArgs="${templateArgs} ETCD_CA=/etcd/apiserver-config/etcd-ca.crt"
templateArgs="${templateArgs} ETCD_WRITE_CRT=/etcd/apiserver-config/etcd-write.crt"
templateArgs="${templateArgs} ETCD_WRITE_KEY=/etcd/apiserver-config/etcd-write.key"
templateArgs="${templateArgs} SERVING_CRT=/etcd/apiserver-config/serving.crt"
templateArgs="${templateArgs} SERVING_KEY=/etcd/apiserver-config/serving.key"
templateArgs="${templateArgs} KUBECONFIG=/etcd/apiserver-config/kubeconfig"
templateArgs="${templateArgs} NAMESPACE=${targetNamespace}"
templateArgs="${templateArgs} CONFIG_DIR=${apiserverConfigDir}"
templateArgs="${templateArgs} CONFIG_DIR_MOUNT=/etcd/apiserver-config"

# 9.  Installer processes and creates the pre-create template - this is the template that provides things like clusterroles, clusterrolebindings, random weird stuff.
# nothing for wardle

# 10. Installer processes the static pod template - this is a template that ONLY creates a static pod.
oc process -f hack/install-apiserver/kube-wardle-pod-template.yaml ${templateArgs} | jq .items[0] > ${nodeManifestDir}/${targetNamespace}.yaml
