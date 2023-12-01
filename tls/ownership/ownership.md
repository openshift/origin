# Certificate Ownership

## Table of Contents
  - [Missing Owners (91)](#Missing-Owners-91)
    - [Certificates (45)](#Certificates-45)
    - [Certificate Authority Bundles (46)](#Certificate-Authority-Bundles-46)
  - [Etcd (28)](#Etcd-28)
    - [Certificates (19)](#Certificates-19)
    - [Certificate Authority Bundles (9)](#Certificate-Authority-Bundles-9)
  - [Networking / cluster-network-operator (29)](#Networking-/-cluster-network-operator-29)
    - [Certificate Authority Bundles (29)](#Certificate-Authority-Bundles-29)
  - [kube-apiserver (6)](#kube-apiserver-6)
    - [Certificates (6)](#Certificates-6)
  - [service-ca (87)](#service-ca-87)
    - [Certificates (84)](#Certificates-84)
    - [Certificate Authority Bundles (3)](#Certificate-Authority-Bundles-3)


## Missing Owners (91)
### Certificates (45)
1. ns/openshift-config-managed secret/kube-controller-manager-client-cert-key

      **Description:** 
      

2. ns/openshift-config-managed secret/kube-scheduler-client-cert-key

      **Description:** 
      

3. ns/openshift-ingress secret/router-certs-default

      **Description:** 
      

4. ns/openshift-ingress-operator secret/router-ca

      **Description:** 
      

5. ns/openshift-kube-apiserver secret/aggregator-client

      **Description:** 
      

6. ns/openshift-kube-apiserver secret/check-endpoints-client-cert-key

      **Description:** 
      

7. ns/openshift-kube-apiserver secret/control-plane-node-admin-client-cert-key

      **Description:** 
      

8. ns/openshift-kube-apiserver secret/external-loadbalancer-serving-certkey

      **Description:** 
      

9. ns/openshift-kube-apiserver secret/internal-loadbalancer-serving-certkey

      **Description:** 
      

10. ns/openshift-kube-apiserver secret/kubelet-client

      **Description:** 
      

11. ns/openshift-kube-apiserver secret/localhost-recovery-serving-certkey

      **Description:** 
      

12. ns/openshift-kube-apiserver secret/localhost-serving-cert-certkey

      **Description:** 
      

13. ns/openshift-kube-apiserver secret/service-network-serving-certkey

      **Description:** 
      

14. ns/openshift-kube-apiserver-operator secret/localhost-recovery-serving-signer

      **Description:** 
      

15. ns/openshift-kube-apiserver-operator secret/node-system-admin-client

      **Description:** 
      

16. ns/openshift-kube-apiserver-operator secret/node-system-admin-signer

      **Description:** 
      

17. ns/openshift-kube-controller-manager secret/csr-signer

      **Description:** 
      

18. ns/openshift-kube-controller-manager secret/kube-controller-manager-client-cert-key

      **Description:** 
      

19. ns/openshift-kube-controller-manager-operator secret/csr-signer

      **Description:** 
      

20. ns/openshift-kube-controller-manager-operator secret/csr-signer-signer

      **Description:** 
      

21. ns/openshift-kube-scheduler secret/kube-scheduler-client-cert-key

      **Description:** 
      

22. ns/openshift-machine-api secret/metal3-ironic-tls

      **Description:** 
      

23. ns/openshift-machine-config-operator secret/machine-config-server-tls

      **Description:** 
      

24. ns/openshift-monitoring secret/federate-client-certs

      **Description:** 
      

25. ns/openshift-monitoring secret/metrics-client-certs

      **Description:** 
      

26. ns/openshift-monitoring secret/prometheus-adapter-1q4502kr4t411

      **Description:** 
      

27. ns/openshift-monitoring secret/prometheus-adapter-29plsl0b2ei2t

      **Description:** 
      

28. ns/openshift-monitoring secret/prometheus-adapter-43lbje12tka6q

      **Description:** 
      

29. ns/openshift-monitoring secret/prometheus-adapter-4o53ruevt7g9i

      **Description:** 
      

30. ns/openshift-monitoring secret/prometheus-adapter-71k3k5u10lsq

      **Description:** 
      

31. ns/openshift-monitoring secret/prometheus-adapter-9k825ug31dq18

      **Description:** 
      

32. ns/openshift-monitoring secret/prometheus-adapter-bcpovp93k8nqd

      **Description:** 
      

33. ns/openshift-monitoring secret/prometheus-adapter-chpt90tvt3lu0

      **Description:** 
      

34. ns/openshift-monitoring secret/prometheus-adapter-e1s9cs5rknc0d

      **Description:** 
      

35. ns/openshift-monitoring secret/prometheus-adapter-e3t89g68h83ul

      **Description:** 
      

36. ns/openshift-monitoring secret/prometheus-adapter-fgiehq9oii8g6

      **Description:** 
      

37. ns/openshift-network-node-identity secret/network-node-identity-ca

      **Description:** 
      

38. ns/openshift-network-node-identity secret/network-node-identity-cert

      **Description:** 
      

39. ns/openshift-oauth-apiserver secret/openshift-authenticator-certs

      **Description:** 
      

40. ns/openshift-operator-lifecycle-manager secret/packageserver-service-cert

      **Description:** 
      

41. ns/openshift-operator-lifecycle-manager secret/pprof-cert

      **Description:** 
      

42. ns/openshift-ovn-kubernetes secret/ovn-ca

      **Description:** 
      

43. ns/openshift-ovn-kubernetes secret/ovn-cert

      **Description:** 
      

44. ns/openshift-ovn-kubernetes secret/signer-ca

      **Description:** 
      

45. ns/openshift-ovn-kubernetes secret/signer-cert

      **Description:** 
      



      

### Certificate Authority Bundles (46)
1. ns/openshift-cloud-controller-manager configmap/ccm-trusted-ca

      **Description:** 
      

2. ns/openshift-config configmap/admin-kubeconfig-client-ca

      **Description:** 
      

3. ns/openshift-config configmap/etcd-ca-bundle

      **Description:** 
      

4. ns/openshift-config configmap/initial-kube-apiserver-server-ca

      **Description:** 
      

5. ns/openshift-config configmap/user-ca-bundle

      **Description:** 
      

6. ns/openshift-config-managed configmap/csr-controller-ca

      **Description:** 
      

7. ns/openshift-config-managed configmap/default-ingress-cert

      **Description:** 
      

8. ns/openshift-config-managed configmap/kube-apiserver-aggregator-client-ca

      **Description:** 
      

9. ns/openshift-config-managed configmap/kube-apiserver-client-ca

      **Description:** 
      

10. ns/openshift-config-managed configmap/kube-apiserver-server-ca

      **Description:** 
      

11. ns/openshift-config-managed configmap/kubelet-bootstrap-kubeconfig

      **Description:** 
      

12. ns/openshift-config-managed configmap/kubelet-serving-ca

      **Description:** 
      

13. ns/openshift-config-managed configmap/oauth-serving-cert

      **Description:** 
      

14. ns/openshift-console configmap/default-ingress-cert

      **Description:** 
      

15. ns/openshift-console configmap/oauth-serving-cert

      **Description:** 
      

16. ns/openshift-controller-manager configmap/client-ca

      **Description:** 
      

17. ns/openshift-etcd configmap/etcd-ca-bundle

      **Description:** 
      

18. ns/openshift-etcd configmap/etcd-peer-client-ca

      **Description:** 
      

19. ns/openshift-etcd-operator configmap/etcd-ca-bundle

      **Description:** 
      

20. ns/openshift-kube-apiserver configmap/aggregator-client-ca

      **Description:** 
      

21. ns/openshift-kube-apiserver configmap/client-ca

      **Description:** 
      

22. ns/openshift-kube-apiserver configmap/kube-apiserver-server-ca

      **Description:** 
      

23. ns/openshift-kube-apiserver configmap/kubelet-serving-ca

      **Description:** 
      

24. ns/openshift-kube-apiserver-operator configmap/kube-apiserver-to-kubelet-client-ca

      **Description:** 
      

25. ns/openshift-kube-apiserver-operator configmap/kube-control-plane-signer-ca

      **Description:** 
      

26. ns/openshift-kube-apiserver-operator configmap/loadbalancer-serving-ca

      **Description:** 
      

27. ns/openshift-kube-apiserver-operator configmap/localhost-recovery-serving-ca

      **Description:** 
      

28. ns/openshift-kube-apiserver-operator configmap/localhost-serving-ca

      **Description:** 
      

29. ns/openshift-kube-apiserver-operator configmap/node-system-admin-ca

      **Description:** 
      

30. ns/openshift-kube-apiserver-operator configmap/service-network-serving-ca

      **Description:** 
      

31. ns/openshift-kube-controller-manager configmap/aggregator-client-ca

      **Description:** 
      

32. ns/openshift-kube-controller-manager configmap/client-ca

      **Description:** 
      

33. ns/openshift-kube-controller-manager configmap/serviceaccount-ca

      **Description:** 
      

34. ns/openshift-kube-controller-manager-operator configmap/csr-controller-ca

      **Description:** 
      

35. ns/openshift-kube-controller-manager-operator configmap/csr-controller-signer-ca

      **Description:** 
      

36. ns/openshift-kube-controller-manager-operator configmap/csr-signer-ca

      **Description:** 
      

37. ns/openshift-kube-scheduler configmap/serviceaccount-ca

      **Description:** 
      

38. ns/openshift-monitoring configmap/alertmanager-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

39. ns/openshift-monitoring configmap/kubelet-serving-ca-bundle

      **Description:** 
      

40. ns/openshift-monitoring configmap/prometheus-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

41. ns/openshift-monitoring configmap/telemeter-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

42. ns/openshift-monitoring configmap/thanos-querier-trusted-ca-bundle-2ua4n9ob5qr8o

      **Description:** 
      

43. ns/openshift-network-node-identity configmap/network-node-identity-ca

      **Description:** 
      

44. ns/openshift-ovn-kubernetes configmap/ovn-ca

      **Description:** 
      

45. ns/openshift-ovn-kubernetes configmap/signer-ca

      **Description:** 
      

46. ns/openshift-route-controller-manager configmap/client-ca

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
      



      

## Networking / cluster-network-operator (29)
### Certificate Authority Bundles (29)
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
      

26. ns/openshift-monitoring configmap/alertmanager-trusted-ca-bundle

      **Description:** 
      

27. ns/openshift-monitoring configmap/prometheus-trusted-ca-bundle

      **Description:** 
      

28. ns/openshift-monitoring configmap/telemeter-trusted-ca-bundle

      **Description:** 
      

29. ns/openshift-monitoring configmap/thanos-querier-trusted-ca-bundle

      **Description:** 
      



      

## kube-apiserver (6)
### Certificates (6)
1. ns/openshift-kube-apiserver-operator secret/aggregator-client-signer

      **Description:** 
      

2. ns/openshift-kube-apiserver-operator secret/kube-apiserver-to-kubelet-signer

      **Description:** 
      

3. ns/openshift-kube-apiserver-operator secret/kube-control-plane-signer

      **Description:** 
      

4. ns/openshift-kube-apiserver-operator secret/loadbalancer-serving-signer

      **Description:** 
      

5. ns/openshift-kube-apiserver-operator secret/localhost-serving-signer

      **Description:** 
      

6. ns/openshift-kube-apiserver-operator secret/service-network-serving-signer

      **Description:** 
      



      

## service-ca (87)
### Certificates (84)
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
      

8. ns/openshift-cluster-csi-drivers secret/azure-disk-csi-driver-controller-metrics-serving-cert

      **Description:** 
      

9. ns/openshift-cluster-csi-drivers secret/azure-file-csi-driver-controller-metrics-serving-cert

      **Description:** 
      

10. ns/openshift-cluster-csi-drivers secret/gcp-pd-csi-driver-controller-metrics-serving-cert

      **Description:** 
      

11. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-controller-metrics-serving-cert

      **Description:** 
      

12. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-operator-metrics-serving-cert

      **Description:** 
      

13. ns/openshift-cluster-csi-drivers secret/vmware-vsphere-csi-driver-webhook-secret

      **Description:** 
      

14. ns/openshift-cluster-machine-approver secret/machine-approver-tls

      **Description:** 
      

15. ns/openshift-cluster-node-tuning-operator secret/node-tuning-operator-tls

      **Description:** 
      

16. ns/openshift-cluster-node-tuning-operator secret/performance-addon-operator-webhook-cert

      **Description:** 
      

17. ns/openshift-cluster-samples-operator secret/samples-operator-tls

      **Description:** 
      

18. ns/openshift-cluster-storage-operator secret/cluster-storage-operator-serving-cert

      **Description:** 
      

19. ns/openshift-cluster-storage-operator secret/csi-snapshot-webhook-secret

      **Description:** 
      

20. ns/openshift-cluster-storage-operator secret/serving-cert

      **Description:** 
      

21. ns/openshift-cluster-storage-operator secret/vsphere-problem-detector-serving-cert

      **Description:** 
      

22. ns/openshift-cluster-version secret/cluster-version-operator-serving-cert

      **Description:** 
      

23. ns/openshift-config-operator secret/config-operator-serving-cert

      **Description:** 
      

24. ns/openshift-console secret/console-serving-cert

      **Description:** 
      

25. ns/openshift-console-operator secret/serving-cert

      **Description:** 
      

26. ns/openshift-console-operator secret/webhook-serving-cert

      **Description:** 
      

27. ns/openshift-controller-manager secret/serving-cert

      **Description:** 
      

28. ns/openshift-controller-manager-operator secret/openshift-controller-manager-operator-serving-cert

      **Description:** 
      

29. ns/openshift-dns secret/dns-default-metrics-tls

      **Description:** 
      

30. ns/openshift-dns-operator secret/metrics-tls

      **Description:** 
      

31. ns/openshift-e2e-loki secret/proxy-tls

      **Description:** 
      

32. ns/openshift-etcd secret/serving-cert

      **Description:** 
      

33. ns/openshift-etcd-operator secret/etcd-operator-serving-cert

      **Description:** 
      

34. ns/openshift-image-registry secret/image-registry-operator-tls

      **Description:** 
      

35. ns/openshift-image-registry secret/image-registry-tls

      **Description:** 
      

36. ns/openshift-ingress secret/router-metrics-certs-default

      **Description:** 
      

37. ns/openshift-ingress-operator secret/metrics-tls

      **Description:** 
      

38. ns/openshift-insights secret/openshift-insights-serving-cert

      **Description:** 
      

39. ns/openshift-kube-apiserver-operator secret/kube-apiserver-operator-serving-cert

      **Description:** 
      

40. ns/openshift-kube-controller-manager secret/serving-cert

      **Description:** 
      

41. ns/openshift-kube-controller-manager-operator secret/kube-controller-manager-operator-serving-cert

      **Description:** 
      

42. ns/openshift-kube-scheduler secret/serving-cert

      **Description:** 
      

43. ns/openshift-kube-scheduler-operator secret/kube-scheduler-operator-serving-cert

      **Description:** 
      

44. ns/openshift-kube-storage-version-migrator-operator secret/serving-cert

      **Description:** 
      

45. ns/openshift-machine-api secret/baremetal-operator-webhook-server-cert

      **Description:** 
      

46. ns/openshift-machine-api secret/cluster-autoscaler-operator-cert

      **Description:** 
      

47. ns/openshift-machine-api secret/cluster-baremetal-operator-tls

      **Description:** 
      

48. ns/openshift-machine-api secret/cluster-baremetal-webhook-server-cert

      **Description:** 
      

49. ns/openshift-machine-api secret/control-plane-machine-set-operator-tls

      **Description:** 
      

50. ns/openshift-machine-api secret/machine-api-controllers-tls

      **Description:** 
      

51. ns/openshift-machine-api secret/machine-api-operator-machine-webhook-cert

      **Description:** 
      

52. ns/openshift-machine-api secret/machine-api-operator-tls

      **Description:** 
      

53. ns/openshift-machine-api secret/machine-api-operator-webhook-cert

      **Description:** 
      

54. ns/openshift-machine-config-operator secret/mcc-proxy-tls

      **Description:** 
      

55. ns/openshift-machine-config-operator secret/mco-proxy-tls

      **Description:** 
      

56. ns/openshift-machine-config-operator secret/proxy-tls

      **Description:** 
      

57. ns/openshift-marketplace secret/marketplace-operator-metrics

      **Description:** 
      

58. ns/openshift-monitoring secret/alertmanager-main-tls

      **Description:** 
      

59. ns/openshift-monitoring secret/cluster-monitoring-operator-tls

      **Description:** 
      

60. ns/openshift-monitoring secret/kube-state-metrics-tls

      **Description:** 
      

61. ns/openshift-monitoring secret/monitoring-plugin-cert

      **Description:** 
      

62. ns/openshift-monitoring secret/node-exporter-tls

      **Description:** 
      

63. ns/openshift-monitoring secret/openshift-state-metrics-tls

      **Description:** 
      

64. ns/openshift-monitoring secret/prometheus-adapter-tls

      **Description:** 
      

65. ns/openshift-monitoring secret/prometheus-k8s-thanos-sidecar-tls

      **Description:** 
      

66. ns/openshift-monitoring secret/prometheus-k8s-tls

      **Description:** 
      

67. ns/openshift-monitoring secret/prometheus-operator-admission-webhook-tls

      **Description:** 
      

68. ns/openshift-monitoring secret/prometheus-operator-tls

      **Description:** 
      

69. ns/openshift-monitoring secret/telemeter-client-tls

      **Description:** 
      

70. ns/openshift-monitoring secret/thanos-querier-tls

      **Description:** 
      

71. ns/openshift-multus secret/metrics-daemon-secret

      **Description:** 
      

72. ns/openshift-multus secret/multus-admission-controller-secret

      **Description:** 
      

73. ns/openshift-network-operator secret/metrics-tls

      **Description:** 
      

74. ns/openshift-oauth-apiserver secret/serving-cert

      **Description:** 
      

75. ns/openshift-operator-lifecycle-manager secret/catalog-operator-serving-cert

      **Description:** 
      

76. ns/openshift-operator-lifecycle-manager secret/olm-operator-serving-cert

      **Description:** 
      

77. ns/openshift-operator-lifecycle-manager secret/package-server-manager-serving-cert

      **Description:** 
      

78. ns/openshift-ovn-kubernetes secret/ovn-control-plane-metrics-cert

      **Description:** 
      

79. ns/openshift-ovn-kubernetes secret/ovn-node-metrics-cert

      **Description:** 
      

80. ns/openshift-route-controller-manager secret/serving-cert

      **Description:** 
      

81. ns/openshift-sdn secret/sdn-controller-metrics-certs

      **Description:** 
      

82. ns/openshift-sdn secret/sdn-metrics-certs

      **Description:** 
      

83. ns/openshift-service-ca secret/signing-key

      **Description:** 
      

84. ns/openshift-service-ca-operator secret/serving-cert

      **Description:** 
      



      

### Certificate Authority Bundles (3)
1. ns/openshift-config-managed configmap/service-ca

      **Description:** 
      

2. ns/openshift-kube-controller-manager configmap/service-ca

      **Description:** 
      

3. ns/openshift-service-ca configmap/signing-cabundle

      **Description:** 
      



      

