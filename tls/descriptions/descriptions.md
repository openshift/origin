# Description of TLS Artifacts

## Table of Contents
  - [How to meet the requirement](#How-to-meet-the-requirement)
  - [Items Do NOT Meet the Requirement (139)](#Items-Do-NOT-Meet-the-Requirement-139)
    - [ (20)](#-20)
      - [Certificates (9)](#Certificates-9)
      - [Certificate Authority Bundles (11)](#Certificate-Authority-Bundles-11)
    - [Cloud Compute / Cloud Controller Manager (1)](#Cloud-Compute-/-Cloud-Controller-Manager-1)
      - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
    - [End User (1)](#End-User-1)
      - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
    - [Etcd (28)](#Etcd-28)
      - [Certificates (19)](#Certificates-19)
      - [Certificate Authority Bundles (9)](#Certificate-Authority-Bundles-9)
    - [Machine Config Operator (2)](#Machine-Config-Operator-2)
      - [Certificates (1)](#Certificates-1)
      - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
    - [Monitoring (7)](#Monitoring-7)
      - [Certificates (2)](#Certificates-2)
      - [Certificate Authority Bundles (5)](#Certificate-Authority-Bundles-5)
    - [Networking / cluster-network-operator (25)](#Networking-/-cluster-network-operator-25)
      - [Certificate Authority Bundles (25)](#Certificate-Authority-Bundles-25)
    - [Operator Framework / operator-lifecycle-manager (2)](#Operator-Framework-/-operator-lifecycle-manager-2)
      - [Certificates (2)](#Certificates-2)
    - [apiserver-auth (3)](#apiserver-auth-3)
      - [Certificates (1)](#Certificates-1)
      - [Certificate Authority Bundles (2)](#Certificate-Authority-Bundles-2)
    - [kube-apiserver (39)](#kube-apiserver-39)
      - [Certificates (22)](#Certificates-22)
      - [Certificate Authority Bundles (17)](#Certificate-Authority-Bundles-17)
    - [kube-controller-manager (10)](#kube-controller-manager-10)
      - [Certificates (3)](#Certificates-3)
      - [Certificate Authority Bundles (7)](#Certificate-Authority-Bundles-7)
    - [kube-scheduler (1)](#kube-scheduler-1)
      - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
  - [Items That DO Meet the Requirement (88)](#Items-That-DO-Meet-the-Requirement-88)
    - [service-ca (88)](#service-ca-88)
      - [Certificates (85)](#Certificates-85)
      - [Certificate Authority Bundles (3)](#Certificate-Authority-Bundles-3)


## How to meet the requirement
TLS artifacts must have user-facing descriptions on their in-cluster resources.
These descriptions must be in the style of API documentation and must include
1. Which connections a CA bundle can be used to verify.
2. What kind of certificates a signer will sign for.
3. Which names and IPs a serving certificate terminates.
4. Which subject (user and group) a client certificate is created for.
5. Which binary and flags is this certificate wired to.

To create a description, set the `openshift.io/description` annotation to the markdown formatted string describing your TLS artifact. 

## Items Do NOT Meet the Requirement (139)
###  (20)
#### Certificates (9)
1. ns/openshift-ingress secret/router-certs-default

      **Description:** 
      

2. ns/openshift-ingress-operator secret/router-ca

      **Description:** 
      

3. ns/openshift-machine-api secret/metal3-ironic-tls

      **Description:** 
      

4. ns/openshift-network-node-identity secret/network-node-identity-ca

      **Description:** 
      

5. ns/openshift-network-node-identity secret/network-node-identity-cert

      **Description:** 
      

6. ns/openshift-ovn-kubernetes secret/ovn-ca

      **Description:** 
      

7. ns/openshift-ovn-kubernetes secret/ovn-cert

      **Description:** 
      

8. ns/openshift-ovn-kubernetes secret/signer-ca

      **Description:** 
      

9. ns/openshift-ovn-kubernetes secret/signer-cert

      **Description:** 
      



#### Certificate Authority Bundles (11)
1. ns/openshift-config configmap/admin-kubeconfig-client-ca

      **Description:** 
      

2. ns/openshift-config configmap/etcd-ca-bundle

      **Description:** 
      

3. ns/openshift-config-managed configmap/default-ingress-cert

      **Description:** 
      

4. ns/openshift-config-managed configmap/kubelet-bootstrap-kubeconfig

      **Description:** 
      

5. ns/openshift-console configmap/default-ingress-cert

      **Description:** 
      

6. ns/openshift-etcd configmap/etcd-ca-bundle

      **Description:** 
      

7. ns/openshift-etcd configmap/etcd-peer-client-ca

      **Description:** 
      

8. ns/openshift-etcd-operator configmap/etcd-ca-bundle

      **Description:** 
      

9. ns/openshift-network-node-identity configmap/network-node-identity-ca

      **Description:** 
      

10. ns/openshift-ovn-kubernetes configmap/ovn-ca

      **Description:** 
      

11. ns/openshift-ovn-kubernetes configmap/signer-ca

      **Description:** 
      



### Cloud Compute / Cloud Controller Manager (1)
#### Certificate Authority Bundles (1)
1. ns/openshift-cloud-controller-manager configmap/ccm-trusted-ca

      **Description:** 
      



### End User (1)
#### Certificate Authority Bundles (1)
1. ns/openshift-config configmap/user-ca-bundle

      **Description:** 
      



### Etcd (28)
#### Certificates (19)
1. ns/openshift-apiserver secret/etcd-client

      **Description:** 
      

2. ns/openshift-config secret/etcd-client

      **Description:** 
      

3. ns/openshift-config secret/etcd-metric-client

      **Description:** 
      

4. ns/openshift-config secret/etcd-metric-signer

      **Description:** 
      

5. ns/openshift-config secret/etcd-signer

      **Description:** 
      

6. ns/openshift-etcd secret/etcd-client

      **Description:** 
      

7. ns/openshift-etcd secret/etcd-peer-\<master-0>

      **Description:** 
      

8. ns/openshift-etcd secret/etcd-peer-\<master-1>

      **Description:** 
      

9. ns/openshift-etcd secret/etcd-peer-\<master-2>

      **Description:** 
      

10. ns/openshift-etcd secret/etcd-serving-\<master-0>

      **Description:** 
      

11. ns/openshift-etcd secret/etcd-serving-\<master-1>

      **Description:** 
      

12. ns/openshift-etcd secret/etcd-serving-\<master-2>

      **Description:** 
      

13. ns/openshift-etcd secret/etcd-serving-metrics-\<master-0>

      **Description:** 
      

14. ns/openshift-etcd secret/etcd-serving-metrics-\<master-1>

      **Description:** 
      

15. ns/openshift-etcd secret/etcd-serving-metrics-\<master-2>

      **Description:** 
      

16. ns/openshift-etcd-operator secret/etcd-client

      **Description:** 
      

17. ns/openshift-etcd-operator secret/etcd-metric-client

      **Description:** 
      

18. ns/openshift-kube-apiserver secret/etcd-client

      **Description:** 
      

19. ns/openshift-oauth-apiserver secret/etcd-client

      **Description:** 
      



#### Certificate Authority Bundles (9)
1. ns/openshift-apiserver configmap/etcd-serving-ca

      **Description:** 
      

2. ns/openshift-config configmap/etcd-metric-serving-ca

      **Description:** 
      

3. ns/openshift-config configmap/etcd-serving-ca

      **Description:** 
      

4. ns/openshift-etcd configmap/etcd-metrics-proxy-client-ca

      **Description:** 
      

5. ns/openshift-etcd configmap/etcd-metrics-proxy-serving-ca

      **Description:** 
      

6. ns/openshift-etcd configmap/etcd-serving-ca

      **Description:** 
      

7. ns/openshift-etcd-operator configmap/etcd-metric-serving-ca

      **Description:** 
      

8. ns/openshift-kube-apiserver configmap/etcd-serving-ca

      **Description:** 
      

9. ns/openshift-oauth-apiserver configmap/etcd-serving-ca

      **Description:** 
      



### Machine Config Operator (2)
#### Certificates (1)
1. ns/openshift-machine-config-operator secret/machine-config-server-tls

      **Description:** 
      



#### Certificate Authority Bundles (1)
1. ns/openshift-config configmap/initial-kube-apiserver-server-ca

      **Description:** 
      



### Monitoring (7)
#### Certificates (2)
1. ns/openshift-monitoring secret/federate-client-certs

      **Description:** 
      

2. ns/openshift-monitoring secret/metrics-client-certs

      **Description:** 
      



#### Certificate Authority Bundles (5)
1. ns/openshift-monitoring configmap/alertmanager-trusted-ca-bundle

      **Description:** 
      

2. ns/openshift-monitoring configmap/kubelet-serving-ca-bundle

      **Description:** 
      

3. ns/openshift-monitoring configmap/prometheus-trusted-ca-bundle

      **Description:** 
      

4. ns/openshift-monitoring configmap/telemeter-trusted-ca-bundle

      **Description:** 
      

5. ns/openshift-monitoring configmap/thanos-querier-trusted-ca-bundle

      **Description:** 
      



### Networking / cluster-network-operator (25)
#### Certificate Authority Bundles (25)
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
      

11. ns/openshift-cluster-csi-drivers configmap/vmware-vsphere-csi-driver-trusted-ca-bundle

      **Description:** 
      

12. ns/openshift-cluster-csi-drivers configmap/vsphere-csi-driver-operator-trusted-ca-bundle

      **Description:** 
      

13. ns/openshift-cluster-node-tuning-operator configmap/trusted-ca

      **Description:** 
      

14. ns/openshift-cluster-storage-operator configmap/trusted-ca-bundle

      **Description:** 
      

15. ns/openshift-config-managed configmap/trusted-ca-bundle

      **Description:** 
      

16. ns/openshift-console configmap/trusted-ca-bundle

      **Description:** 
      

17. ns/openshift-controller-manager configmap/openshift-global-ca

      **Description:** 
      

18. ns/openshift-image-registry configmap/trusted-ca

      **Description:** 
      

19. ns/openshift-ingress-operator configmap/trusted-ca

      **Description:** 
      

20. ns/openshift-insights configmap/trusted-ca-bundle

      **Description:** 
      

21. ns/openshift-kube-apiserver configmap/trusted-ca-bundle

      **Description:** 
      

22. ns/openshift-kube-controller-manager configmap/trusted-ca-bundle

      **Description:** 
      

23. ns/openshift-machine-api configmap/cbo-trusted-ca

      **Description:** 
      

24. ns/openshift-machine-api configmap/mao-trusted-ca

      **Description:** 
      

25. ns/openshift-marketplace configmap/marketplace-trusted-ca

      **Description:** 
      



### Operator Framework / operator-lifecycle-manager (2)
#### Certificates (2)
1. ns/openshift-operator-lifecycle-manager secret/packageserver-service-cert

      **Description:** 
      

2. ns/openshift-operator-lifecycle-manager secret/pprof-cert

      **Description:** 
      



### apiserver-auth (3)
#### Certificates (1)
1. ns/openshift-oauth-apiserver secret/openshift-authenticator-certs

      **Description:** 
      



#### Certificate Authority Bundles (2)
1. ns/openshift-config-managed configmap/oauth-serving-cert

      **Description:** 
      

2. ns/openshift-console configmap/oauth-serving-cert

      **Description:** 
      



### kube-apiserver (39)
#### Certificates (22)
1. ns/openshift-config-managed secret/kube-controller-manager-client-cert-key

      **Description:** 
      

2. ns/openshift-config-managed secret/kube-scheduler-client-cert-key

      **Description:** 
      

3. ns/openshift-kube-apiserver secret/aggregator-client

      **Description:** 
      

4. ns/openshift-kube-apiserver secret/check-endpoints-client-cert-key

      **Description:** 
      

5. ns/openshift-kube-apiserver secret/control-plane-node-admin-client-cert-key

      **Description:** 
      

6. ns/openshift-kube-apiserver secret/external-loadbalancer-serving-certkey

      **Description:** 
      

7. ns/openshift-kube-apiserver secret/internal-loadbalancer-serving-certkey

      **Description:** 
      

8. ns/openshift-kube-apiserver secret/kubelet-client

      **Description:** 
      

9. ns/openshift-kube-apiserver secret/localhost-recovery-serving-certkey

      **Description:** 
      

10. ns/openshift-kube-apiserver secret/localhost-serving-cert-certkey

      **Description:** 
      

11. ns/openshift-kube-apiserver secret/service-network-serving-certkey

      **Description:** 
      

12. ns/openshift-kube-apiserver-operator secret/aggregator-client-signer

      **Description:** 
      

13. ns/openshift-kube-apiserver-operator secret/kube-apiserver-to-kubelet-signer

      **Description:** 
      

14. ns/openshift-kube-apiserver-operator secret/kube-control-plane-signer

      **Description:** 
      

15. ns/openshift-kube-apiserver-operator secret/loadbalancer-serving-signer

      **Description:** 
      

16. ns/openshift-kube-apiserver-operator secret/localhost-recovery-serving-signer

      **Description:** 
      

17. ns/openshift-kube-apiserver-operator secret/localhost-serving-signer

      **Description:** 
      

18. ns/openshift-kube-apiserver-operator secret/node-system-admin-client

      **Description:** 
      

19. ns/openshift-kube-apiserver-operator secret/node-system-admin-signer

      **Description:** 
      

20. ns/openshift-kube-apiserver-operator secret/service-network-serving-signer

      **Description:** 
      

21. ns/openshift-kube-controller-manager secret/kube-controller-manager-client-cert-key

      **Description:** 
      

22. ns/openshift-kube-scheduler secret/kube-scheduler-client-cert-key

      **Description:** 
      



#### Certificate Authority Bundles (17)
1. ns/openshift-config-managed configmap/kube-apiserver-aggregator-client-ca

      **Description:** 
      

2. ns/openshift-config-managed configmap/kube-apiserver-client-ca

      **Description:** 
      

3. ns/openshift-config-managed configmap/kube-apiserver-server-ca

      **Description:** 
      

4. ns/openshift-controller-manager configmap/client-ca

      **Description:** 
      

5. ns/openshift-kube-apiserver configmap/aggregator-client-ca

      **Description:** 
      

6. ns/openshift-kube-apiserver configmap/client-ca

      **Description:** 
      

7. ns/openshift-kube-apiserver configmap/kube-apiserver-server-ca

      **Description:** 
      

8. ns/openshift-kube-apiserver-operator configmap/kube-apiserver-to-kubelet-client-ca

      **Description:** 
      

9. ns/openshift-kube-apiserver-operator configmap/kube-control-plane-signer-ca

      **Description:** 
      

10. ns/openshift-kube-apiserver-operator configmap/loadbalancer-serving-ca

      **Description:** 
      

11. ns/openshift-kube-apiserver-operator configmap/localhost-recovery-serving-ca

      **Description:** 
      

12. ns/openshift-kube-apiserver-operator configmap/localhost-serving-ca

      **Description:** 
      

13. ns/openshift-kube-apiserver-operator configmap/node-system-admin-ca

      **Description:** 
      

14. ns/openshift-kube-apiserver-operator configmap/service-network-serving-ca

      **Description:** 
      

15. ns/openshift-kube-controller-manager configmap/aggregator-client-ca

      **Description:** 
      

16. ns/openshift-kube-controller-manager configmap/client-ca

      **Description:** 
      

17. ns/openshift-route-controller-manager configmap/client-ca

      **Description:** 
      



### kube-controller-manager (10)
#### Certificates (3)
1. ns/openshift-kube-controller-manager secret/csr-signer

      **Description:** 
      

2. ns/openshift-kube-controller-manager-operator secret/csr-signer

      **Description:** 
      

3. ns/openshift-kube-controller-manager-operator secret/csr-signer-signer

      **Description:** 
      



#### Certificate Authority Bundles (7)
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
      



### kube-scheduler (1)
#### Certificate Authority Bundles (1)
1. ns/openshift-kube-scheduler configmap/serviceaccount-ca

      **Description:** 
      



## Items That DO Meet the Requirement (88)
### service-ca (88)
#### Certificates (85)
1. ns/openshift-apiserver secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/api with hostname api.openshift-apiserver.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

2. ns/openshift-apiserver-operator secret/openshift-apiserver-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-apiserver-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-apiserver-operator-serving-cert'. The certificate is valid for 2 years.
      

3. ns/openshift-authentication secret/v4-0-config-system-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/oauth-openshift with hostname oauth-openshift.openshift-authentication.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: v4-0-config-system-serving-cert'. The certificate is valid for 2 years.
      

4. ns/openshift-authentication-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-authentication-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

5. ns/openshift-cloud-controller-manager-operator secret/cloud-controller-manager-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cloud-controller-manager-operator with hostname cloud-controller-manager-operator.openshift-cloud-controller-manager-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cloud-controller-manager-operator-tls'. The certificate is valid for 2 years.
      

6. ns/openshift-cloud-credential-operator secret/cloud-credential-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cco-metrics with hostname cco-metrics.openshift-cloud-credential-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cloud-credential-operator-serving-cert'. The certificate is valid for 2 years.
      

7. ns/openshift-cloud-credential-operator secret/pod-identity-webhook

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/pod-identity-webhook with hostname pod-identity-webhook.openshift-cloud-credential-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: pod-identity-webhook'. The certificate is valid for 2 years.
      

8. ns/openshift-cluster-csi-drivers secret/aws-ebs-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/aws-ebs-csi-driver-controller-metrics with hostname aws-ebs-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: aws-ebs-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

9. ns/openshift-cluster-csi-drivers secret/azure-disk-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/azure-disk-csi-driver-controller-metrics with hostname azure-disk-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: azure-disk-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

10. ns/openshift-cluster-csi-drivers secret/azure-file-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/azure-file-csi-driver-controller-metrics with hostname azure-file-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: azure-file-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

11. ns/openshift-cluster-csi-drivers secret/gcp-pd-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/gcp-pd-csi-driver-controller-metrics with hostname gcp-pd-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: gcp-pd-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

12. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/vmware-vsphere-csi-driver-controller-metrics with hostname vmware-vsphere-csi-driver-controller-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: vmware-vsphere-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

13. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-operator-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/vmware-vsphere-csi-driver-operator-metrics with hostname vmware-vsphere-csi-driver-operator-metrics.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: vmware-vsphere-csi-driver-operator-metrics-serving-cert'. The certificate is valid for 2 years.
      

14. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-webhook-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/vmware-vsphere-csi-driver-webhook-svc with hostname vmware-vsphere-csi-driver-webhook-svc.openshift-cluster-csi-drivers.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: vmware-vsphere-csi-driver-webhook-secret'. The certificate is valid for 2 years.
      

15. ns/openshift-cluster-machine-approver secret/machine-approver-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-approver with hostname machine-approver.openshift-cluster-machine-approver.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-approver-tls'. The certificate is valid for 2 years.
      

16. ns/openshift-cluster-node-tuning-operator secret/node-tuning-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/node-tuning-operator with hostname node-tuning-operator.openshift-cluster-node-tuning-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: node-tuning-operator-tls'. The certificate is valid for 2 years.
      

17. ns/openshift-cluster-node-tuning-operator secret/performance-addon-operator-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/performance-addon-operator-service with hostname performance-addon-operator-service.openshift-cluster-node-tuning-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: performance-addon-operator-webhook-cert'. The certificate is valid for 2 years.
      

18. ns/openshift-cluster-samples-operator secret/samples-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-cluster-samples-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: samples-operator-tls'. The certificate is valid for 2 years.
      

19. ns/openshift-cluster-storage-operator secret/cluster-storage-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-storage-operator-metrics with hostname cluster-storage-operator-metrics.openshift-cluster-storage-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-storage-operator-serving-cert'. The certificate is valid for 2 years.
      

20. ns/openshift-cluster-storage-operator secret/csi-snapshot-webhook-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/csi-snapshot-webhook with hostname csi-snapshot-webhook.openshift-cluster-storage-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: csi-snapshot-webhook-secret'. The certificate is valid for 2 years.
      

21. ns/openshift-cluster-storage-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/csi-snapshot-controller-operator-metrics with hostname csi-snapshot-controller-operator-metrics.openshift-cluster-storage-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

22. ns/openshift-cluster-storage-operator secret/vsphere-problem-detector-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/vsphere-problem-detector-metrics with hostname vsphere-problem-detector-metrics.openshift-cluster-storage-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: vsphere-problem-detector-serving-cert'. The certificate is valid for 2 years.
      

23. ns/openshift-cluster-version secret/cluster-version-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-version-operator with hostname cluster-version-operator.openshift-cluster-version.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-version-operator-serving-cert'. The certificate is valid for 2 years.
      

24. ns/openshift-config-operator secret/config-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-config-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: config-operator-serving-cert'. The certificate is valid for 2 years.
      

25. ns/openshift-console secret/console-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/console with hostname console.openshift-console.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: console-serving-cert'. The certificate is valid for 2 years.
      

26. ns/openshift-console-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-console-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

27. ns/openshift-console-operator secret/webhook-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/webhook with hostname webhook.openshift-console-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: webhook-serving-cert'. The certificate is valid for 2 years.
      

28. ns/openshift-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/controller-manager with hostname controller-manager.openshift-controller-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

29. ns/openshift-controller-manager-operator secret/openshift-controller-manager-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-controller-manager-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-controller-manager-operator-serving-cert'. The certificate is valid for 2 years.
      

30. ns/openshift-dns secret/dns-default-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/dns-default with hostname dns-default.openshift-dns.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: dns-default-metrics-tls'. The certificate is valid for 2 years.
      

31. ns/openshift-dns-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-dns-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

32. ns/openshift-e2e-loki secret/proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/promtail with hostname promtail.openshift-e2e-loki.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: proxy-tls'. The certificate is valid for 2 years.
      

33. ns/openshift-etcd secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/etcd with hostname etcd.openshift-etcd.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

34. ns/openshift-etcd-operator secret/etcd-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-etcd-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: etcd-operator-serving-cert'. The certificate is valid for 2 years.
      

35. ns/openshift-image-registry secret/image-registry-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/image-registry-operator with hostname image-registry-operator.openshift-image-registry.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: image-registry-operator-tls'. The certificate is valid for 2 years.
      

36. ns/openshift-image-registry secret/image-registry-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/image-registry with hostname image-registry.openshift-image-registry.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: image-registry-tls'. The certificate is valid for 2 years.
      

37. ns/openshift-ingress secret/router-metrics-certs-default

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/router-internal-default with hostname router-internal-default.openshift-ingress.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: router-metrics-certs-default'. The certificate is valid for 2 years.
      

38. ns/openshift-ingress-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-ingress-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

39. ns/openshift-insights secret/openshift-insights-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-insights.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-insights-serving-cert'. The certificate is valid for 2 years.
      

40. ns/openshift-kube-apiserver-operator secret/kube-apiserver-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-kube-apiserver-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-apiserver-operator-serving-cert'. The certificate is valid for 2 years.
      

41. ns/openshift-kube-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/kube-controller-manager with hostname kube-controller-manager.openshift-kube-controller-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

42. ns/openshift-kube-controller-manager-operator secret/kube-controller-manager-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-kube-controller-manager-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-controller-manager-operator-serving-cert'. The certificate is valid for 2 years.
      

43. ns/openshift-kube-scheduler secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/scheduler with hostname scheduler.openshift-kube-scheduler.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

44. ns/openshift-kube-scheduler-operator secret/kube-scheduler-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-kube-scheduler-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-scheduler-operator-serving-cert'. The certificate is valid for 2 years.
      

45. ns/openshift-kube-storage-version-migrator-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-kube-storage-version-migrator-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

46. ns/openshift-machine-api secret/baremetal-operator-webhook-server-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/baremetal-operator-webhook-service with hostname baremetal-operator-webhook-service.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: baremetal-operator-webhook-server-cert'. The certificate is valid for 2 years.
      

47. ns/openshift-machine-api secret/cluster-autoscaler-operator-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-autoscaler-operator with hostname cluster-autoscaler-operator.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-autoscaler-operator-cert'. The certificate is valid for 2 years.
      

48. ns/openshift-machine-api secret/cluster-baremetal-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-baremetal-operator-service with hostname cluster-baremetal-operator-service.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-baremetal-operator-tls'. The certificate is valid for 2 years.
      

49. ns/openshift-machine-api secret/cluster-baremetal-webhook-server-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-baremetal-webhook-service with hostname cluster-baremetal-webhook-service.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-baremetal-webhook-server-cert'. The certificate is valid for 2 years.
      

50. ns/openshift-machine-api secret/control-plane-machine-set-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/control-plane-machine-set-operator with hostname control-plane-machine-set-operator.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: control-plane-machine-set-operator-tls'. The certificate is valid for 2 years.
      

51. ns/openshift-machine-api secret/machine-api-controllers-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-api-controllers with hostname machine-api-controllers.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-controllers-tls'. The certificate is valid for 2 years.
      

52. ns/openshift-machine-api secret/machine-api-operator-machine-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-api-operator-machine-webhook with hostname machine-api-operator-machine-webhook.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-machine-webhook-cert'. The certificate is valid for 2 years.
      

53. ns/openshift-machine-api secret/machine-api-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-api-operator with hostname machine-api-operator.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-tls'. The certificate is valid for 2 years.
      

54. ns/openshift-machine-api secret/machine-api-operator-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-api-operator-webhook with hostname machine-api-operator-webhook.openshift-machine-api.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-webhook-cert'. The certificate is valid for 2 years.
      

55. ns/openshift-machine-config-operator secret/mcc-proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-config-controller with hostname machine-config-controller.openshift-machine-config-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: mcc-proxy-tls'. The certificate is valid for 2 years.
      

56. ns/openshift-machine-config-operator secret/mco-proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-config-operator with hostname machine-config-operator.openshift-machine-config-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: mco-proxy-tls'. The certificate is valid for 2 years.
      

57. ns/openshift-machine-config-operator secret/proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/machine-config-daemon with hostname machine-config-daemon.openshift-machine-config-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: proxy-tls'. The certificate is valid for 2 years.
      

58. ns/openshift-marketplace secret/marketplace-operator-metrics

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/marketplace-operator-metrics with hostname marketplace-operator-metrics.openshift-marketplace.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: marketplace-operator-metrics'. The certificate is valid for 2 years.
      

59. ns/openshift-monitoring secret/alertmanager-main-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/alertmanager-main with hostname alertmanager-main.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: alertmanager-main-tls'. The certificate is valid for 2 years.
      

60. ns/openshift-monitoring secret/cluster-monitoring-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/cluster-monitoring-operator with hostname cluster-monitoring-operator.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-monitoring-operator-tls'. The certificate is valid for 2 years.
      

61. ns/openshift-monitoring secret/kube-state-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/kube-state-metrics with hostname kube-state-metrics.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-state-metrics-tls'. The certificate is valid for 2 years.
      

62. ns/openshift-monitoring secret/monitoring-plugin-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/monitoring-plugin with hostname monitoring-plugin.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: monitoring-plugin-cert'. The certificate is valid for 2 years.
      

63. ns/openshift-monitoring secret/node-exporter-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/node-exporter with hostname node-exporter.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: node-exporter-tls'. The certificate is valid for 2 years.
      

64. ns/openshift-monitoring secret/openshift-state-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/openshift-state-metrics with hostname openshift-state-metrics.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-state-metrics-tls'. The certificate is valid for 2 years.
      

65. ns/openshift-monitoring secret/prometheus-adapter-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-adapter with hostname prometheus-adapter.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-adapter-tls'. The certificate is valid for 2 years.
      

66. ns/openshift-monitoring secret/prometheus-k8s-thanos-sidecar-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-k8s-thanos-sidecar with hostname prometheus-k8s-thanos-sidecar.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-k8s-thanos-sidecar-tls'. The certificate is valid for 2 years.
      

67. ns/openshift-monitoring secret/prometheus-k8s-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-k8s with hostname prometheus-k8s.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-k8s-tls'. The certificate is valid for 2 years.
      

68. ns/openshift-monitoring secret/prometheus-operator-admission-webhook-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-operator-admission-webhook with hostname prometheus-operator-admission-webhook.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-operator-admission-webhook-tls'. The certificate is valid for 2 years.
      

69. ns/openshift-monitoring secret/prometheus-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/prometheus-operator with hostname prometheus-operator.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-operator-tls'. The certificate is valid for 2 years.
      

70. ns/openshift-monitoring secret/telemeter-client-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/telemeter-client with hostname telemeter-client.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: telemeter-client-tls'. The certificate is valid for 2 years.
      

71. ns/openshift-monitoring secret/thanos-querier-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/thanos-querier with hostname thanos-querier.openshift-monitoring.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: thanos-querier-tls'. The certificate is valid for 2 years.
      

72. ns/openshift-multus secret/metrics-daemon-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/network-metrics-service with hostname network-metrics-service.openshift-multus.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-daemon-secret'. The certificate is valid for 2 years.
      

73. ns/openshift-multus secret/multus-admission-controller-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/multus-admission-controller with hostname multus-admission-controller.openshift-multus.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: multus-admission-controller-secret'. The certificate is valid for 2 years.
      

74. ns/openshift-network-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-network-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

75. ns/openshift-oauth-apiserver secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/api with hostname api.openshift-oauth-apiserver.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

76. ns/openshift-operator-lifecycle-manager secret/catalog-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/catalog-operator-metrics with hostname catalog-operator-metrics.openshift-operator-lifecycle-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: catalog-operator-serving-cert'. The certificate is valid for 2 years.
      

77. ns/openshift-operator-lifecycle-manager secret/olm-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/olm-operator-metrics with hostname olm-operator-metrics.openshift-operator-lifecycle-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: olm-operator-serving-cert'. The certificate is valid for 2 years.
      

78. ns/openshift-operator-lifecycle-manager secret/package-server-manager-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/package-server-manager-metrics with hostname package-server-manager-metrics.openshift-operator-lifecycle-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: package-server-manager-serving-cert'. The certificate is valid for 2 years.
      

79. ns/openshift-ovn-kubernetes secret/ovn-control-plane-metrics-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/ovn-kubernetes-control-plane with hostname ovn-kubernetes-control-plane.openshift-ovn-kubernetes.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: ovn-control-plane-metrics-cert'. The certificate is valid for 2 years.
      

80. ns/openshift-ovn-kubernetes secret/ovn-node-metrics-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/ovn-kubernetes-node with hostname ovn-kubernetes-node.openshift-ovn-kubernetes.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: ovn-node-metrics-cert'. The certificate is valid for 2 years.
      

81. ns/openshift-route-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/route-controller-manager with hostname route-controller-manager.openshift-route-controller-manager.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

82. ns/openshift-sdn secret/sdn-controller-metrics-certs

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/sdn-controller with hostname sdn-controller.openshift-sdn.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: sdn-controller-metrics-certs'. The certificate is valid for 2 years.
      

83. ns/openshift-sdn secret/sdn-metrics-certs

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/sdn with hostname sdn.openshift-sdn.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: sdn-metrics-certs'. The certificate is valid for 2 years.
      

84. ns/openshift-service-ca secret/signing-key

      **Description:** Service CA secret contains a signing key that will be used to issue a signed serving certificate/key pair to services annotated with 'service.beta.openshift.io/serving-cert-secret-name'
      

85. ns/openshift-service-ca-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service/metrics with hostname metrics.openshift-service-ca-operator.svc and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      



#### Certificate Authority Bundles (3)
1. ns/openshift-config-managed configmap/service-ca

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      

2. ns/openshift-kube-controller-manager configmap/service-ca

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      

3. ns/openshift-service-ca configmap/signing-cabundle

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      



