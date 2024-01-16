# kube-apiserver Serving Certificates

kube-apiserver Serving Certificates

![PKI Graph](cert-flow.png)

- [Signing Certificate/Key Pairs](#signing-certificatekey-pairs)
    - [ingress-operator@1704273869](#ingress-operator1704273869)
    - [kube-apiserver-lb-signer](#kube-apiserver-lb-signer)
    - [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer)
    - [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer)
    - [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804)
- [Serving Certificate/Key Pairs](#serving-certificatekey-pairs)
    - [*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org](#*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org)
    - [api-int.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org](#api-int.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org)
    - [api.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org](#api.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org)
    - [127.0.0.1](#127.0.0.1)
    - [172.30.0.1](#172.30.0.1)
    - [localhost-recovery](#localhost-recovery)
- [Client Certificate/Key Pairs](#client-certificatekey-pairs)
- [Certificates Without Keys](#certificates-without-keys)
- [Certificate Authority Bundles](#certificate-authority-bundles)
    - [*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org|ingress-operator@1704273869](#*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org|ingress-operator1704273869)
    - [kube-apiserver-lb-signer](#kube-apiserver-lb-signer)
    - [kube-apiserver-lb-signer|kube-apiserver-localhost-signer|kube-apiserver-service-network-signer|openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#kube-apiserver-lb-signer|kube-apiserver-localhost-signer|kube-apiserver-service-network-signer|openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804)
    - [kube-apiserver-lb-signer|kube-apiserver-localhost-signer|kube-apiserver-service-network-signer|openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804|*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org|ingress-operator@1704273869](#kube-apiserver-lb-signer|kube-apiserver-localhost-signer|kube-apiserver-service-network-signer|openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804|*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org|ingress-operator1704273869)
    - [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer)
    - [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer)
    - [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804)

## Signing Certificate/Key Pairs


### ingress-operator@1704273869
![PKI Graph](subcert-ingress-operator17042738691.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | ingress-operator@1704273869 |
| SerialNumber | 1 |
| Issuer CommonName | [ingress-operator@1704273869](#ingress-operator1704273869) |
| Validity | 2y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### ingress-operator@1704273869 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-ingress-operator | router-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-lb-signer
![PKI Graph](subcert-kube-apiserver-lb-signer4624852848640529251.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | kube-apiserver-lb-signer |
| SerialNumber | 4624852848640529251 |
| Issuer CommonName | [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) |
| Validity | 10y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### kube-apiserver-lb-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | loadbalancer-serving-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-localhost-signer
![PKI Graph](subcert-kube-apiserver-localhost-signer6263268324069729512.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | kube-apiserver-localhost-signer |
| SerialNumber | 6263268324069729512 |
| Issuer CommonName | [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer) |
| Validity | 10y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### kube-apiserver-localhost-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | localhost-serving-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-service-network-signer
![PKI Graph](subcert-kube-apiserver-service-network-signer4393677514218278744.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | kube-apiserver-service-network-signer |
| SerialNumber | 4393677514218278744 |
| Issuer CommonName | [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer) |
| Validity | 10y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### kube-apiserver-service-network-signer Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | service-network-serving-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804
![PKI Graph](subcert-openshift-kube-apiserver-operator_localhost-recovery-serving-signer17042738047848375026133314319.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804 |
| SerialNumber | 7848375026133314319 |
| Issuer CommonName | [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804) |
| Validity | 10y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | localhost-recovery-serving-signer |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Serving Certificate/Key Pairs


### *.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org
![PKI Graph](subcert-*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org2822621581346623594.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving |
| CommonName | *.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org |
| SerialNumber | 2822621581346623594 |
| Issuer CommonName | [ingress-operator@1704273869](#ingress-operator1704273869) |
| Validity | 2y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth |
| DNS Names | - *.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org |
| IP Addresses |  |


#### *.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-ingress | router-certs-default |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### api-int.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org
![PKI Graph](subcert-api-int.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org2878555592070185251.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving |
| CommonName | api-int.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org |
| SerialNumber | 2878555592070185251 |
| Issuer CommonName | [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth |
| DNS Names | - api-int.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org |
| IP Addresses |  |


#### api-int.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | internal-loadbalancer-serving-certkey |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### api.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org
![PKI Graph](subcert-api.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org7483887125459485093.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving |
| CommonName | api.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org |
| SerialNumber | 7483887125459485093 |
| Issuer CommonName | [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth |
| DNS Names | - api.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org |
| IP Addresses |  |


#### api.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | external-loadbalancer-serving-certkey |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### 127.0.0.1
![PKI Graph](subcert-127.0.0.17980066732065963980.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving |
| CommonName | 127.0.0.1 |
| SerialNumber | 7980066732065963980 |
| Issuer CommonName | [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth |
| DNS Names | - localhost<br/>- 127.0.0.1 |
| IP Addresses | - 127.0.0.1 |


#### 127.0.0.1 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | localhost-serving-cert-certkey |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### 172.30.0.1
![PKI Graph](subcert-172.30.0.13074158037071720576.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving |
| CommonName | 172.30.0.1 |
| SerialNumber | 3074158037071720576 |
| Issuer CommonName | [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer) |
| Validity | 12h |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth |
| DNS Names | - kubernetes<br/>- kubernetes.default<br/>- kubernetes.default.svc<br/>- kubernetes.default.svc.cluster.local<br/>- openshift<br/>- openshift.default<br/>- openshift.default.svc<br/>- openshift.default.svc.cluster.local<br/>- 172.30.0.1 |
| IP Addresses | - 172.30.0.1 |


#### 172.30.0.1 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | service-network-serving-certkey |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### localhost-recovery
![PKI Graph](subcert-localhost-recovery3947081783999751122.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving |
| CommonName | localhost-recovery |
| SerialNumber | 3947081783999751122 |
| Issuer CommonName | [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804) |
| Validity | 9y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth |
| DNS Names | - localhost-recovery |
| IP Addresses |  |


#### localhost-recovery Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-kube-apiserver | localhost-recovery-serving-certkey |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Client Certificate/Key Pairs

## Certificates Without Keys

These certificates are present in certificate authority bundles, but do not have keys in the cluster.
This happens when the installer bootstrap clusters with a set of certificate/key pairs that are deleted during the
installation process.

## Certificate Authority Bundles


### *.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org|ingress-operator@1704273869
![PKI Graph](subca-890184271.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org](#*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org) | [ingress-operator@1704273869](#ingress-operator1704273869) | 2y | RSA 2048 bit |
| [ingress-operator@1704273869](#ingress-operator1704273869) | [ingress-operator@1704273869](#ingress-operator1704273869) | 2y | RSA 2048 bit |

#### *.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org|ingress-operator@1704273869 Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-config-managed | default-ingress-cert |
| openshift-config-managed | oauth-serving-cert |
| openshift-console | default-ingress-cert |
| openshift-console | oauth-serving-cert |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-lb-signer
![PKI Graph](subca-3666101852.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) | [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) | 10y | RSA 2048 bit |

#### kube-apiserver-lb-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | loadbalancer-serving-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-lb-signer|kube-apiserver-localhost-signer|kube-apiserver-service-network-signer|openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804
![PKI Graph](subca-1798593712.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) | [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) | 10y | RSA 2048 bit |
| [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer) | [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer) | 10y | RSA 2048 bit |
| [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer) | [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer) | 10y | RSA 2048 bit |
| [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804) | [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804) | 10y | RSA 2048 bit |

#### kube-apiserver-lb-signer|kube-apiserver-localhost-signer|kube-apiserver-service-network-signer|openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804 Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-config-managed | kube-apiserver-server-ca |
| openshift-kube-apiserver | kube-apiserver-server-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-lb-signer|kube-apiserver-localhost-signer|kube-apiserver-service-network-signer|openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804|*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org|ingress-operator@1704273869
![PKI Graph](subca-1887140450.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) | [kube-apiserver-lb-signer](#kube-apiserver-lb-signer) | 10y | RSA 2048 bit |
| [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer) | [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer) | 10y | RSA 2048 bit |
| [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer) | [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer) | 10y | RSA 2048 bit |
| [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804) | [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804) | 10y | RSA 2048 bit |
| [*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org](#*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org) | [ingress-operator@1704273869](#ingress-operator1704273869) | 2y | RSA 2048 bit |
| [ingress-operator@1704273869](#ingress-operator1704273869) | [ingress-operator@1704273869](#ingress-operator1704273869) | 2y | RSA 2048 bit |

#### kube-apiserver-lb-signer|kube-apiserver-localhost-signer|kube-apiserver-service-network-signer|openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804|*.apps.ci-op-mlisxgvx-5feef.aws-2.ci.openshift.org|ingress-operator@1704273869 Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-controller-manager | serviceaccount-ca |
| openshift-kube-scheduler | serviceaccount-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-localhost-signer
![PKI Graph](subca-537014043.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer) | [kube-apiserver-localhost-signer](#kube-apiserver-localhost-signer) | 10y | RSA 2048 bit |

#### kube-apiserver-localhost-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | localhost-serving-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### kube-apiserver-service-network-signer
![PKI Graph](subca-1272615924.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer) | [kube-apiserver-service-network-signer](#kube-apiserver-service-network-signer) | 10y | RSA 2048 bit |

#### kube-apiserver-service-network-signer Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | service-network-serving-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |




### openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804
![PKI Graph](subca-3377924736.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804) | [openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804](#openshift-kube-apiserver-operator_localhost-recovery-serving-signer1704273804) | 10y | RSA 2048 bit |

#### openshift-kube-apiserver-operator_localhost-recovery-serving-signer@1704273804 Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-kube-apiserver-operator | localhost-recovery-serving-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



