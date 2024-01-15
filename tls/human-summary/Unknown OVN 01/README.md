# Unknown OVN 01

Unknown OVN 01

![PKI Graph](cert-flow.png)

- [Signing Certificate/Key Pairs](#signing-certificatekey-pairs)
    - [openshift-ovn-kubernetes_ovn-ca@1704273747](#openshift-ovn-kubernetes_ovn-ca1704273747)
- [Serving Certificate/Key Pairs](#serving-certificatekey-pairs)
    - [ovn](#ovn)
- [Client Certificate/Key Pairs](#client-certificatekey-pairs)
    - [ovn](#ovn)
- [Certificates Without Keys](#certificates-without-keys)
- [Certificate Authority Bundles](#certificate-authority-bundles)
    - [openshift-ovn-kubernetes_ovn-ca@1704273747](#openshift-ovn-kubernetes_ovn-ca1704273747)

## Signing Certificate/Key Pairs


### openshift-ovn-kubernetes_ovn-ca@1704273747
![PKI Graph](subcert-openshift-ovn-kubernetes_ovn-ca17042737474223223887240789198.png)



| Property | Value |
| ----------- | ----------- |
| Type | Signer |
| CommonName | openshift-ovn-kubernetes_ovn-ca@1704273747 |
| SerialNumber | 4223223887240789198 |
| Issuer CommonName | [openshift-ovn-kubernetes_ovn-ca@1704273747](#openshift-ovn-kubernetes_ovn-ca1704273747) |
| Validity | 10y |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment<br/>- KeyUsageCertSign |
| ExtendedUsages |  |


#### openshift-ovn-kubernetes_ovn-ca@1704273747 Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-ovn-kubernetes | ovn-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Serving Certificate/Key Pairs


### ovn
![PKI Graph](subcert-ovn6017550323937142920.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving,Client |
| CommonName | ovn |
| SerialNumber | 6017550323937142920 |
| Issuer CommonName | [openshift-ovn-kubernetes_ovn-ca@1704273747](#openshift-ovn-kubernetes_ovn-ca1704273747) |
| Validity | 182d |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth<br/>- ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |
| DNS Names | - ovn |
| IP Addresses |  |


#### ovn Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-ovn-kubernetes | ovn-cert |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Client Certificate/Key Pairs


### ovn
![PKI Graph](subcert-ovn6017550323937142920.png)



| Property | Value |
| ----------- | ----------- |
| Type | Serving,Client |
| CommonName | ovn |
| SerialNumber | 6017550323937142920 |
| Issuer CommonName | [openshift-ovn-kubernetes_ovn-ca@1704273747](#openshift-ovn-kubernetes_ovn-ca1704273747) |
| Validity | 182d |
| Signature Algorithm | SHA256-RSA |
| PublicKey Algorithm | RSA 2048 bit |
| Usages | - KeyUsageDigitalSignature<br/>- KeyUsageKeyEncipherment |
| ExtendedUsages | - ExtKeyUsageServerAuth<br/>- ExtKeyUsageClientAuth |
| Organizations (User Groups) |  |
| DNS Names | - ovn |
| IP Addresses |  |


#### ovn Locations
| Namespace | Secret Name |
| ----------- | ----------- |
| openshift-ovn-kubernetes | ovn-cert |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



## Certificates Without Keys

These certificates are present in certificate authority bundles, but do not have keys in the cluster.
This happens when the installer bootstrap clusters with a set of certificate/key pairs that are deleted during the
installation process.

## Certificate Authority Bundles


### openshift-ovn-kubernetes_ovn-ca@1704273747
![PKI Graph](subca-715647561.png)



**Bundled Certificates**

| CommonName | Issuer CommonName | Validity | PublicKey Algorithm |
| ----------- | ----------- | ----------- | ----------- |
| [openshift-ovn-kubernetes_ovn-ca@1704273747](#openshift-ovn-kubernetes_ovn-ca1704273747) | [openshift-ovn-kubernetes_ovn-ca@1704273747](#openshift-ovn-kubernetes_ovn-ca1704273747) | 10y | RSA 2048 bit |

#### openshift-ovn-kubernetes_ovn-ca@1704273747 Locations
| Namespace | ConfigMap Name |
| ----------- | ----------- |
| openshift-ovn-kubernetes | ovn-ca |

| File | Permissions | User | Group | SE Linux |
| ----------- | ----------- | ----------- | ----------- | ----------- |



