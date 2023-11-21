# Certificate Ownership

## Table of Contents
  - [Missing Owners (44)](#Missing-Owners-44)
    - [Certificates (18)](#Certificates-18)
    - [Certificate Authority Bundles (26)](#Certificate-Authority-Bundles-26)
  - [Etcd (28)](#Etcd-28)
    - [Certificates (19)](#Certificates-19)
    - [Certificate Authority Bundles (9)](#Certificate-Authority-Bundles-9)
  - [Networking / cluster-network-operator (22)](#Networking-/-cluster-network-operator-22)
    - [Certificate Authority Bundles (22)](#Certificate-Authority-Bundles-22)
  - [kube-apiserver (40)](#kube-apiserver-40)
    - [Certificates (22)](#Certificates-22)
    - [Certificate Authority Bundles (18)](#Certificate-Authority-Bundles-18)
  - [service-ca (76)](#service-ca-76)
    - [Certificates (73)](#Certificates-73)
    - [Certificate Authority Bundles (3)](#Certificate-Authority-Bundles-3)


## Missing Owners (44)
### Certificates (18)
1. ns/openshift-ingress secret/router-certs-default

      **Description:** 
      

2. ns/openshift-ingress-operator secret/router-ca

      **Description:** 
      

3. ns/openshift-kube-controller-manager secret/csr-signer

      **Description:** 
      

4. ns/openshift-kube-controller-manager-operator secret/csr-signer

      **Description:** 
      

5. ns/openshift-kube-controller-manager-operator secret/csr-signer-signer

      **Description:** 
      

6. ns/openshift-machine-config-operator secret/machine-config-server-tls

      **Description:** 
      

7. ns/openshift-monitoring secret/federate-client-certs

      **Description:** 
      

8. ns/openshift-monitoring secret/metrics-client-certs

      **Description:** 
      

9. ns/openshift-monitoring secret/prometheus-adapter-2h00aqagi7m9i

      **Description:** 
      

10. ns/openshift-network-node-identity secret/network-node-identity-ca

      **Description:** 
      

11. ns/openshift-network-node-identity secret/network-node-identity-cert

      **Description:** 
      

12. ns/openshift-oauth-apiserver secret/openshift-authenticator-certs

      **Description:** 
      

13. ns/openshift-operator-lifecycle-manager secret/packageserver-service-cert

      **Description:** 
      

14. ns/openshift-operator-lifecycle-manager secret/pprof-cert

      **Description:** 
      

15. ns/openshift-ovn-kubernetes secret/ovn-ca

      **Description:** 
      

16. ns/openshift-ovn-kubernetes secret/ovn-cert

      **Description:** 
      

17. ns/openshift-ovn-kubernetes secret/signer-ca

      **Description:** 
      

18. ns/openshift-ovn-kubernetes secret/signer-cert

      **Description:** 
      



      

### Certificate Authority Bundles (26)
1. ns/openshift-cloud-controller-manager configmap/ccm-trusted-ca

      **Description:** 
      

2. ns/openshift-config configmap/etcd-ca-bundle

      **Description:** 
      

3. ns/openshift-config configmap/initial-kube-apiserver-server-ca

      **Description:** 
      

4. ns/openshift-config-managed configmap/csr-controller-ca

      **Description:** 
      

5. ns/openshift-config-managed configmap/default-ingress-cert

      **Description:** 
      

6. ns/openshift-config-managed configmap/kubelet-bootstrap-kubeconfig

      **Description:** 
      

7. ns/openshift-config-managed configmap/kubelet-serving-ca

      **Description:** 
      

8. ns/openshift-config-managed configmap/oauth-serving-cert

      **Description:** 
      

9. ns/openshift-console configmap/default-ingress-cert

      **Description:** 
      

10. ns/openshift-console configmap/oauth-serving-cert

      **Description:** 
      

11. ns/openshift-etcd configmap/etcd-ca-bundle

      **Description:** 
      

12. ns/openshift-etcd configmap/etcd-peer-client-ca

      **Description:** 
      

13. ns/openshift-etcd-operator configmap/etcd-ca-bundle

      **Description:** 
      

14. ns/openshift-kube-apiserver configmap/kubelet-serving-ca

      **Description:** 
      

15. ns/openshift-kube-controller-manager configmap/serviceaccount-ca

      **Description:** 
      

16. ns/openshift-kube-controller-manager-operator configmap/csr-controller-ca

      **Description:** 
      

17. ns/openshift-kube-controller-manager-operator configmap/csr-controller-signer-ca

      **Description:** 
      

18. ns/openshift-kube-controller-manager-operator configmap/csr-signer-ca

      **Description:** 
      

19. ns/openshift-kube-scheduler configmap/serviceaccount-ca

      **Description:** 
      

20. ns/openshift-monitoring configmap/alertmanager-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

21. ns/openshift-monitoring configmap/kubelet-serving-ca-bundle

      **Description:** 
      

22. ns/openshift-monitoring configmap/prometheus-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

23. ns/openshift-monitoring configmap/thanos-querier-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

24. ns/openshift-network-node-identity configmap/network-node-identity-ca

      **Description:** 
      

25. ns/openshift-ovn-kubernetes configmap/ovn-ca

      **Description:** 
      

26. ns/openshift-ovn-kubernetes configmap/signer-ca

      **Description:** 
      



      

## Etcd (28)
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
      



      

### Certificate Authority Bundles (9)
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
      



      

## Networking / cluster-network-operator (22)
### Certificate Authority Bundles (22)
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

      **Description:** 
      

13. ns/openshift-kube-apiserver-operator secret/kube-apiserver-to-kubelet-signer

      **Description:** 
      

14. ns/openshift-kube-apiserver-operator secret/kube-control-plane-signer

      **Description:** 
      

15. ns/openshift-kube-apiserver-operator secret/loadbalancer-serving-signer

      **Description:** 
      

16. ns/openshift-kube-apiserver-operator secret/localhost-recovery-serving-signer

      **Description:** Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via the localhost recovery SNI ServerName
      

17. ns/openshift-kube-apiserver-operator secret/localhost-serving-signer

      **Description:** 
      

18. ns/openshift-kube-apiserver-operator secret/node-system-admin-client

      **Description:** Client certificate (system:masters) placed on each master to allow communication to kube-apiserver for debugging.
      

19. ns/openshift-kube-apiserver-operator secret/node-system-admin-signer

      **Description:** Signer for the per-master-debugging-client.
      

20. ns/openshift-kube-apiserver-operator secret/service-network-serving-signer

      **Description:** 
      

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
      



      

## service-ca (76)
### Certificates (73)
1. ns/openshift-apiserver secret/serving-cert

      **Description:** 
      

2. ns/openshift-apiserver-operator secret/openshift-apiserver-operator-serving-cert

      **Description:** 
      

3. ns/openshift-authentication secret/v4-0-config-system-serving-cert

      **Description:** 
      

4. ns/openshift-authentication-operator secret/serving-cert

      **Description:** 
      

5. ns/openshift-cloud-credential-operator secret/cloud-credential-operator-serving-cert

      **Description:** 
      

6. ns/openshift-cloud-credential-operator secret/pod-identity-webhook

      **Description:** 
      

7. ns/openshift-cluster-csi-drivers secret/aws-ebs-csi-driver-controller-metrics-serving-cert

      **Description:** 
      

8. ns/openshift-cluster-machine-approver secret/machine-approver-tls

      **Description:** 
      

9. ns/openshift-cluster-node-tuning-operator secret/node-tuning-operator-tls

      **Description:** 
      

10. ns/openshift-cluster-node-tuning-operator secret/performance-addon-operator-webhook-cert

      **Description:** 
      

11. ns/openshift-cluster-samples-operator secret/samples-operator-tls

      **Description:** 
      

12. ns/openshift-cluster-storage-operator secret/cluster-storage-operator-serving-cert

      **Description:** 
      

13. ns/openshift-cluster-storage-operator secret/csi-snapshot-webhook-secret

      **Description:** 
      

14. ns/openshift-cluster-storage-operator secret/serving-cert

      **Description:** 
      

15. ns/openshift-cluster-version secret/cluster-version-operator-serving-cert

      **Description:** 
      

16. ns/openshift-config-operator secret/config-operator-serving-cert

      **Description:** 
      

17. ns/openshift-console secret/console-serving-cert

      **Description:** 
      

18. ns/openshift-console-operator secret/serving-cert

      **Description:** 
      

19. ns/openshift-console-operator secret/webhook-serving-cert

      **Description:** 
      

20. ns/openshift-controller-manager secret/serving-cert

      **Description:** 
      

21. ns/openshift-controller-manager-operator secret/openshift-controller-manager-operator-serving-cert

      **Description:** 
      

22. ns/openshift-dns secret/dns-default-metrics-tls

      **Description:** 
      

23. ns/openshift-dns-operator secret/metrics-tls

      **Description:** 
      

24. ns/openshift-e2e-loki secret/proxy-tls

      **Description:** 
      

25. ns/openshift-etcd secret/serving-cert

      **Description:** 
      

26. ns/openshift-etcd-operator secret/etcd-operator-serving-cert

      **Description:** 
      

27. ns/openshift-image-registry secret/image-registry-operator-tls

      **Description:** 
      

28. ns/openshift-image-registry secret/image-registry-tls

      **Description:** 
      

29. ns/openshift-ingress secret/router-metrics-certs-default

      **Description:** 
      

30. ns/openshift-ingress-operator secret/metrics-tls

      **Description:** 
      

31. ns/openshift-insights secret/openshift-insights-serving-cert

      **Description:** 
      

32. ns/openshift-kube-apiserver-operator secret/kube-apiserver-operator-serving-cert

      **Description:** 
      

33. ns/openshift-kube-controller-manager secret/serving-cert

      **Description:** 
      

34. ns/openshift-kube-controller-manager-operator secret/kube-controller-manager-operator-serving-cert

      **Description:** 
      

35. ns/openshift-kube-scheduler secret/serving-cert

      **Description:** 
      

36. ns/openshift-kube-scheduler-operator secret/kube-scheduler-operator-serving-cert

      **Description:** 
      

37. ns/openshift-kube-storage-version-migrator-operator secret/serving-cert

      **Description:** 
      

38. ns/openshift-machine-api secret/cluster-autoscaler-operator-cert

      **Description:** 
      

39. ns/openshift-machine-api secret/cluster-baremetal-operator-tls

      **Description:** 
      

40. ns/openshift-machine-api secret/cluster-baremetal-webhook-server-cert

      **Description:** 
      

41. ns/openshift-machine-api secret/control-plane-machine-set-operator-tls

      **Description:** 
      

42. ns/openshift-machine-api secret/machine-api-controllers-tls

      **Description:** 
      

43. ns/openshift-machine-api secret/machine-api-operator-machine-webhook-cert

      **Description:** 
      

44. ns/openshift-machine-api secret/machine-api-operator-tls

      **Description:** 
      

45. ns/openshift-machine-api secret/machine-api-operator-webhook-cert

      **Description:** 
      

46. ns/openshift-machine-config-operator secret/mcc-proxy-tls

      **Description:** 
      

47. ns/openshift-machine-config-operator secret/mco-proxy-tls

      **Description:** 
      

48. ns/openshift-machine-config-operator secret/proxy-tls

      **Description:** 
      

49. ns/openshift-marketplace secret/marketplace-operator-metrics

      **Description:** 
      

50. ns/openshift-monitoring secret/alertmanager-main-tls

      **Description:** 
      

51. ns/openshift-monitoring secret/cluster-monitoring-operator-tls

      **Description:** 
      

52. ns/openshift-monitoring secret/kube-state-metrics-tls

      **Description:** 
      

53. ns/openshift-monitoring secret/monitoring-plugin-cert

      **Description:** 
      

54. ns/openshift-monitoring secret/node-exporter-tls

      **Description:** 
      

55. ns/openshift-monitoring secret/openshift-state-metrics-tls

      **Description:** 
      

56. ns/openshift-monitoring secret/prometheus-adapter-tls

      **Description:** 
      

57. ns/openshift-monitoring secret/prometheus-k8s-thanos-sidecar-tls

      **Description:** 
      

58. ns/openshift-monitoring secret/prometheus-k8s-tls

      **Description:** 
      

59. ns/openshift-monitoring secret/prometheus-operator-admission-webhook-tls

      **Description:** 
      

60. ns/openshift-monitoring secret/prometheus-operator-tls

      **Description:** 
      

61. ns/openshift-monitoring secret/thanos-querier-tls

      **Description:** 
      

62. ns/openshift-multus secret/metrics-daemon-secret

      **Description:** 
      

63. ns/openshift-multus secret/multus-admission-controller-secret

      **Description:** 
      

64. ns/openshift-network-operator secret/metrics-tls

      **Description:** 
      

65. ns/openshift-oauth-apiserver secret/serving-cert

      **Description:** 
      

66. ns/openshift-operator-lifecycle-manager secret/catalog-operator-serving-cert

      **Description:** 
      

67. ns/openshift-operator-lifecycle-manager secret/olm-operator-serving-cert

      **Description:** 
      

68. ns/openshift-operator-lifecycle-manager secret/package-server-manager-serving-cert

      **Description:** 
      

69. ns/openshift-ovn-kubernetes secret/ovn-control-plane-metrics-cert

      **Description:** 
      

70. ns/openshift-ovn-kubernetes secret/ovn-node-metrics-cert

      **Description:** 
      

71. ns/openshift-route-controller-manager secret/serving-cert

      **Description:** 
      

72. ns/openshift-service-ca secret/signing-key

      **Description:** 
      

73. ns/openshift-service-ca-operator secret/serving-cert

      **Description:** 
      



      

### Certificate Authority Bundles (3)
1. ns/openshift-config-managed configmap/service-ca

      **Description:** 
      

2. ns/openshift-kube-controller-manager configmap/service-ca

      **Description:** 
      

3. ns/openshift-service-ca configmap/signing-cabundle

      **Description:** 
      



      

