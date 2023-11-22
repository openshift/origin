# Certificate Ownership

## Table of Contents
  - [Missing Owners (22)](#Missing-Owners-22)
    - [Certificates (9)](#Certificates-9)
    - [Certificate Authority Bundles (13)](#Certificate-Authority-Bundles-13)
  - [Cloud Compute / Cloud Controller Manager (1)](#Cloud-Compute-/-Cloud-Controller-Manager-1)
    - [Certificate Authority Bundles (1)](#Certificate-Authority-Bundles-1)
  - [Etcd (32)](#Etcd-32)
    - [Certificates (19)](#Certificates-19)
    - [Certificate Authority Bundles (13)](#Certificate-Authority-Bundles-13)
  - [Networking / cluster-network-operator (31)](#Networking-/-cluster-network-operator-31)
    - [Certificates (6)](#Certificates-6)
    - [Certificate Authority Bundles (25)](#Certificate-Authority-Bundles-25)
  - [Networking / router (3)](#Networking-/-router-3)
    - [Certificates (1)](#Certificates-1)
    - [Certificate Authority Bundles (2)](#Certificate-Authority-Bundles-2)
  - [kube-apiserver (40)](#kube-apiserver-40)
    - [Certificates (22)](#Certificates-22)
    - [Certificate Authority Bundles (18)](#Certificate-Authority-Bundles-18)
  - [kube-controller-manager (5)](#kube-controller-manager-5)
    - [Certificates (2)](#Certificates-2)
    - [Certificate Authority Bundles (3)](#Certificate-Authority-Bundles-3)
  - [service-ca (76)](#service-ca-76)
    - [Certificates (73)](#Certificates-73)
    - [Certificate Authority Bundles (3)](#Certificate-Authority-Bundles-3)


## Missing Owners (22)
### Certificates (9)
1. ns/openshift-ingress secret/router-certs-default

      **Description:** 
      

2. ns/openshift-kube-controller-manager secret/csr-signer

      **Description:** 
      

3. ns/openshift-machine-config-operator secret/machine-config-server-tls

      **Description:** 
      

4. ns/openshift-monitoring secret/federate-client-certs

      **Description:** 
      

5. ns/openshift-monitoring secret/metrics-client-certs

      **Description:** 
      

6. ns/openshift-monitoring secret/prometheus-adapter-e3ihmvv2tuqs4

      **Description:** 
      

7. ns/openshift-oauth-apiserver secret/openshift-authenticator-certs

      **Description:** 
      

8. ns/openshift-operator-lifecycle-manager secret/packageserver-service-cert

      **Description:** 
      

9. ns/openshift-operator-lifecycle-manager secret/pprof-cert

      **Description:** 
      



      

### Certificate Authority Bundles (13)
1. ns/openshift-config configmap/initial-kube-apiserver-server-ca

      **Description:** 
      

2. ns/openshift-config-managed configmap/csr-controller-ca

      **Description:** 
      

3. ns/openshift-config-managed configmap/kubelet-bootstrap-kubeconfig

      **Description:** 
      

4. ns/openshift-config-managed configmap/oauth-serving-cert

      **Description:** 
      

5. ns/openshift-console configmap/oauth-serving-cert

      **Description:** 
      

6. ns/openshift-kube-controller-manager configmap/serviceaccount-ca

      **Description:** 
      

7. ns/openshift-kube-controller-manager-operator configmap/csr-controller-ca

      **Description:** 
      

8. ns/openshift-kube-controller-manager-operator configmap/csr-signer-ca

      **Description:** 
      

9. ns/openshift-kube-scheduler configmap/serviceaccount-ca

      **Description:** 
      

10. ns/openshift-monitoring configmap/alertmanager-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

11. ns/openshift-monitoring configmap/kubelet-serving-ca-bundle

      **Description:** 
      

12. ns/openshift-monitoring configmap/prometheus-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

13. ns/openshift-monitoring configmap/thanos-querier-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      



      

## Cloud Compute / Cloud Controller Manager (1)
### Certificate Authority Bundles (1)
1. ns/openshift-cloud-controller-manager configmap/ccm-trusted-ca

      **Description:** 
      



      

## Etcd (32)
### Certificates (19)
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
      



      

### Certificate Authority Bundles (13)
1. ns/openshift-apiserver configmap/etcd-serving-ca

      **Description:** 
      

2. ns/openshift-config configmap/etcd-ca-bundle

      **Description:** 
      

3. ns/openshift-config configmap/etcd-metric-serving-ca

      **Description:** 
      

4. ns/openshift-config configmap/etcd-serving-ca

      **Description:** 
      

5. ns/openshift-etcd configmap/etcd-ca-bundle

      **Description:** 
      

6. ns/openshift-etcd configmap/etcd-metrics-proxy-client-ca

      **Description:** 
      

7. ns/openshift-etcd configmap/etcd-metrics-proxy-serving-ca

      **Description:** 
      

8. ns/openshift-etcd configmap/etcd-peer-client-ca

      **Description:** 
      

9. ns/openshift-etcd configmap/etcd-serving-ca

      **Description:** 
      

10. ns/openshift-etcd-operator configmap/etcd-ca-bundle

      **Description:** 
      

11. ns/openshift-etcd-operator configmap/etcd-metric-serving-ca

      **Description:** 
      

12. ns/openshift-kube-apiserver configmap/etcd-serving-ca

      **Description:** 
      

13. ns/openshift-oauth-apiserver configmap/etcd-serving-ca

      **Description:** 
      



      

## Networking / cluster-network-operator (31)
### Certificates (6)
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
      



      

### Certificate Authority Bundles (25)
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
      

8. ns/openshift-cluster-node-tuning-operator configmap/trusted-ca

      **Description:** 
      

9. ns/openshift-config-managed configmap/trusted-ca-bundle

      **Description:** 
      

10. ns/openshift-console configmap/trusted-ca-bundle

      **Description:** 
      

11. ns/openshift-controller-manager configmap/openshift-global-ca

      **Description:** 
      

12. ns/openshift-image-registry configmap/trusted-ca

      **Description:** 
      

13. ns/openshift-ingress-operator configmap/trusted-ca

      **Description:** 
      

14. ns/openshift-insights configmap/trusted-ca-bundle

      **Description:** 
      

15. ns/openshift-kube-apiserver configmap/trusted-ca-bundle

      **Description:** 
      

16. ns/openshift-kube-controller-manager configmap/trusted-ca-bundle

      **Description:** 
      

17. ns/openshift-machine-api configmap/cbo-trusted-ca

      **Description:** 
      

18. ns/openshift-machine-api configmap/mao-trusted-ca

      **Description:** 
      

19. ns/openshift-marketplace configmap/marketplace-trusted-ca

      **Description:** 
      

20. ns/openshift-monitoring configmap/alertmanager-trusted-ca-bundle

      **Description:** 
      

21. ns/openshift-monitoring configmap/prometheus-trusted-ca-bundle

      **Description:** 
      

22. ns/openshift-monitoring configmap/thanos-querier-trusted-ca-bundle

      **Description:** 
      

23. ns/openshift-network-node-identity configmap/network-node-identity-ca

      **Description:** 
      

24. ns/openshift-ovn-kubernetes configmap/ovn-ca

      **Description:** 
      

25. ns/openshift-ovn-kubernetes configmap/signer-ca

      **Description:** 
      



      

## Networking / router (3)
### Certificates (1)
1. ns/openshift-ingress-operator secret/router-ca

      **Description:** 
      



      

### Certificate Authority Bundles (2)
1. ns/openshift-config-managed configmap/default-ingress-cert

      **Description:** 
      

2. ns/openshift-console configmap/default-ingress-cert

      **Description:** 
      



      

## kube-apiserver (40)
### Certificates (22)
1. ns/openshift-config-managed secret/kube-controller-manager-client-cert-key

      **Description:** Client certificate used by the kube-controller-manager to authenticate to the kube-apiserver.
      

2. ns/openshift-config-managed secret/kube-scheduler-client-cert-key

      **Description:** Client certificate used by the kube-scheduler to authenticate to the kube-apiserver.
      

3. ns/openshift-kube-apiserver secret/aggregator-client

      **Description:** Client certificate used by the kube-apiserver to communicate to aggregated apiservers.
      

4. ns/openshift-kube-apiserver secret/check-endpoints-client-cert-key

      **Description:** Client certificate used by the check endpoints sidecar
      

5. ns/openshift-kube-apiserver secret/control-plane-node-admin-client-cert-key

      **Description:** Client certificate used by the control plane node admin
      

6. ns/openshift-kube-apiserver secret/external-loadbalancer-serving-certkey

      **Description:** Serving certificate used by the kube-apiserver to terminate requests via the external load balancer.
      

7. ns/openshift-kube-apiserver secret/internal-loadbalancer-serving-certkey

      **Description:** Serving certificate used by the kube-apiserver to terminate requests via the internal load balancer.
      

8. ns/openshift-kube-apiserver secret/kubelet-client

      **Description:** Client certificate used by the kube-apiserver to authenticate to the kubelet for requests like exec and logs.
      

9. ns/openshift-kube-apiserver secret/localhost-recovery-serving-certkey

      **Description:** Serving certificate used by the kube-apiserver to terminate requests via the localhost recovery SNI ServerName.
      

10. ns/openshift-kube-apiserver secret/localhost-serving-cert-certkey

      **Description:** Serving certificate used by the kube-apiserver to terminate requests via localhost.
      

11. ns/openshift-kube-apiserver secret/service-network-serving-certkey

      **Description:** Serving certificate used by the kube-apiserver to terminate requests via the service network.
      

12. ns/openshift-kube-apiserver-operator secret/aggregator-client-signer

      **Description:** Signer for the kube-apiserver to create client certificates for aggregated apiservers to recognize as a front-proxy.
      

13. ns/openshift-kube-apiserver-operator secret/kube-apiserver-to-kubelet-signer

      **Description:** Signer for the kube-apiserver-to-kubelet-client so kubelets can recognize the kube-apiserver.
      

14. ns/openshift-kube-apiserver-operator secret/kube-control-plane-signer

      **Description:** Signer for kube-controller-manager and kube-scheduler client certificates.
      

15. ns/openshift-kube-apiserver-operator secret/loadbalancer-serving-signer

      **Description:** Signer used by the kube-apiserver operator to create serving certificates for the kube-apiserver via internal and external load balancers.
      

16. ns/openshift-kube-apiserver-operator secret/localhost-recovery-serving-signer

      **Description:** Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via the localhost recovery SNI ServerName
      

17. ns/openshift-kube-apiserver-operator secret/localhost-serving-signer

      **Description:** Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via localhost.
      

18. ns/openshift-kube-apiserver-operator secret/node-system-admin-client

      **Description:** Client certificate (system:masters) placed on each master to allow communication to kube-apiserver for debugging.
      

19. ns/openshift-kube-apiserver-operator secret/node-system-admin-signer

      **Description:** Signer for the per-master-debugging-client.
      

20. ns/openshift-kube-apiserver-operator secret/service-network-serving-signer

      **Description:** Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via the service network.
      

21. ns/openshift-kube-controller-manager secret/kube-controller-manager-client-cert-key

      **Description:** Client certificate used by the kube-controller-manager to authenticate to the kube-apiserver.
      

22. ns/openshift-kube-scheduler secret/kube-scheduler-client-cert-key

      **Description:** Client certificate used by the kube-scheduler to authenticate to the kube-apiserver.
      



      

### Certificate Authority Bundles (18)
1. ns/openshift-config configmap/admin-kubeconfig-client-ca

      **Description:** 
      

2. ns/openshift-config-managed configmap/kube-apiserver-aggregator-client-ca

      **Description:** CA for aggregated apiservers to recognize kube-apiserver as front-proxy.
      

3. ns/openshift-config-managed configmap/kube-apiserver-client-ca

      **Description:** CA for kube-apiserver clients
      

4. ns/openshift-config-managed configmap/kube-apiserver-server-ca

      **Description:** CA for kube-apiserver server
      

5. ns/openshift-controller-manager configmap/client-ca

      **Description:** CA for kube-apiserver clients
      

6. ns/openshift-kube-apiserver configmap/aggregator-client-ca

      **Description:** CA for aggregated apiservers to recognize kube-apiserver as front-proxy.
      

7. ns/openshift-kube-apiserver configmap/client-ca

      **Description:** CA for kube-apiserver clients
      

8. ns/openshift-kube-apiserver configmap/kube-apiserver-server-ca

      **Description:** CA for kube-apiserver server
      

9. ns/openshift-kube-apiserver-operator configmap/kube-apiserver-to-kubelet-client-ca

      **Description:** CA for the kubelet to recognize the kube-apiserver client certificate.
      

10. ns/openshift-kube-apiserver-operator configmap/kube-control-plane-signer-ca

      **Description:** CA for kube-apiserver to recognize the kube-controller-manager and kube-scheduler client certificates.
      

11. ns/openshift-kube-apiserver-operator configmap/loadbalancer-serving-ca

      **Description:** CA for recognizing the kube-apiserver when connecting via the internal or external load balancers.
      

12. ns/openshift-kube-apiserver-operator configmap/localhost-recovery-serving-ca

      **Description:** CA for recognizing the kube-apiserver when connecting via the localhost recovery SNI ServerName.
      

13. ns/openshift-kube-apiserver-operator configmap/localhost-serving-ca

      **Description:** CA for recognizing the kube-apiserver when connecting via localhost.
      

14. ns/openshift-kube-apiserver-operator configmap/node-system-admin-ca

      **Description:** CA for kube-apiserver to recognize local system:masters rendered to each master.
      

15. ns/openshift-kube-apiserver-operator configmap/service-network-serving-ca

      **Description:** CA for recognizing the kube-apiserver when connecting via the service network (kuberentes.default.svc).
      

16. ns/openshift-kube-controller-manager configmap/aggregator-client-ca

      **Description:** CA for aggregated apiservers to recognize kube-apiserver as front-proxy.
      

17. ns/openshift-kube-controller-manager configmap/client-ca

      **Description:** CA for kube-apiserver clients
      

18. ns/openshift-route-controller-manager configmap/client-ca

      **Description:** CA for kube-apiserver clients
      



      

## kube-controller-manager (5)
### Certificates (2)
1. ns/openshift-kube-controller-manager-operator secret/csr-signer

      **Description:** 
      

2. ns/openshift-kube-controller-manager-operator secret/csr-signer-signer

      **Description:** 
      



      

### Certificate Authority Bundles (3)
1. ns/openshift-config-managed configmap/kubelet-serving-ca

      **Description:** 
      

2. ns/openshift-kube-apiserver configmap/kubelet-serving-ca

      **Description:** 
      

3. ns/openshift-kube-controller-manager-operator configmap/csr-controller-signer-ca

      **Description:** 
      



      

## service-ca (76)
### Certificates (73)
1. ns/openshift-apiserver secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service api/openshift-apiserver and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

2. ns/openshift-apiserver-operator secret/openshift-apiserver-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-apiserver-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-apiserver-operator-serving-cert'. The certificate is valid for 2 years.
      

3. ns/openshift-authentication secret/v4-0-config-system-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service oauth-openshift/openshift-authentication and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: v4-0-config-system-serving-cert'. The certificate is valid for 2 years.
      

4. ns/openshift-authentication-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-authentication-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

5. ns/openshift-cloud-credential-operator secret/cloud-credential-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service cco-metrics/openshift-cloud-credential-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cloud-credential-operator-serving-cert'. The certificate is valid for 2 years.
      

6. ns/openshift-cloud-credential-operator secret/pod-identity-webhook

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service pod-identity-webhook/openshift-cloud-credential-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: pod-identity-webhook'. The certificate is valid for 2 years.
      

7. ns/openshift-cluster-csi-drivers secret/aws-ebs-csi-driver-controller-metrics-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service aws-ebs-csi-driver-controller-metrics/openshift-cluster-csi-drivers and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: aws-ebs-csi-driver-controller-metrics-serving-cert'. The certificate is valid for 2 years.
      

8. ns/openshift-cluster-machine-approver secret/machine-approver-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service machine-approver/openshift-cluster-machine-approver and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-approver-tls'. The certificate is valid for 2 years.
      

9. ns/openshift-cluster-node-tuning-operator secret/node-tuning-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service node-tuning-operator/openshift-cluster-node-tuning-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: node-tuning-operator-tls'. The certificate is valid for 2 years.
      

10. ns/openshift-cluster-node-tuning-operator secret/performance-addon-operator-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service performance-addon-operator-service/openshift-cluster-node-tuning-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: performance-addon-operator-webhook-cert'. The certificate is valid for 2 years.
      

11. ns/openshift-cluster-samples-operator secret/samples-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-cluster-samples-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: samples-operator-tls'. The certificate is valid for 2 years.
      

12. ns/openshift-cluster-storage-operator secret/cluster-storage-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service cluster-storage-operator-metrics/openshift-cluster-storage-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-storage-operator-serving-cert'. The certificate is valid for 2 years.
      

13. ns/openshift-cluster-storage-operator secret/csi-snapshot-webhook-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service csi-snapshot-webhook/openshift-cluster-storage-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: csi-snapshot-webhook-secret'. The certificate is valid for 2 years.
      

14. ns/openshift-cluster-storage-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service csi-snapshot-controller-operator-metrics/openshift-cluster-storage-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

15. ns/openshift-cluster-version secret/cluster-version-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service cluster-version-operator/openshift-cluster-version and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-version-operator-serving-cert'. The certificate is valid for 2 years.
      

16. ns/openshift-config-operator secret/config-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-config-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: config-operator-serving-cert'. The certificate is valid for 2 years.
      

17. ns/openshift-console secret/console-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service console/openshift-console and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: console-serving-cert'. The certificate is valid for 2 years.
      

18. ns/openshift-console-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-console-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

19. ns/openshift-console-operator secret/webhook-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service webhook/openshift-console-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: webhook-serving-cert'. The certificate is valid for 2 years.
      

20. ns/openshift-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service controller-manager/openshift-controller-manager and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

21. ns/openshift-controller-manager-operator secret/openshift-controller-manager-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-controller-manager-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-controller-manager-operator-serving-cert'. The certificate is valid for 2 years.
      

22. ns/openshift-dns secret/dns-default-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service dns-default/openshift-dns and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: dns-default-metrics-tls'. The certificate is valid for 2 years.
      

23. ns/openshift-dns-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-dns-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

24. ns/openshift-e2e-loki secret/proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service promtail/openshift-e2e-loki and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: proxy-tls'. The certificate is valid for 2 years.
      

25. ns/openshift-etcd secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service etcd/openshift-etcd and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

26. ns/openshift-etcd-operator secret/etcd-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-etcd-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: etcd-operator-serving-cert'. The certificate is valid for 2 years.
      

27. ns/openshift-image-registry secret/image-registry-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service image-registry-operator/openshift-image-registry and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: image-registry-operator-tls'. The certificate is valid for 2 years.
      

28. ns/openshift-image-registry secret/image-registry-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service image-registry/openshift-image-registry and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: image-registry-tls'. The certificate is valid for 2 years.
      

29. ns/openshift-ingress secret/router-metrics-certs-default

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service router-internal-default/openshift-ingress and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: router-metrics-certs-default'. The certificate is valid for 2 years.
      

30. ns/openshift-ingress-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-ingress-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

31. ns/openshift-insights secret/openshift-insights-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-insights and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-insights-serving-cert'. The certificate is valid for 2 years.
      

32. ns/openshift-kube-apiserver-operator secret/kube-apiserver-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-kube-apiserver-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-apiserver-operator-serving-cert'. The certificate is valid for 2 years.
      

33. ns/openshift-kube-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service kube-controller-manager/openshift-kube-controller-manager and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

34. ns/openshift-kube-controller-manager-operator secret/kube-controller-manager-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-kube-controller-manager-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-controller-manager-operator-serving-cert'. The certificate is valid for 2 years.
      

35. ns/openshift-kube-scheduler secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service scheduler/openshift-kube-scheduler and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

36. ns/openshift-kube-scheduler-operator secret/kube-scheduler-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-kube-scheduler-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-scheduler-operator-serving-cert'. The certificate is valid for 2 years.
      

37. ns/openshift-kube-storage-version-migrator-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-kube-storage-version-migrator-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

38. ns/openshift-machine-api secret/cluster-autoscaler-operator-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service cluster-autoscaler-operator/openshift-machine-api and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-autoscaler-operator-cert'. The certificate is valid for 2 years.
      

39. ns/openshift-machine-api secret/cluster-baremetal-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service cluster-baremetal-operator-service/openshift-machine-api and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-baremetal-operator-tls'. The certificate is valid for 2 years.
      

40. ns/openshift-machine-api secret/cluster-baremetal-webhook-server-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service cluster-baremetal-webhook-service/openshift-machine-api and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-baremetal-webhook-server-cert'. The certificate is valid for 2 years.
      

41. ns/openshift-machine-api secret/control-plane-machine-set-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service control-plane-machine-set-operator/openshift-machine-api and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: control-plane-machine-set-operator-tls'. The certificate is valid for 2 years.
      

42. ns/openshift-machine-api secret/machine-api-controllers-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service machine-api-controllers/openshift-machine-api and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-controllers-tls'. The certificate is valid for 2 years.
      

43. ns/openshift-machine-api secret/machine-api-operator-machine-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service machine-api-operator-machine-webhook/openshift-machine-api and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-machine-webhook-cert'. The certificate is valid for 2 years.
      

44. ns/openshift-machine-api secret/machine-api-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service machine-api-operator/openshift-machine-api and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-tls'. The certificate is valid for 2 years.
      

45. ns/openshift-machine-api secret/machine-api-operator-webhook-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service machine-api-operator-webhook/openshift-machine-api and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: machine-api-operator-webhook-cert'. The certificate is valid for 2 years.
      

46. ns/openshift-machine-config-operator secret/mcc-proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service machine-config-controller/openshift-machine-config-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: mcc-proxy-tls'. The certificate is valid for 2 years.
      

47. ns/openshift-machine-config-operator secret/mco-proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service machine-config-operator/openshift-machine-config-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: mco-proxy-tls'. The certificate is valid for 2 years.
      

48. ns/openshift-machine-config-operator secret/proxy-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service machine-config-daemon/openshift-machine-config-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: proxy-tls'. The certificate is valid for 2 years.
      

49. ns/openshift-marketplace secret/marketplace-operator-metrics

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service marketplace-operator-metrics/openshift-marketplace and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: marketplace-operator-metrics'. The certificate is valid for 2 years.
      

50. ns/openshift-monitoring secret/alertmanager-main-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service alertmanager-main/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: alertmanager-main-tls'. The certificate is valid for 2 years.
      

51. ns/openshift-monitoring secret/cluster-monitoring-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service cluster-monitoring-operator/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: cluster-monitoring-operator-tls'. The certificate is valid for 2 years.
      

52. ns/openshift-monitoring secret/kube-state-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service kube-state-metrics/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: kube-state-metrics-tls'. The certificate is valid for 2 years.
      

53. ns/openshift-monitoring secret/monitoring-plugin-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service monitoring-plugin/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: monitoring-plugin-cert'. The certificate is valid for 2 years.
      

54. ns/openshift-monitoring secret/node-exporter-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service node-exporter/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: node-exporter-tls'. The certificate is valid for 2 years.
      

55. ns/openshift-monitoring secret/openshift-state-metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service openshift-state-metrics/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: openshift-state-metrics-tls'. The certificate is valid for 2 years.
      

56. ns/openshift-monitoring secret/prometheus-adapter-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service prometheus-adapter/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-adapter-tls'. The certificate is valid for 2 years.
      

57. ns/openshift-monitoring secret/prometheus-k8s-thanos-sidecar-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service prometheus-k8s-thanos-sidecar/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-k8s-thanos-sidecar-tls'. The certificate is valid for 2 years.
      

58. ns/openshift-monitoring secret/prometheus-k8s-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service prometheus-k8s/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-k8s-tls'. The certificate is valid for 2 years.
      

59. ns/openshift-monitoring secret/prometheus-operator-admission-webhook-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service prometheus-operator-admission-webhook/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-operator-admission-webhook-tls'. The certificate is valid for 2 years.
      

60. ns/openshift-monitoring secret/prometheus-operator-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service prometheus-operator/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: prometheus-operator-tls'. The certificate is valid for 2 years.
      

61. ns/openshift-monitoring secret/thanos-querier-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service thanos-querier/openshift-monitoring and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: thanos-querier-tls'. The certificate is valid for 2 years.
      

62. ns/openshift-multus secret/metrics-daemon-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service network-metrics-service/openshift-multus and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-daemon-secret'. The certificate is valid for 2 years.
      

63. ns/openshift-multus secret/multus-admission-controller-secret

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service multus-admission-controller/openshift-multus and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: multus-admission-controller-secret'. The certificate is valid for 2 years.
      

64. ns/openshift-network-operator secret/metrics-tls

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-network-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: metrics-tls'. The certificate is valid for 2 years.
      

65. ns/openshift-oauth-apiserver secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service api/openshift-oauth-apiserver and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

66. ns/openshift-operator-lifecycle-manager secret/catalog-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service catalog-operator-metrics/openshift-operator-lifecycle-manager and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: catalog-operator-serving-cert'. The certificate is valid for 2 years.
      

67. ns/openshift-operator-lifecycle-manager secret/olm-operator-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service olm-operator-metrics/openshift-operator-lifecycle-manager and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: olm-operator-serving-cert'. The certificate is valid for 2 years.
      

68. ns/openshift-operator-lifecycle-manager secret/package-server-manager-serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service package-server-manager-metrics/openshift-operator-lifecycle-manager and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: package-server-manager-serving-cert'. The certificate is valid for 2 years.
      

69. ns/openshift-ovn-kubernetes secret/ovn-control-plane-metrics-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service ovn-kubernetes-control-plane/openshift-ovn-kubernetes and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: ovn-control-plane-metrics-cert'. The certificate is valid for 2 years.
      

70. ns/openshift-ovn-kubernetes secret/ovn-node-metrics-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service ovn-kubernetes-node/openshift-ovn-kubernetes and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: ovn-node-metrics-cert'. The certificate is valid for 2 years.
      

71. ns/openshift-route-controller-manager secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service route-controller-manager/openshift-route-controller-manager and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      

72. ns/openshift-service-ca secret/signing-key

      **Description:** Service CA secret contains a signing key that will be used to issue a signed serving certificate/key pair to services annotated with 'service.beta.openshift.io/serving-cert-secret-name'
      

73. ns/openshift-service-ca-operator secret/serving-cert

      **Description:** Secret contains a pair signed serving certificate/key that is generated by Service CA operator for service metrics/openshift-service-ca-operator and is annotated to the service with annotating a service resource with 'service.beta.openshift.io/serving-cert-secret-name: serving-cert'. The certificate is valid for 2 years.
      



      

### Certificate Authority Bundles (3)
1. ns/openshift-config-managed configmap/service-ca

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      

2. ns/openshift-kube-controller-manager configmap/service-ca

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      

3. ns/openshift-service-ca configmap/signing-cabundle

      **Description:** Service CA configmap contains the data for the PEM-encoded CA signing bundle which will be injected to resources annotated with 'service.beta.openshift.io/inject-cabundle=true'
      



      

