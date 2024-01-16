# kube-apiserver Client Certificates

kube-apiserver Client Certificates

![PKI Graph](cert-flow.png)

- [Signing Certificate/Key Pairs](#signing-certificatekey-pairs)
    - [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer)
    - [kube-control-plane-signer](#kube-control-plane-signer)
    - [kubelet-signer](#kubelet-signer)
    - [kube-csr-signer_@1704273808](#kube-csr-signer_1704273808)
    - [openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#openshift-kube-apiserver-operator_node-system-admin-signer1704273804)
- [Serving Certificate/Key Pairs](#serving-certificatekey-pairs)
- [Client Certificate/Key Pairs](#client-certificatekey-pairs)
    - [system:admin](#systemadmin)
    - [system:control-plane-node-admin](#systemcontrol-plane-node-admin)
    - [system:kube-apiserver](#systemkube-apiserver)
    - [system:kube-controller-manager](#systemkube-controller-manager)
    - [system:kube-scheduler](#systemkube-scheduler)
    - [system:serviceaccount:openshift-kube-apiserver:check-endpoints](#systemserviceaccountopenshift-kube-apiservercheck-endpoints)
    - [system:serviceaccount:openshift-monitoring:prometheus-k8s](#systemserviceaccountopenshift-monitoringprometheus-k8s)
    - [system:serviceaccount:openshift-monitoring:prometheus-k8s](#systemserviceaccountopenshift-monitoringprometheus-k8s)
    - [system:serviceaccount:openshift-oauth-apiserver:openshift-authenticator](#systemserviceaccountopenshift-oauth-apiserveropenshift-authenticator)
- [Certificates Without Keys](#certificates-without-keys)
    - [admin-kubeconfig-signer](#admin-kubeconfig-signer)
    - [kubelet-bootstrap-kubeconfig-signer](#kubelet-bootstrap-kubeconfig-signer)
- [Certificate Authority Bundles](#certificate-authority-bundles)
    - [admin-kubeconfig-signer](#admin-kubeconfig-signer)
    - [admin-kubeconfig-signer|kube-csr-signer_@1704273808|kubelet-signer|kube-apiserver-to-kubelet-signer|kube-control-plane-signer|kubelet-bootstrap-kubeconfig-signer|openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#admin-kubeconfig-signer|kube-csr-signer_1704273808|kubelet-signer|kube-apiserver-to-kubelet-signer|kube-control-plane-signer|kubelet-bootstrap-kubeconfig-signer|openshift-kube-apiserver-operator_node-system-admin-signer1704273804)
    - [admin-kubeconfig-signer|kubelet-signer|kube-control-plane-signer|kube-apiserver-to-kubelet-signer|kubelet-bootstrap-kubeconfig-signer](#admin-kubeconfig-signer|kubelet-signer|kube-control-plane-signer|kube-apiserver-to-kubelet-signer|kubelet-bootstrap-kubeconfig-signer)
    - [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer)
    - [kube-control-plane-signer](#kube-control-plane-signer)
    - [kube-csr-signer_@1704273808|kubelet-signer](#kube-csr-signer_1704273808|kubelet-signer)
    - [kubelet-bootstrap-kubeconfig-signer](#kubelet-bootstrap-kubeconfig-signer)
    - [kubelet-signer](#kubelet-signer)
    - [openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#openshift-kube-apiserver-operator_node-system-admin-signer1704273804)

## Signing Certificate/Key Pairs


### kube-apiserver-to-kubelet-signer
![PKI Graph](subcert-kube-apiserver-to-kubelet-signer5911051446634154830.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | kube-apiserver-to-kubelet-signer |
| SerialNumber | 5911051446634154830 |
| Issuer CommonName | [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer) |
| Validity | 365d |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### kube-apiserver-to-kubelet-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | kube-apiserver-to-kubelet-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-control-plane-signer
![PKI Graph](subcert-kube-control-plane-signer227887426676886745.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | kube-control-plane-signer |
| SerialNumber | 227887426676886745 |
| Issuer CommonName | [kube-control-plane-signer](#kube-control-plane-signer) |
| Validity | 365d |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### kube-control-plane-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | kube-control-plane-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kubelet-signer
![PKI Graph](subcert-kubelet-signer2112051707629287438.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | kubelet-signer |
| SerialNumber | 2112051707629287438 |
| Issuer CommonName | [kubelet-signer](#kubelet-signer) |
| Validity | 24h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### kubelet-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-controller-manager-operator | csr-signer-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-csr-signer_@1704273808
![PKI Graph](subcert-kube-csr-signer_17042738089167585266960646865.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | kube-csr-signer_@1704273808 |
| SerialNumber | 9167585266960646865 |
| Issuer CommonName | [kubelet-signer](#kubelet-signer) |
| Validity | 23h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### kube-csr-signer_@1704273808 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-controller-manager-operator | csr-signer |
| openshift-kube-controller-manager | csr-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### openshift-kube-apiserver-operator_node-system-admin-signer@1704273804
![PKI Graph](subcert-openshift-kube-apiserver-operator_node-system-admin-signer17042738043782504597567108544.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | openshift-kube-apiserver-operator_node-system-admin-signer@1704273804 |
| SerialNumber | 3782504597567108544 |
| Issuer CommonName | [openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#openshift-kube-apiserver-operator_node-system-admin-signer1704273804) |
| Validity | 365d |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### openshift-kube-apiserver-operator_node-system-admin-signer@1704273804 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | node-system-admin-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Serving Certificate/Key Pairs

## Client Certificate/Key Pairs


### system:admin
![PKI Graph](subcert-systemadmin8613085297608310858.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:admin |
| SerialNumber | 8613085297608310858 |
| Issuer CommonName | [openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#openshift-kube-apiserver-operator_node-system-admin-signer1704273804) |
| Validity | 120d |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) | - system:masters |


#### system:admin Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | node-system-admin-client |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### system:control-plane-node-admin
![PKI Graph](subcert-systemcontrol-plane-node-admin1059210817525278201.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:control-plane-node-admin |
| SerialNumber | 1059210817525278201 |
| Issuer CommonName | [kube-control-plane-signer](#kube-control-plane-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) | - system:masters |


#### system:control-plane-node-admin Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | control-plane-node-admin-client-cert-key |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### system:kube-apiserver
![PKI Graph](subcert-systemkube-apiserver4721398849933843688.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:kube-apiserver |
| SerialNumber | 4721398849933843688 |
| Issuer CommonName | [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) | - kube-master |


#### system:kube-apiserver Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | kubelet-client |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### system:kube-controller-manager
![PKI Graph](subcert-systemkube-controller-manager3036637072996509399.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:kube-controller-manager |
| SerialNumber | 3036637072996509399 |
| Issuer CommonName | [kube-control-plane-signer](#kube-control-plane-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |


#### system:kube-controller-manager Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-config-managed | kube-controller-manager-client-cert-key |
| openshift-kube-controller-manager | kube-controller-manager-client-cert-key |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### system:kube-scheduler
![PKI Graph](subcert-systemkube-scheduler6257318142771253203.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:kube-scheduler |
| SerialNumber | 6257318142771253203 |
| Issuer CommonName | [kube-control-plane-signer](#kube-control-plane-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |


#### system:kube-scheduler Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-config-managed | kube-scheduler-client-cert-key |
| openshift-kube-scheduler | kube-scheduler-client-cert-key |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### system:serviceaccount:openshift-kube-apiserver:check-endpoints
![PKI Graph](subcert-systemserviceaccountopenshift-kube-apiservercheck-endpoints2576004262911247332.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:serviceaccount:openshift-kube-apiserver:check-endpoints |
| SerialNumber | 2576004262911247332 |
| Issuer CommonName | [kube-control-plane-signer](#kube-control-plane-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |


#### system:serviceaccount:openshift-kube-apiserver:check-endpoints Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | check-endpoints-client-cert-key |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### system:serviceaccount:openshift-monitoring:prometheus-k8s
![PKI Graph](subcert-systemserviceaccountopenshift-monitoringprometheus-k8s74288101070105103377383352571615521356.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:serviceaccount:openshift-monitoring:prometheus-k8s |
| SerialNumber | 74288101070105103377383352571615521356 |
| Issuer CommonName | [kubelet-signer](#kubelet-signer) |
| Validity | 23h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | ECDSA 256 bit, P-256 curve |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |


#### system:serviceaccount:openshift-monitoring:prometheus-k8s Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-monitoring | metrics-client-certs |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### system:serviceaccount:openshift-monitoring:prometheus-k8s
![PKI Graph](subcert-systemserviceaccountopenshift-monitoringprometheus-k8s144420821587744385522574508721281371890.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:serviceaccount:openshift-monitoring:prometheus-k8s |
| SerialNumber | 144420821587744385522574508721281371890 |
| Issuer CommonName | [kubelet-signer](#kubelet-signer) |
| Validity | 23h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | ECDSA 256 bit, P-256 curve |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |


#### system:serviceaccount:openshift-monitoring:prometheus-k8s Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-monitoring | federate-client-certs |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### system:serviceaccount:openshift-oauth-apiserver:openshift-authenticator
![PKI Graph](subcert-systemserviceaccountopenshift-oauth-apiserveropenshift-authenticator39470527241106009848019183161878199127.png)



| Property | Value |
| ----------- | ----------- |
| Type | Client |
| CommonName | system:serviceaccount:openshift-oauth-apiserver:openshift-authenticator |
| SerialNumber | 39470527241106009848019183161878199127 |
| Issuer CommonName | [kubelet-signer](#kubelet-signer) |
| Validity | 23h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | ECDSA 256 bit, P-256 curve |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |


#### system:serviceaccount:openshift-oauth-apiserver:openshift-authenticator Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-oauth-apiserver | openshift-authenticator-certs |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Certificates Without Keys

These certificates are present in certificate authority bundles, but do not have keys in the cluster.
This happens when the installer bootstrap clusters with a set of certificate/key pairs that are deleted during the
installation process.


### admin-kubeconfig-signer
![PKI Graph](subcert-admin-kubeconfig-signer.png)



| Property | Value |
| ----------- | ----------- |
| Type |  |
| CommonName | admin-kubeconfig-signer |
| SerialNumber |  |
| Issuer CommonName | None |
| Validity |  |
| Signature Algorithm |  |
| PublicKey Algorithm |   |
| Usages |  |
| ExtendedUsages |  |


#### admin-kubeconfig-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |


| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kubelet-bootstrap-kubeconfig-signer
![PKI Graph](subcert-kubelet-bootstrap-kubeconfig-signer.png)



| Property | Value |
| ----------- | ----------- |
| Type |  |
| CommonName | kubelet-bootstrap-kubeconfig-signer |
| SerialNumber |  |
| Issuer CommonName | None |
| Validity |  |
| Signature Algorithm |  |
| PublicKey Algorithm |   |
| Usages |  |
| ExtendedUsages |  |


#### kubelet-bootstrap-kubeconfig-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |


| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Certificate Authority Bundles


### admin-kubeconfig-signer
![PKI Graph](subca-1227797221.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [admin-kubeconfig-signer](#admin-kubeconfig-signer) | [admin-kubeconfig-signer](#admin-kubeconfig-signer) | 10y | RSA 2048 bit |

#### admin-kubeconfig-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-config | admin-kubeconfig-client-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### admin-kubeconfig-signer|kube-csr-signer_@1704273808|kubelet-signer|kube-apiserver-to-kubelet-signer|kube-control-plane-signer|kubelet-bootstrap-kubeconfig-signer|openshift-kube-apiserver-operator_node-system-admin-signer@1704273804
![PKI Graph](subca-218503947.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [admin-kubeconfig-signer](#admin-kubeconfig-signer) | [admin-kubeconfig-signer](#admin-kubeconfig-signer) | 10y | RSA 2048 bit |
| [kube-csr-signer_@1704273808](#kube-csr-signer_1704273808) | [kubelet-signer](#kubelet-signer) | 23h | RSA 2048 bit |
| [kubelet-signer](#kubelet-signer) | [kubelet-signer](#kubelet-signer) | 24h | RSA 2048 bit |
| [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer) | [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer) | 365d | RSA 2048 bit |
| [kube-control-plane-signer](#kube-control-plane-signer) | [kube-control-plane-signer](#kube-control-plane-signer) | 365d | RSA 2048 bit |
| [kubelet-bootstrap-kubeconfig-signer](#kubelet-bootstrap-kubeconfig-signer) | [kubelet-bootstrap-kubeconfig-signer](#kubelet-bootstrap-kubeconfig-signer) | 10y | RSA 2048 bit |
| [openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#openshift-kube-apiserver-operator_node-system-admin-signer1704273804) | [openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#openshift-kube-apiserver-operator_node-system-admin-signer1704273804) | 365d | RSA 2048 bit |

#### admin-kubeconfig-signer|kube-csr-signer_@1704273808|kubelet-signer|kube-apiserver-to-kubelet-signer|kube-control-plane-signer|kubelet-bootstrap-kubeconfig-signer|openshift-kube-apiserver-operator_node-system-admin-signer@1704273804 Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-config-managed | kube-apiserver-client-ca |
| openshift-controller-manager | client-ca |
| openshift-kube-apiserver | client-ca |
| openshift-kube-controller-manager | client-ca |
| openshift-route-controller-manager | client-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### admin-kubeconfig-signer|kubelet-signer|kube-control-plane-signer|kube-apiserver-to-kubelet-signer|kubelet-bootstrap-kubeconfig-signer
![PKI Graph](subca-3228260836.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [admin-kubeconfig-signer](#admin-kubeconfig-signer) | [admin-kubeconfig-signer](#admin-kubeconfig-signer) | 10y | RSA 2048 bit |
| [kubelet-signer](#kubelet-signer) | [kubelet-signer](#kubelet-signer) | 24h | RSA 2048 bit |
| [kube-control-plane-signer](#kube-control-plane-signer) | [kube-control-plane-signer](#kube-control-plane-signer) | 365d | RSA 2048 bit |
| [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer) | [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer) | 365d | RSA 2048 bit |
| [kubelet-bootstrap-kubeconfig-signer](#kubelet-bootstrap-kubeconfig-signer) | [kubelet-bootstrap-kubeconfig-signer](#kubelet-bootstrap-kubeconfig-signer) | 10y | RSA 2048 bit |

#### admin-kubeconfig-signer|kubelet-signer|kube-control-plane-signer|kube-apiserver-to-kubelet-signer|kubelet-bootstrap-kubeconfig-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-config | initial-kube-apiserver-server-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-to-kubelet-signer
![PKI Graph](subca-1996114968.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer) | [kube-apiserver-to-kubelet-signer](#kube-apiserver-to-kubelet-signer) | 365d | RSA 2048 bit |

#### kube-apiserver-to-kubelet-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | kube-apiserver-to-kubelet-client-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-control-plane-signer
![PKI Graph](subca-3938274656.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kube-control-plane-signer](#kube-control-plane-signer) | [kube-control-plane-signer](#kube-control-plane-signer) | 365d | RSA 2048 bit |

#### kube-control-plane-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | kube-control-plane-signer-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-csr-signer_@1704273808|kubelet-signer
![PKI Graph](subca-429284096.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kube-csr-signer_@1704273808](#kube-csr-signer_1704273808) | [kubelet-signer](#kubelet-signer) | 23h | RSA 2048 bit |
| [kubelet-signer](#kubelet-signer) | [kubelet-signer](#kubelet-signer) | 24h | RSA 2048 bit |

#### kube-csr-signer_@1704273808|kubelet-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-config-managed | csr-controller-ca |
| openshift-config-managed | kubelet-serving-ca |
| openshift-kube-apiserver | kubelet-serving-ca |
| openshift-kube-controller-manager-operator | csr-controller-ca |
| openshift-kube-controller-manager-operator | csr-signer-ca |
| openshift-monitoring | kubelet-serving-ca-bundle |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kubelet-bootstrap-kubeconfig-signer
![PKI Graph](subca-2747639387.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kubelet-bootstrap-kubeconfig-signer](#kubelet-bootstrap-kubeconfig-signer) | [kubelet-bootstrap-kubeconfig-signer](#kubelet-bootstrap-kubeconfig-signer) | 10y | RSA 2048 bit |

#### kubelet-bootstrap-kubeconfig-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-config-managed | kubelet-bootstrap-kubeconfig |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kubelet-signer
![PKI Graph](subca-2106850666.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kubelet-signer](#kubelet-signer) | [kubelet-signer](#kubelet-signer) | 24h | RSA 2048 bit |

#### kubelet-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-controller-manager-operator | csr-controller-signer-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### openshift-kube-apiserver-operator_node-system-admin-signer@1704273804
![PKI Graph](subca-4039217392.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#openshift-kube-apiserver-operator_node-system-admin-signer1704273804) | [openshift-kube-apiserver-operator_node-system-admin-signer@1704273804](#openshift-kube-apiserver-operator_node-system-admin-signer1704273804) | 365d | RSA 2048 bit |

#### openshift-kube-apiserver-operator_node-system-admin-signer@1704273804 Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | node-system-admin-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



