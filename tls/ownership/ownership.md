# Certificate Ownership

## Table of Contents
  - [Missing Owners (4)](#Missing-Owners-4)
    - [Certificates (2)](#Certificates-2)
    - [Certificate Authority Bundles (2)](#Certificate-Authority-Bundles-2)
  - [Bare Metal Hardware Provisioning / cluster-baremetal-operator (1)](#Bare-Metal-Hardware-Provisioning-/-cluster-baremetal-operator-1)
    - [Certificates (1)](#Certificates-1)
  - [Cloud Compute / Cloud Controller Manager (1)](#Cloud-Compute-/-Cloud-Controller-Manager-1)
    - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
  - [End User (1)](#End-User-1)
    - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
  - [Image Registry (2)](#Image-Registry-2)
    - [Certificate Authority Bundles (2)](#Certificate-Authority-Bundles-2)
  - [Machine Config Operator (3)](#Machine-Config-Operator-3)
    - [Certificates (1)](#Certificates-1)
    - [Certificate Authority Bundles (2)](#Certificate-Authority-Bundles-2)
  - [Monitoring (6)](#Monitoring-6)
    - [Certificates (2)](#Certificates-2)
    - [Certificate Authority Bundles (4)](#Certificate-Authority-Bundles-4)
  - [Networking / cluster-network-operator (39)](#Networking-/-cluster-network-operator-39)
    - [Certificates (8)](#Certificates-8)
    - [Certificate Authority Bundles (31)](#Certificate-Authority-Bundles-31)
  - [Node / Kubelet (2)](#Node-/-Kubelet-2)
    - [Certificates (2)](#Certificates-2)
  - [Operator Framework / operator-lifecycle-manager (2)](#Operator-Framework-/-operator-lifecycle-manager-2)
    - [Certificates (2)](#Certificates-2)
  - [RHCOS (2)](#RHCOS-2)
    - [Certificate Authority Bundles (2)](#Certificate-Authority-Bundles-2)
  - [apiserver-auth (5)](#apiserver-auth-5)
    - [Certificates (3)](#Certificates-3)
    - [Certificate Authority Bundles (2)](#Certificate-Authority-Bundles-2)
  - [cluster-network-operator (1)](#cluster-network-operator-1)
    - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
  - [etcd (31)](#etcd-31)
    - [Certificates (22)](#Certificates-22)
    - [Certificate Authority Bundles (9)](#Certificate-Authority-Bundles-9)
  - [kube-apiserver (46)](#kube-apiserver-46)
    - [Certificates (25)](#Certificates-25)
    - [Certificate Authority Bundles (21)](#Certificate-Authority-Bundles-21)
  - [kube-controller-manager (12)](#kube-controller-manager-12)
    - [Certificates (3)](#Certificates-3)
    - [Certificate Authority Bundles (9)](#Certificate-Authority-Bundles-9)
  - [kube-scheduler (1)](#kube-scheduler-1)
    - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
  - [service-ca (100)](#service-ca-100)
    - [Certificates (97)](#Certificates-97)
    - [Certificate Authority Bundles (3)](#Certificate-Authority-Bundles-3)


## Missing Owners (4)
### Certificates (2)
1. ns/openshift-ingress secret/router-certs-default

      **Description:** 
      

2. ns/openshift-ingress-operator secret/router-ca

      **Description:** 
      



### Certificate Authority Bundles (2)
1. ns/openshift-config-managed configmap/default-ingress-cert

      **Description:** 
      

2. ns/openshift-console configmap/default-ingress-cert

      **Description:** 
      



## Bare Metal Hardware Provisioning / cluster-baremetal-operator (1)
### Certificates (1)
1. ns/openshift-machine-api secret/metal3-ironic-tls

      **Description:** 
      



## Cloud Compute / Cloud Controller Manager (1)
### Certificate Authority Bundles (1)
1. ns/openshift-cloud-controller-manager configmap/ccm-trusted-ca

      **Description:** 
      



## End User (1)
### Certificate Authority Bundles (1)
1. ns/openshift-config configmap/user-ca-bundle

      **Description:** 
      

      Other locations:

      * file /etc/docker/certs.d/virthost.ostest.test.metalkube.org:5000/ca.crt
      



## Image Registry (2)
### Certificate Authority Bundles (2)
1. file /etc/docker/certs.d/image-registry.openshift-image-registry.svc.cluster.local:5000/ca.crt

      **Description:** 
      

      Other locations:

      * file /etc/docker/certs.d/image-registry.openshift-image-registry.svc:5000/ca.crt
      

2. file /etc/docker/certs.d/image-registry.openshift-image-registry.svc:5000/ca.crt

      **Description:** 
      

      Other locations:

      * file /etc/docker/certs.d/image-registry.openshift-image-registry.svc.cluster.local:5000/ca.crt
      



## Machine Config Operator (3)
### Certificates (1)
1. ns/openshift-machine-config-operator secret/machine-config-server-tls

      **Description:** 
      



### Certificate Authority Bundles (2)
1. ns/openshift-config configmap/initial-kube-apiserver-server-ca

      **Description:** 
      

2. file /etc/kubernetes/ca.crt

      **Description:** 
      



## Monitoring (6)
### Certificates (2)
1. ns/openshift-monitoring secret/federate-client-certs

      **Description:** 
      

2. ns/openshift-monitoring secret/metrics-client-certs

      **Description:** 
      



### Certificate Authority Bundles (4)
1. ns/openshift-monitoring configmap/alertmanager-trusted-ca-bundle

      **Description:** 
      

2. ns/openshift-monitoring configmap/kubelet-serving-ca-bundle

      **Description:** 
      

3. ns/openshift-monitoring configmap/prometheus-trusted-ca-bundle

      **Description:** 
      

4. ns/openshift-monitoring configmap/telemeter-trusted-ca-bundle

      **Description:** 
      



## Networking / cluster-network-operator (39)
### Certificates (8)
1. ns/openshift-network-node-identity secret/network-node-identity-ca

      **Description:** 
      

2. ns/openshift-network-node-identity secret/network-node-identity-cert

      **Description:** 
      

3. ns/openshift-ovn-kubernetes secret/ovn-ca

      **Description:** 
      

4. ns/openshift-ovn-kubernetes secret/ovn-cert

      **Description:** 
      

5. ns/openshift-ovn-kubernetes secret/signer-ca

      **Description:** 
      

6. ns/openshift-ovn-kubernetes secret/signer-cert

      **Description:** 
      

7. file /etc/cni/multus/certs/multus-client-\<timestamp>.pem

      **Description:** 
      

8. file /var/lib/ovn-ic/etc/ovnkube-node-certs/ovnkube-client-\<timestamp>.pem

      **Description:** 
      



### Certificate Authority Bundles (31)
1. ns/openshift-apiserver configmap/trusted-ca-bundle

      **Description:** 
      

2. ns/openshift-apiserver-operator configmap/trusted-ca-bundle

      **Description:** 
      

3. ns/openshift-authentication configmap/v4-0-config-system-trusted-ca-bundle

      **Description:** 
      

4. ns/openshift-authentication-operator configmap/trusted-ca-bundle

      **Description:** 
      

5. ns/openshift-cloud-credential-operator configmap/cco-trusted-ca

      **Description:** 
      

6. ns/openshift-cloud-network-config-controller configmap/trusted-ca

      **Description:** 
      

7. ns/openshift-cluster-csi-drivers configmap/aws-ebs-csi-driver-trusted-ca-bundle

      **Description:** 
      

8. ns/openshift-cluster-csi-drivers configmap/azure-disk-csi-driver-trusted-ca-bundle

      **Description:** 
      

9. ns/openshift-cluster-csi-drivers configmap/azure-file-csi-driver-trusted-ca-bundle

      **Description:** 
      

10. ns/openshift-cluster-csi-drivers configmap/gcp-pd-csi-driver-trusted-ca-bundle

      **Description:** 
      

11. ns/openshift-cluster-csi-drivers configmap/openstack-cinder-csi-driver-trusted-ca-bundle

      **Description:** 
      

12. ns/openshift-cluster-csi-drivers configmap/shared-resource-csi-driver-operator-trusted-ca-bundle

      **Description:** 
      

13. ns/openshift-cluster-csi-drivers configmap/vmware-vsphere-csi-driver-trusted-ca-bundle

      **Description:** 
      

14. ns/openshift-cluster-csi-drivers configmap/vsphere-csi-driver-operator-trusted-ca-bundle

      **Description:** 
      

15. ns/openshift-cluster-node-tuning-operator configmap/trusted-ca

      **Description:** 
      

16. ns/openshift-cluster-storage-operator configmap/trusted-ca-bundle

      **Description:** 
      

17. ns/openshift-config-managed configmap/trusted-ca-bundle

      **Description:** 
      

18. ns/openshift-console configmap/trusted-ca-bundle

      **Description:** 
      

19. ns/openshift-console-operator configmap/trusted-ca

      **Description:** 
      

20. ns/openshift-controller-manager configmap/openshift-global-ca

      **Description:** 
      

21. ns/openshift-image-registry configmap/trusted-ca

      **Description:** 
      

22. ns/openshift-ingress-operator configmap/trusted-ca

      **Description:** 
      

23. ns/openshift-insights configmap/trusted-ca-bundle

      **Description:** 
      

24. ns/openshift-kube-apiserver configmap/trusted-ca-bundle

      **Description:** 
      

25. ns/openshift-kube-controller-manager configmap/trusted-ca-bundle

      **Description:** 
      

26. ns/openshift-machine-api configmap/cbo-trusted-ca

      **Description:** 
      

27. ns/openshift-machine-api configmap/mao-trusted-ca

      **Description:** 
      

28. ns/openshift-marketplace configmap/marketplace-trusted-ca

      **Description:** 
      

29. ns/openshift-network-node-identity configmap/network-node-identity-ca

      **Description:** 
      

30. ns/openshift-ovn-kubernetes configmap/ovn-ca

      **Description:** 
      

31. ns/openshift-ovn-kubernetes configmap/signer-ca

      **Description:** 
      



## Node / Kubelet (2)
### Certificates (2)
1. file /var/lib/kubelet/pki/kubelet-client-\<timestamp>.pem

      **Description:** 
      

2. file /var/lib/kubelet/pki/kubelet-server-\<timestamp>.pem

      **Description:** 
      



## Operator Framework / operator-lifecycle-manager (2)
### Certificates (2)
1. ns/openshift-operator-lifecycle-manager secret/packageserver-service-cert

      **Description:** 
      

2. ns/openshift-operator-lifecycle-manager secret/pprof-cert

      **Description:** 
      



## RHCOS (2)
### Certificate Authority Bundles (2)
1. file /etc/pki/tls/cert.pem

      **Description:** 
      

      Other locations:

      * file /etc/pki/tls/certs/ca-bundle.crt
      

2. file /etc/pki/tls/certs/ca-bundle.crt

      **Description:** 
      

      Other locations:

      * file /etc/pki/tls/cert.pem
      



## apiserver-auth (5)
### Certificates (3)
1. ns/openshift-config secret/webhook-authentication-integrated-oauth

      **Description:** 
      

2. ns/openshift-kube-apiserver secret/webhook-authenticator

      **Description:** 
      

3. ns/openshift-oauth-apiserver secret/openshift-authenticator-certs

      **Description:** 
      



### Certificate Authority Bundles (2)
1. ns/openshift-config-managed configmap/oauth-serving-cert

      **Description:** 
      

2. ns/openshift-console configmap/oauth-serving-cert

      **Description:** 
      



## cluster-network-operator (1)
### Certificate Authority Bundles (1)
1. file /etc/kubernetes/cni/net.d/whereabouts.d/whereabouts.kubeconfig

      **Description:** 
      



## etcd (31)
### Certificates (22)
1. ns/openshift-apiserver secret/etcd-client

      **Description:** Client certificate for apiserver, cluster-etcd-operator and etcdctl to reach etcd. Generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

2. ns/openshift-config secret/etcd-client

      **Description:** Client certificate for apiserver, cluster-etcd-operator and etcdctl to reach etcd. Generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

3. ns/openshift-etcd secret/etcd-client

      **Description:** Client certificate for apiserver, cluster-etcd-operator and etcdctl to reach etcd. Generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

4. ns/openshift-etcd secret/etcd-metric-client

      **Description:** Client certificate for Prometheus ServiceMonitors to reach etcd grpc-proxy to retrieve metrics. Generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

5. ns/openshift-etcd secret/etcd-metric-signer

      **Description:** Generated by cluster-etcd-operator for etcd and is used to sign peer, server and client certificates for Prometheus ServiceMonitors. This certificate is valid for 1825 days and starts refreshing after 1533 days.
      

6. ns/openshift-etcd secret/etcd-peer-\<bootstrap>

      **Description:** Peer (client and server) certificate for node \<bootstrap>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

7. ns/openshift-etcd secret/etcd-peer-\<master-0>

      **Description:** Peer (client and server) certificate for node \<master-0>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-peer-\<master-0>.crt
      

8. ns/openshift-etcd secret/etcd-peer-\<master-1>

      **Description:** Peer (client and server) certificate for node \<master-1>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-peer-\<master-1>.crt
      

9. ns/openshift-etcd secret/etcd-peer-\<master-2>

      **Description:** Peer (client and server) certificate for node \<master-2>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-peer-\<master-2>.crt
      

10. ns/openshift-etcd secret/etcd-serving-\<bootstrap>

      **Description:** Serving (client and server) certificate for node \<bootstrap>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

11. ns/openshift-etcd secret/etcd-serving-\<master-0>

      **Description:** Serving (client and server) certificate for node \<master-0>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-serving-\<master-0>.crt
      

12. ns/openshift-etcd secret/etcd-serving-\<master-1>

      **Description:** Serving (client and server) certificate for node \<master-1>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-serving-\<master-1>.crt
      

13. ns/openshift-etcd secret/etcd-serving-\<master-2>

      **Description:** Serving (client and server) certificate for node \<master-2>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-serving-\<master-2>.crt
      

14. ns/openshift-etcd secret/etcd-serving-metrics-\<bootstrap>

      **Description:** Serving (client and server) certificate for node \<bootstrap>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

15. ns/openshift-etcd secret/etcd-serving-metrics-\<master-0>

      **Description:** Serving (client and server) certificate for node \<master-0>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-serving-metrics-\<master-0>.crt
      

16. ns/openshift-etcd secret/etcd-serving-metrics-\<master-1>

      **Description:** Serving (client and server) certificate for node \<master-1>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-serving-metrics-\<master-1>.crt
      

17. ns/openshift-etcd secret/etcd-serving-metrics-\<master-2>

      **Description:** Serving (client and server) certificate for node \<master-2>, generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/secrets/etcd-all-certs/etcd-serving-metrics-\<master-2>.crt
      

18. ns/openshift-etcd secret/etcd-signer

      **Description:** Generated by cluster-etcd-operator for etcd and is used to sign peer, server and client certificates. This certificate is valid for 1825 days and starts refreshing after 1533 days.
      

19. ns/openshift-etcd-operator secret/etcd-client

      **Description:** Client certificate for apiserver, cluster-etcd-operator and etcdctl to reach etcd. Generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

20. ns/openshift-etcd-operator secret/etcd-metric-client

      **Description:** Client certificate for Prometheus ServiceMonitors to reach etcd grpc-proxy to retrieve metrics. Generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

21. ns/openshift-kube-apiserver secret/etcd-client

      **Description:** Client certificate for apiserver, cluster-etcd-operator and etcdctl to reach etcd. Generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      

22. ns/openshift-oauth-apiserver secret/etcd-client

      **Description:** Client certificate for apiserver, cluster-etcd-operator and etcdctl to reach etcd. Generated by cluster-etcd-operator for etcd. This certificate is valid for 1095 days and starts refreshing after 803 days.
      



### Certificate Authority Bundles (9)
1. ns/openshift-apiserver configmap/etcd-serving-ca

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate clients and peers of etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/server-ca-bundle.crt
      

2. ns/openshift-config configmap/etcd-ca-bundle

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate clients and peers of etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/server-ca-bundle.crt
      

3. ns/openshift-config configmap/etcd-serving-ca

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate clients and peers of etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/server-ca-bundle.crt
      

4. ns/openshift-etcd configmap/etcd-ca-bundle

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate clients and peers of etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/server-ca-bundle.crt
      

5. ns/openshift-etcd configmap/etcd-metrics-ca-bundle

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate Prometheus ServiceMonitors reaching etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/metrics-ca-bundle.crt
      

6. ns/openshift-etcd-operator configmap/etcd-ca-bundle

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate clients and peers of etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/server-ca-bundle.crt
      

7. ns/openshift-etcd-operator configmap/etcd-metric-serving-ca

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate Prometheus ServiceMonitors reaching etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/metrics-ca-bundle.crt
      

8. ns/openshift-kube-apiserver configmap/etcd-serving-ca

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate clients and peers of etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/server-ca-bundle.crt
      

9. ns/openshift-oauth-apiserver configmap/etcd-serving-ca

      **Description:** Generated by cluster-etcd-operator for etcd and is used to authenticate clients and peers of etcd.
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/etcd-certs/configmaps/etcd-all-bundles/server-ca-bundle.crt
      



## kube-apiserver (46)
### Certificates (25)
1. ns/openshift-config-managed secret/kube-controller-manager-client-cert-key

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/secrets/kube-controller-manager-client-cert-key/tls.crt
      

2. ns/openshift-config-managed secret/kube-scheduler-client-cert-key

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-scheduler-certs/secrets/kube-scheduler-client-cert-key/tls.crt
      

3. ns/openshift-kube-apiserver secret/aggregator-client

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/aggregator-client/tls.crt
      

4. ns/openshift-kube-apiserver secret/check-endpoints-client-cert-key

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/check-endpoints-client-cert-key/tls.crt
      

5. ns/openshift-kube-apiserver secret/control-plane-node-admin-client-cert-key

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/control-plane-node-admin-client-cert-key/tls.crt
      

6. ns/openshift-kube-apiserver secret/external-loadbalancer-serving-certkey

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/external-loadbalancer-serving-certkey/tls.crt
      

7. ns/openshift-kube-apiserver secret/internal-loadbalancer-serving-certkey

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/internal-loadbalancer-serving-certkey/tls.crt
      

8. ns/openshift-kube-apiserver secret/kubelet-client

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/kubelet-client/tls.crt
      

9. ns/openshift-kube-apiserver secret/localhost-recovery-serving-certkey

      **Description:** 
      

10. ns/openshift-kube-apiserver secret/localhost-serving-cert-certkey

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/localhost-serving-cert-certkey/tls.crt
      

11. ns/openshift-kube-apiserver secret/node-kubeconfigs

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-int.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost-recovery.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig
      

12. ns/openshift-kube-apiserver secret/service-network-serving-certkey

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/service-network-serving-certkey/tls.crt
      

13. ns/openshift-kube-apiserver-operator secret/aggregator-client-signer

      **Description:** 
      

14. ns/openshift-kube-apiserver-operator secret/kube-apiserver-to-kubelet-signer

      **Description:** 
      

15. ns/openshift-kube-apiserver-operator secret/kube-control-plane-signer

      **Description:** 
      

16. ns/openshift-kube-apiserver-operator secret/loadbalancer-serving-signer

      **Description:** 
      

17. ns/openshift-kube-apiserver-operator secret/localhost-recovery-serving-signer

      **Description:** 
      

18. ns/openshift-kube-apiserver-operator secret/localhost-serving-signer

      **Description:** 
      

19. ns/openshift-kube-apiserver-operator secret/node-system-admin-client

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-int.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost-recovery.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig
      

20. ns/openshift-kube-apiserver-operator secret/node-system-admin-signer

      **Description:** 
      

21. ns/openshift-kube-apiserver-operator secret/service-network-serving-signer

      **Description:** 
      

22. ns/openshift-kube-controller-manager secret/kube-controller-manager-client-cert-key

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/secrets/kube-controller-manager-client-cert-key/tls.crt
      

23. ns/openshift-kube-scheduler secret/kube-scheduler-client-cert-key

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-scheduler-certs/secrets/kube-scheduler-client-cert-key/tls.crt
      

24. file /etc/kubernetes/kubeconfig

      **Description:** 
      

25. file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/bound-service-account-signing-key/service-account.key

      **Description:** 
      



### Certificate Authority Bundles (21)
1. ns/openshift-config configmap/admin-kubeconfig-client-ca

      **Description:** 
      

2. ns/openshift-config-managed configmap/kube-apiserver-aggregator-client-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/aggregator-client-ca/ca-bundle.crt
      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/aggregator-client-ca/ca-bundle.crt
      

3. ns/openshift-config-managed configmap/kube-apiserver-client-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/kubelet-ca.crt
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/client-ca/ca-bundle.crt
      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/client-ca/ca-bundle.crt
      

4. ns/openshift-config-managed configmap/kube-apiserver-server-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-int.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost-recovery.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig
      

5. ns/openshift-config-managed configmap/kubelet-bootstrap-kubeconfig

      **Description:** 
      

6. ns/openshift-controller-manager configmap/client-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/kubelet-ca.crt
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/client-ca/ca-bundle.crt
      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/client-ca/ca-bundle.crt
      

7. ns/openshift-kube-apiserver configmap/aggregator-client-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/aggregator-client-ca/ca-bundle.crt
      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/aggregator-client-ca/ca-bundle.crt
      

8. ns/openshift-kube-apiserver configmap/client-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/kubelet-ca.crt
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/client-ca/ca-bundle.crt
      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/client-ca/ca-bundle.crt
      

9. ns/openshift-kube-apiserver configmap/kube-apiserver-server-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-int.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost-recovery.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig
      

10. ns/openshift-kube-apiserver-operator configmap/kube-apiserver-to-kubelet-client-ca

      **Description:** 
      

11. ns/openshift-kube-apiserver-operator configmap/kube-control-plane-signer-ca

      **Description:** 
      

12. ns/openshift-kube-apiserver-operator configmap/loadbalancer-serving-ca

      **Description:** 
      

13. ns/openshift-kube-apiserver-operator configmap/localhost-recovery-serving-ca

      **Description:** 
      

14. ns/openshift-kube-apiserver-operator configmap/localhost-serving-ca

      **Description:** 
      

15. ns/openshift-kube-apiserver-operator configmap/node-system-admin-ca

      **Description:** 
      

16. ns/openshift-kube-apiserver-operator configmap/service-network-serving-ca

      **Description:** 
      

17. ns/openshift-kube-controller-manager configmap/aggregator-client-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/aggregator-client-ca/ca-bundle.crt
      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/aggregator-client-ca/ca-bundle.crt
      

18. ns/openshift-kube-controller-manager configmap/client-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/kubelet-ca.crt
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/client-ca/ca-bundle.crt
      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/client-ca/ca-bundle.crt
      

19. ns/openshift-route-controller-manager configmap/client-ca

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/kubelet-ca.crt
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/client-ca/ca-bundle.crt
      * file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/client-ca/ca-bundle.crt
      

20. file /etc/kubernetes/kubeconfig

      **Description:** 
      

      Other locations:

      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-int.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost-recovery.kubeconfig
      * file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig
      

21. file /etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/trusted-ca-bundle/ca-bundle.crt

      **Description:** 
      



## kube-controller-manager (12)
### Certificates (3)
1. ns/openshift-kube-controller-manager secret/csr-signer

      **Description:** 
      

2. ns/openshift-kube-controller-manager-operator secret/csr-signer

      **Description:** 
      

3. ns/openshift-kube-controller-manager-operator secret/csr-signer-signer

      **Description:** 
      



### Certificate Authority Bundles (9)
1. ns/openshift-config-managed configmap/csr-controller-ca

      **Description:** 
      

2. ns/openshift-config-managed configmap/kubelet-serving-ca

      **Description:** 
      

3. ns/openshift-kube-apiserver configmap/kubelet-serving-ca

      **Description:** 
      

4. ns/openshift-kube-controller-manager configmap/serviceaccount-ca

      **Description:** 
      

5. ns/openshift-kube-controller-manager-operator configmap/csr-controller-ca

      **Description:** 
      

6. ns/openshift-kube-controller-manager-operator configmap/csr-controller-signer-ca

      **Description:** 
      

7. ns/openshift-kube-controller-manager-operator configmap/csr-signer-ca

      **Description:** 
      

8. file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/trusted-ca-bundle/ca-bundle.crt

      **Description:** 
      

9. file /etc/kubernetes/static-pod-resources/kube-controller-manager-certs/secrets/csr-signer/tls.crt

      **Description:** 
      



## kube-scheduler (1)
### Certificate Authority Bundles (1)
1. ns/openshift-kube-scheduler configmap/serviceaccount-ca

      **Description:** 
      



## service-ca (100)
### Certificates (97)
1. ns/openshift-apiserver secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/api with hostname api.openshift-apiserver.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

2. ns/openshift-apiserver-operator secret/openshift-apiserver-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-apiserver-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-apiserver-operator-serving-cert'. The certificate is valid for 2 years.
      

3. ns/openshift-authentication secret/v4-0-config-system-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/oauth-openshift with hostname oauth-openshift.openshift-authentication.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: v4-0-config-system-serving-cert'. The certificate is valid for 2 years.
      

4. ns/openshift-authentication-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-authentication-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

5. ns/openshift-catalogd secret/catalogserver-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/catalogd-catalogserver with hostname catalogd-catalogserver.openshift-catalogd.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: catalogserver-cert'. The certificate is valid for 2 years.
      

6. ns/openshift-cloud-controller-manager-operator secret/cloud-controller-manager-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cloud-controller-manager-operator with hostname cloud-controller-manager-operator.openshift-cloud-controller-manager-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cloud-controller-manager-operator-tls'. The certificate is valid for 2 years.
      

7. ns/openshift-cloud-credential-operator secret/cloud-credential-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cco-metrics with hostname cco-metrics.openshift-cloud-credential-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cloud-credential-operator-serving-cert'. The certificate is valid for 2 years.
      

8. ns/openshift-cloud-credential-operator secret/pod-identity-webhook

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/pod-identity-webhook with hostname pod-identity-webhook.openshift-cloud-credential-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: pod-identity-webhook'. The certificate is valid for 2 years.
      

9. ns/openshift-cluster-api secret/capa-webhook-service-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/capa-webhook-service with hostname capa-webhook-service.openshift-cluster-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: capa-webhook-service-cert'. The certificate is valid for 2 years.
      

10. ns/openshift-cluster-api secret/capg-webhook-service-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/capg-webhook-service with hostname capg-webhook-service.openshift-cluster-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: capg-webhook-service-cert'. The certificate is valid for 2 years.
      

11. ns/openshift-cluster-api secret/capi-webhook-service-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/capi-webhook-service with hostname capi-webhook-service.openshift-cluster-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: capi-webhook-service-cert'. The certificate is valid for 2 years.
      

12. ns/openshift-cluster-api secret/capv-webhook-service-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/capv-webhook-service with hostname capv-webhook-service.openshift-cluster-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: capv-webhook-service-cert'. The certificate is valid for 2 years.
      

13. ns/openshift-cluster-api secret/capz-webhook-service-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/capz-webhook-service with hostname capz-webhook-service.openshift-cluster-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: capz-webhook-service-cert'. The certificate is valid for 2 years.
      

14. ns/openshift-cluster-api secret/cluster-capi-operator-webhook-service-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-capi-operator-webhook-service with hostname cluster-capi-operator-webhook-service.openshift-cluster-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-capi-operator-webhook-service-cert'. The certificate is valid for 2 years.
      

15. ns/openshift-cluster-csi-drivers secret/aws-ebs-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/aws-ebs-csi-driver-controller-metrics with hostname aws-ebs-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: aws-ebs-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

16. ns/openshift-cluster-csi-drivers secret/azure-disk-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/azure-disk-csi-driver-controller-metrics with hostname azure-disk-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: azure-disk-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

17. ns/openshift-cluster-csi-drivers secret/azure-disk-csi-driver-node-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/azure-disk-csi-driver-node-metrics with hostname azure-disk-csi-driver-node-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: azure-disk-csi-driver-node-metrics-serving-cert'. The certificate is valid for 2 years.
      

18. ns/openshift-cluster-csi-drivers secret/azure-file-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/azure-file-csi-driver-controller-metrics with hostname azure-file-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: azure-file-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

19. ns/openshift-cluster-csi-drivers secret/gcp-pd-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/gcp-pd-csi-driver-controller-metrics with hostname gcp-pd-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: gcp-pd-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

20. ns/openshift-cluster-csi-drivers secret/openstack-cinder-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/openstack-cinder-csi-driver-controller-metrics with hostname openstack-cinder-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openstack-cinder-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

21. ns/openshift-cluster-csi-drivers secret/shared-resource-csi-driver-node-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/shared-resource-csi-driver-node-metrics with hostname shared-resource-csi-driver-node-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: shared-resource-csi-driver-node-metrics-serving-cert'. The certificate is valid for 2 years.
      

22. ns/openshift-cluster-csi-drivers secret/shared-resource-csi-driver-operator-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/shared-resource-csi-driver-operator-metrics with hostname shared-resource-csi-driver-operator-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: shared-resource-csi-driver-operator-metrics-serving-cert'. The certificate is valid for 2 years.
      

23. ns/openshift-cluster-csi-drivers secret/shared-resource-csi-driver-webhook-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/shared-resource-csi-driver-webhook with hostname shared-resource-csi-driver-webhook.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: shared-resource-csi-driver-webhook-serving-cert'. The certificate is valid for 2 years.
      

24. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/vmware-vsphere-csi-driver-controller-metrics with hostname vmware-vsphere-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: vmware-vsphere-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

25. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-operator-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/vmware-vsphere-csi-driver-operator-metrics with hostname vmware-vsphere-csi-driver-operator-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: vmware-vsphere-csi-driver-operator-metrics-serving-cert'. The certificate is valid for 2 years.
      

26. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-webhook-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/vmware-vsphere-csi-driver-webhook-svc with hostname vmware-vsphere-csi-driver-webhook-svc.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: vmware-vsphere-csi-driver-webhook-secret'. The certificate is valid for 2 years.
      

27. ns/openshift-cluster-machine-approver secret/machine-approver-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-approver with hostname machine-approver.openshift-cluster-machine-approver.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-approver-tls'. The certificate is valid for 2 years.
      

28. ns/openshift-cluster-node-tuning-operator secret/node-tuning-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/node-tuning-operator with hostname node-tuning-operator.openshift-cluster-node-tuning-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: node-tuning-operator-tls'. The certificate is valid for 2 years.
      

29. ns/openshift-cluster-node-tuning-operator secret/performance-addon-operator-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/performance-addon-operator-service with hostname performance-addon-operator-service.openshift-cluster-node-tuning-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: performance-addon-operator-webhook-cert'. The certificate is valid for 2 years.
      

30. ns/openshift-cluster-olm-operator secret/cluster-olm-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-olm-operator-metrics with hostname cluster-olm-operator-metrics.openshift-cluster-olm-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-olm-operator-serving-cert'. The certificate is valid for 2 years.
      

31. ns/openshift-cluster-samples-operator secret/samples-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-cluster-samples-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: samples-operator-tls'. The certificate is valid for 2 years.
      

32. ns/openshift-cluster-storage-operator secret/cluster-storage-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-storage-operator-metrics with hostname cluster-storage-operator-metrics.openshift-cluster-storage-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-storage-operator-serving-cert'. The certificate is valid for 2 years.
      

33. ns/openshift-cluster-storage-operator secret/csi-snapshot-webhook-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/csi-snapshot-webhook with hostname csi-snapshot-webhook.openshift-cluster-storage-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: csi-snapshot-webhook-secret'. The certificate is valid for 2 years.
      

34. ns/openshift-cluster-storage-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/csi-snapshot-controller-operator-metrics with hostname csi-snapshot-controller-operator-metrics.openshift-cluster-storage-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

35. ns/openshift-cluster-storage-operator secret/vsphere-problem-detector-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/vsphere-problem-detector-metrics with hostname vsphere-problem-detector-metrics.openshift-cluster-storage-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: vsphere-problem-detector-serving-cert'. The certificate is valid for 2 years.
      

36. ns/openshift-cluster-version secret/cluster-version-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-version-operator with hostname cluster-version-operator.openshift-cluster-version.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-version-operator-serving-cert'. The certificate is valid for 2 years.
      

37. ns/openshift-config-operator secret/config-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-config-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: config-operator-serving-cert'. The certificate is valid for 2 years.
      

38. ns/openshift-console secret/console-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/console with hostname console.openshift-console.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: console-serving-cert'. The certificate is valid for 2 years.
      

39. ns/openshift-console-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-console-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

40. ns/openshift-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/controller-manager with hostname controller-manager.openshift-controller-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

41. ns/openshift-controller-manager-operator secret/openshift-controller-manager-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-controller-manager-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-controller-manager-operator-serving-cert'. The certificate is valid for 2 years.
      

42. ns/openshift-dns secret/dns-default-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/dns-default with hostname dns-default.openshift-dns.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: dns-default-metrics-tls'. The certificate is valid for 2 years.
      

43. ns/openshift-dns-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-dns-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

44. ns/openshift-e2e-loki secret/proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/promtail with hostname promtail.openshift-e2e-loki.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: proxy-tls'. The certificate is valid for 2 years.
      

45. ns/openshift-etcd secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/etcd with hostname etcd.openshift-etcd.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

46. ns/openshift-etcd-operator secret/etcd-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-etcd-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: etcd-operator-serving-cert'. The certificate is valid for 2 years.
      

47. ns/openshift-image-registry secret/image-registry-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/image-registry-operator with hostname image-registry-operator.openshift-image-registry.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: image-registry-operator-tls'. The certificate is valid for 2 years.
      

48. ns/openshift-image-registry secret/image-registry-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/image-registry with hostname image-registry.openshift-image-registry.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: image-registry-tls'. The certificate is valid for 2 years.
      

49. ns/openshift-ingress secret/router-metrics-certs-default

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/router-internal-default with hostname router-internal-default.openshift-ingress.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: router-metrics-certs-default'. The certificate is valid for 2 years.
      

50. ns/openshift-ingress-canary secret/canary-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/ingress-canary with hostname ingress-canary.openshift-ingress-canary.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: canary-serving-cert'. The certificate is valid for 2 years.
      

51. ns/openshift-ingress-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-ingress-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

52. ns/openshift-insights secret/openshift-insights-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-insights.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-insights-serving-cert'. The certificate is valid for 2 years.
      

53. ns/openshift-kube-apiserver-operator secret/kube-apiserver-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-kube-apiserver-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-apiserver-operator-serving-cert'. The certificate is valid for 2 years.
      

54. ns/openshift-kube-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/kube-controller-manager with hostname kube-controller-manager.openshift-kube-controller-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

55. ns/openshift-kube-controller-manager-operator secret/kube-controller-manager-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-kube-controller-manager-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-controller-manager-operator-serving-cert'. The certificate is valid for 2 years.
      

56. ns/openshift-kube-scheduler secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/scheduler with hostname scheduler.openshift-kube-scheduler.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

57. ns/openshift-kube-scheduler-operator secret/kube-scheduler-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-kube-scheduler-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-scheduler-operator-serving-cert'. The certificate is valid for 2 years.
      

58. ns/openshift-kube-storage-version-migrator-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-kube-storage-version-migrator-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

59. ns/openshift-machine-api secret/baremetal-operator-webhook-server-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/baremetal-operator-webhook-service with hostname baremetal-operator-webhook-service.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: baremetal-operator-webhook-server-cert'. The certificate is valid for 2 years.
      

60. ns/openshift-machine-api secret/cluster-autoscaler-operator-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-autoscaler-operator with hostname cluster-autoscaler-operator.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-autoscaler-operator-cert'. The certificate is valid for 2 years.
      

61. ns/openshift-machine-api secret/cluster-baremetal-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-baremetal-operator-service with hostname cluster-baremetal-operator-service.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-baremetal-operator-tls'. The certificate is valid for 2 years.
      

62. ns/openshift-machine-api secret/cluster-baremetal-webhook-server-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-baremetal-webhook-service with hostname cluster-baremetal-webhook-service.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-baremetal-webhook-server-cert'. The certificate is valid for 2 years.
      

63. ns/openshift-machine-api secret/control-plane-machine-set-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/control-plane-machine-set-operator with hostname control-plane-machine-set-operator.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: control-plane-machine-set-operator-tls'. The certificate is valid for 2 years.
      

64. ns/openshift-machine-api secret/machine-api-controllers-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-api-controllers with hostname machine-api-controllers.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-controllers-tls'. The certificate is valid for 2 years.
      

65. ns/openshift-machine-api secret/machine-api-operator-machine-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-api-operator-machine-webhook with hostname machine-api-operator-machine-webhook.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-machine-webhook-cert'. The certificate is valid for 2 years.
      

66. ns/openshift-machine-api secret/machine-api-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-api-operator with hostname machine-api-operator.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-tls'. The certificate is valid for 2 years.
      

67. ns/openshift-machine-api secret/machine-api-operator-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-api-operator-webhook with hostname machine-api-operator-webhook.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-webhook-cert'. The certificate is valid for 2 years.
      

68. ns/openshift-machine-config-operator secret/mcc-proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-config-controller with hostname machine-config-controller.openshift-machine-config-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: mcc-proxy-tls'. The certificate is valid for 2 years.
      

69. ns/openshift-machine-config-operator secret/mco-proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-config-operator with hostname machine-config-operator.openshift-machine-config-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: mco-proxy-tls'. The certificate is valid for 2 years.
      

70. ns/openshift-machine-config-operator secret/proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-config-daemon with hostname machine-config-daemon.openshift-machine-config-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: proxy-tls'. The certificate is valid for 2 years.
      

71. ns/openshift-marketplace secret/marketplace-operator-metrics

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/marketplace-operator-metrics with hostname marketplace-operator-metrics.openshift-marketplace.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: marketplace-operator-metrics'. The certificate is valid for 2 years.
      

72. ns/openshift-monitoring secret/alertmanager-main-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/alertmanager-main with hostname alertmanager-main.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: alertmanager-main-tls'. The certificate is valid for 2 years.
      

73. ns/openshift-monitoring secret/cluster-monitoring-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-monitoring-operator with hostname cluster-monitoring-operator.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-monitoring-operator-tls'. The certificate is valid for 2 years.
      

74. ns/openshift-monitoring secret/kube-state-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/kube-state-metrics with hostname kube-state-metrics.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-state-metrics-tls'. The certificate is valid for 2 years.
      

75. ns/openshift-monitoring secret/metrics-server-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics-server with hostname metrics-server.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-server-tls'. The certificate is valid for 2 years.
      

76. ns/openshift-monitoring secret/monitoring-plugin-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/monitoring-plugin with hostname monitoring-plugin.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: monitoring-plugin-cert'. The certificate is valid for 2 years.
      

77. ns/openshift-monitoring secret/node-exporter-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/node-exporter with hostname node-exporter.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: node-exporter-tls'. The certificate is valid for 2 years.
      

78. ns/openshift-monitoring secret/openshift-state-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/openshift-state-metrics with hostname openshift-state-metrics.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-state-metrics-tls'. The certificate is valid for 2 years.
      

79. ns/openshift-monitoring secret/prometheus-k8s-thanos-sidecar-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-k8s-thanos-sidecar with hostname prometheus-k8s-thanos-sidecar.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-k8s-thanos-sidecar-tls'. The certificate is valid for 2 years.
      

80. ns/openshift-monitoring secret/prometheus-k8s-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-k8s with hostname prometheus-k8s.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-k8s-tls'. The certificate is valid for 2 years.
      

81. ns/openshift-monitoring secret/prometheus-operator-admission-webhook-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-operator-admission-webhook with hostname prometheus-operator-admission-webhook.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-operator-admission-webhook-tls'. The certificate is valid for 2 years.
      

82. ns/openshift-monitoring secret/prometheus-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-operator with hostname prometheus-operator.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-operator-tls'. The certificate is valid for 2 years.
      

83. ns/openshift-monitoring secret/telemeter-client-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/telemeter-client with hostname telemeter-client.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: telemeter-client-tls'. The certificate is valid for 2 years.
      

84. ns/openshift-monitoring secret/thanos-querier-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/thanos-querier with hostname thanos-querier.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: thanos-querier-tls'. The certificate is valid for 2 years.
      

85. ns/openshift-multus secret/metrics-daemon-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/network-metrics-service with hostname network-metrics-service.openshift-multus.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-daemon-secret'. The certificate is valid for 2 years.
      

86. ns/openshift-multus secret/multus-admission-controller-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/multus-admission-controller with hostname multus-admission-controller.openshift-multus.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: multus-admission-controller-secret'. The certificate is valid for 2 years.
      

87. ns/openshift-network-console secret/networking-console-plugin-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/networking-console-plugin with hostname networking-console-plugin.openshift-network-console.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: networking-console-plugin-cert'. The certificate is valid for 2 years.
      

88. ns/openshift-network-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-network-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

89. ns/openshift-oauth-apiserver secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/api with hostname api.openshift-oauth-apiserver.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

90. ns/openshift-operator-lifecycle-manager secret/catalog-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/catalog-operator-metrics with hostname catalog-operator-metrics.openshift-operator-lifecycle-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: catalog-operator-serving-cert'. The certificate is valid for 2 years.
      

91. ns/openshift-operator-lifecycle-manager secret/olm-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/olm-operator-metrics with hostname olm-operator-metrics.openshift-operator-lifecycle-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: olm-operator-serving-cert'. The certificate is valid for 2 years.
      

92. ns/openshift-operator-lifecycle-manager secret/package-server-manager-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/package-server-manager-metrics with hostname package-server-manager-metrics.openshift-operator-lifecycle-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: package-server-manager-serving-cert'. The certificate is valid for 2 years.
      

93. ns/openshift-ovn-kubernetes secret/ovn-control-plane-metrics-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/ovn-kubernetes-control-plane with hostname ovn-kubernetes-control-plane.openshift-ovn-kubernetes.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: ovn-control-plane-metrics-cert'. The certificate is valid for 2 years.
      

94. ns/openshift-ovn-kubernetes secret/ovn-node-metrics-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/ovn-kubernetes-node with hostname ovn-kubernetes-node.openshift-ovn-kubernetes.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: ovn-node-metrics-cert'. The certificate is valid for 2 years.
      

95. ns/openshift-route-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/route-controller-manager with hostname route-controller-manager.openshift-route-controller-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

96. ns/openshift-service-ca secret/signing-key

      **Description:** Service CA secret contains a signing key that will be used to issue a signed serving certificate/key pair to services annotated with 'service.beta.openshift.io/serving-cert-secret-name'
      

97. ns/openshift-service-ca-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-service-ca-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      



### Certificate Authority Bundles (3)
1. ns/openshift-config-managed configmap/service-ca

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      

2. ns/openshift-kube-controller-manager configmap/service-ca

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      

3. ns/openshift-service-ca configmap/signing-cabundle

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      



